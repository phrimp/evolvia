package controllers

import (
	"fmt"
	"strings"
	"time"

	"llm-service/models"
	"llm-service/services"
	"llm-service/utils"

	"github.com/gin-gonic/gin"
)

type LLMController struct{}

func NewLLMController() *LLMController {
	return &LLMController{}
}

// GET /public/llm/ping
func (ctrl *LLMController) Ping(c *gin.Context) {
	utils.SuccessResponse(c, "pong", gin.H{
		"service":   "llm-service",
		"version":   "1.0.0",
		"timestamp": time.Now(),
	})
}

// GET /public/llm/model
func (ctrl *LLMController) GetModelStatus(c *gin.Context) {
	db := services.GetDatabaseService()
	rabbitmq := services.GetRabbitMQService()
	llm := services.GetLLMService()

	status := &models.ServiceStatus{
		Service:   "llm-service",
		Version:   "1.0.0",
		Status:    "active",
		Timestamp: time.Now(),
		Connection: models.ServiceConnection{
			MongoDB:  db != nil && db.IsConnected(),
			RabbitMQ: rabbitmq != nil && rabbitmq.IsConnected(),
			LLMModel: llm != nil && llm.IsConnected(),
		},
	}

	utils.SuccessResponse(c, "Service status retrieved", status)
}

// POST /public/llm/model/session
func (ctrl *LLMController) CreateChatSession(c *gin.Context) {
	// Get user ID from token if available
	userID, err := utils.GetUserIDFromToken(c)
	if err != nil {
		// Log error for debugging
		fmt.Printf("JWT Error: %v\n", err)
		// Allow anonymous sessions
		userID = "anonymous"
	}

	// Debug log
	fmt.Printf("Creating session for userID: %s\n", userID)

	// Create a session ID
	sessionID := "session_" + utils.GenerateUUID()

	// Save session to database
	db := services.GetDatabaseService()
	var session *models.ChatSession
	if db != nil && db.IsConnected() {
		session, err = db.CreateChatSession(sessionID, userID)
		if err != nil {
			utils.InternalErrorResponse(c, "Failed to create chat session", err)
			return
		}
	}

	// Publish event to RabbitMQ
	if rabbitmq := services.GetRabbitMQService(); rabbitmq != nil {
		rabbitmq.PublishLLMEvent("session_created", map[string]interface{}{
			"sessionId": sessionID,
			"userId":    userID,
		})
	}

	response := gin.H{
		"sessionId": sessionID,
		"userId":    userID,
		"createdAt": time.Now(),
	}

	if session != nil {
		response["id"] = session.ID
		response["isActive"] = session.IsActive
	}

	utils.SuccessResponse(c, "Chat session created successfully", response)
}

// POST /public/llm/model/:sessionId/chat
func (ctrl *LLMController) Chat(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Param("sessionId"))
	if sessionID == "" {
		utils.BadRequestResponse(c, "Session ID is required")
		return
	}

	var request models.LLMRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequestResponse(c, "Invalid request format")
		return
	}

	// Get user ID from token if available
	userID, err := utils.GetUserIDFromToken(c)
	if err != nil {
		fmt.Printf("JWT parse error: %v\n", err)
		// Allow anonymous users
		userID = "anonymous"
	}

	fmt.Printf("Chat request - SessionID: %s, UserID: %s\n", sessionID, userID)

	// Validate session exists and belongs to user
	db := services.GetDatabaseService()
	if db != nil && db.IsConnected() {
		// Temporarily disable session validation for debugging
		// if !db.ValidateSession(sessionID, userID) {
		// 	utils.BadRequestResponse(c, "Invalid session or session does not belong to user")
		// 	return
		// }
		fmt.Printf("Session validation temporarily disabled for debugging\n")
	}

	// Process with LLM service
	llm := services.GetLLMService()
	if llm == nil {
		utils.InternalErrorResponse(c, "LLM service not available", nil)
		return
	}

	response, err := llm.ProcessChat(request.Message, userID)
	if err != nil {
		utils.InternalErrorResponse(c, "Failed to process chat message", err)
		return
	}

	// Save chat message to database
	if db != nil && db.IsConnected() {
		_, err = db.SaveChatMessage(sessionID, userID, request.Message, response.Message)
		if err != nil {
			// Log error but don't fail the request
			utils.InternalErrorResponse(c, "Failed to save chat message", err)
			return
		}
	}

	// Update response with session info
	response.SessionID = sessionID

	// Publish event to RabbitMQ
	if rabbitmq := services.GetRabbitMQService(); rabbitmq != nil {
		rabbitmq.PublishLLMEvent("chat_message", map[string]interface{}{
			"sessionId": sessionID,
			"userId":    userID,
			"message":   request.Message,
			"response":  response.Message,
		})
	}

	utils.SuccessResponse(c, "Chat message processed", response)
}

// GET /public/llm/model/history/:sessionId
func (ctrl *LLMController) GetChatHistory(c *gin.Context) {
	sessionID := strings.TrimSpace(c.Param("sessionId"))
	if sessionID == "" {
		utils.BadRequestResponse(c, "Session ID is required")
		return
	}

	// Get user ID from token if available
	userID, err := utils.GetUserIDFromToken(c)
	if err != nil {
		userID = "anonymous"
	}

	db := services.GetDatabaseService()
	if db == nil || !db.IsConnected() {
		utils.InternalErrorResponse(c, "Database service not available", nil)
		return
	}

	// Validate session exists and belongs to user
	if !db.ValidateSession(sessionID, userID) {
		utils.BadRequestResponse(c, "Invalid session or session does not belong to user")
		return
	}

	// Get chat history from database
	messages, err := db.GetChatHistory(sessionID, 0) // 0 = no limit
	if err != nil {
		utils.InternalErrorResponse(c, "Failed to retrieve chat history", err)
		return
	}

	utils.SuccessResponse(c, "Chat history retrieved", gin.H{
		"sessionId": sessionID,
		"messages":  messages,
		"count":     len(messages),
	})
}

// POST /public/llm/query - Allow LLM to execute custom queries
func (ctrl *LLMController) ExecuteQuery(c *gin.Context) {
	var request struct {
		Collection string                 `json:"collection" binding:"required"`
		Query      map[string]interface{} `json:"query" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequestResponse(c, "Invalid request format")
		return
	}

	// Get user ID from token if available
	userID, err := utils.GetUserIDFromToken(c)
	if err != nil {
		utils.UnauthorizedResponse(c, "Invalid token")
		return
	}

	if userID == "" {
		utils.UnauthorizedResponse(c, "User authentication required for database queries")
		return
	}

	// Get RAG service
	rag := services.GetRAGService()
	if rag == nil {
		utils.InternalErrorResponse(c, "RAG service not available", nil)
		return
	}

	// Execute query with security constraints
	results, err := rag.ExecuteCustomQuery(userID, "", request.Collection, request.Query)
	if err != nil {
		utils.InternalErrorResponse(c, "Failed to execute query", err)
		return
	}

	utils.SuccessResponse(c, "Query executed successfully", gin.H{
		"collection": request.Collection,
		"results":    results,
		"count":      len(results),
	})
}

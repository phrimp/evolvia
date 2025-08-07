package handlers

import (
	"context"
	"net/http"
	"quiz-service/internal/models"
	"quiz-service/internal/service"
	"time"

	"github.com/gin-gonic/gin"
)

type SessionHandler struct {
	Service       *service.SessionService
	AnswerService *service.AnswerService
}

func NewSessionHandler(s *service.SessionService, as *service.AnswerService) *SessionHandler {
	return &SessionHandler{
		Service:       s,
		AnswerService: as,
	}
}

func (h *SessionHandler) GetSession(c *gin.Context) {
	id := c.Param("id")
	session, err := h.Service.GetSession(context.Background(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}
	c.JSON(http.StatusOK, session)
}

func (h *SessionHandler) CreateSession(c *gin.Context) {
	var session models.QuizSession
	if err := c.ShouldBindJSON(&session); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set initial values for adaptive quiz
	session.StartTime = time.Now()
	session.Status = "active"
	session.CurrentStage = "easy"
	session.TotalQuestionsAsked = 0
	session.QuestionsUsed = []string{}
	session.FinalScore = 0

	if err := h.Service.CreateSession(context.Background(), &session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, session)
}

func (h *SessionHandler) UpdateSession(c *gin.Context) {
	id := c.Param("id")
	var update map[string]interface{}
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Service.UpdateSession(context.Background(), id, update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// SubmitAnswer handles answer submission with adaptive logic
func (h *SessionHandler) SubmitAnswer(c *gin.Context) {
	sessionID := c.Param("id")

	var answerData struct {
		QuestionID string `json:"question_id"`
		UserAnswer string `json:"user_answer"`
		IsCorrect  bool   `json:"is_correct"`
		TimeSpent  int    `json:"time_spent_seconds"`
	}

	if err := c.ShouldBindJSON(&answerData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Process answer through adaptive logic
	result, err := h.Service.ProcessAnswer(
		context.Background(),
		sessionID,
		answerData.QuestionID,
		answerData.UserAnswer,
		answerData.IsCorrect,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Store the answer record
	answer := models.QuizAnswer{
		SessionID:        sessionID,
		QuestionID:       answerData.QuestionID,
		UserAnswer:       answerData.UserAnswer,
		IsCorrect:        answerData.IsCorrect,
		PointsEarned:     result.PointsEarned,
		TimeSpentSeconds: answerData.TimeSpent,
		AnsweredAt:       time.Now(),
	}

	if h.AnswerService != nil {
		_ = h.AnswerService.CreateAnswer(context.Background(), &answer)
	}

	// Return adaptive result
	response := gin.H{
		"is_correct":    result.IsCorrect,
		"points_earned": result.PointsEarned,
		"stage_update":  result.StageUpdate,
		"is_complete":   result.IsComplete,
	}

	if result.StageUpdate {
		response["next_stage"] = result.NextStage
	}

	c.JSON(http.StatusOK, response)
}

func (h *SessionHandler) NextQuestion(c *gin.Context) {
	sessionID := c.Param("id")

	// Get next question based on adaptive logic
	question, err := h.Service.GetNextQuestion(context.Background(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "No next question available",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, question)
}

func (h *SessionHandler) SubmitSession(c *gin.Context) {
	sessionID := c.Param("id")
	var submitData struct {
		CompletionType string  `json:"completion_type"`
		FinalScore     float64 `json:"final_score"`
	}
	if err := c.ShouldBindJSON(&submitData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.Service.SubmitSession(context.Background(), sessionID, submitData.CompletionType, submitData.FinalScore)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *SessionHandler) PauseSession(c *gin.Context) {
	sessionID := c.Param("id")
	var pauseData struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&pauseData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err := h.Service.PauseSession(context.Background(), sessionID, pauseData.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Session paused"})
}

// GetSessionStatus returns current adaptive session status
func (h *SessionHandler) GetSessionStatus(c *gin.Context) {
	sessionID := c.Param("id")

	session, err := h.Service.GetSession(context.Background(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	status := gin.H{
		"session_id":      session.ID,
		"current_stage":   session.CurrentStage,
		"total_questions": session.TotalQuestionsAsked,
		"current_score":   session.FinalScore,
		"status":          session.Status,
		"stage_progress":  session.StageProgress,
	}

	c.JSON(http.StatusOK, status)
}

// GetQuizPoolInfo returns information about the quiz question pool
func (h *SessionHandler) GetQuizPoolInfo(c *gin.Context) {
	quizID := c.Query("quiz_id")
	skillID := c.Query("skill_id")

	if quizID == "" || skillID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "quiz_id and skill_id are required",
		})
		return
	}

	poolInfo, err := h.Service.GetQuizPoolInfo(context.Background(), quizID, skillID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, poolInfo)
}

// PreloadQuestions allows pre-loading questions for a stage
func (h *SessionHandler) PreloadQuestions(c *gin.Context) {
	var request struct {
		QuizID     string   `json:"quiz_id"`
		SkillID    string   `json:"skill_id"`
		Stage      string   `json:"stage"`
		Count      int      `json:"count"`
		ExcludeIDs []string `json:"exclude_ids"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	questions, err := h.Service.SelectQuestionsForStage(
		context.Background(),
		request.QuizID,
		request.SkillID,
		request.Stage,
		request.Count,
		request.ExcludeIDs,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"questions": questions,
		"count":     len(questions),
		"stage":     request.Stage,
	})
}

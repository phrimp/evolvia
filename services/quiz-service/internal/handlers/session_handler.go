package handlers

import (
	"context"
	"net/http"

	"quiz-service/internal/models"
	"quiz-service/internal/service"

	"github.com/gin-gonic/gin"
)

type SessionHandler struct {
	Service *service.SessionService
}

func NewSessionHandler(s *service.SessionService) *SessionHandler {
	return &SessionHandler{Service: s}
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

func (h *SessionHandler) NextQuestion(c *gin.Context) {
	sessionID := c.Param("id")
	session, err := h.Service.GetSession(context.Background(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}
	question, err := h.Service.GetNextQuestion(context.Background(), session.QuizID, session.AnsweredQuestionIDs)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No next question"})
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

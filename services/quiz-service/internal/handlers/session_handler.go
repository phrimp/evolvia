package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"quiz-service/internal/models"
	"quiz-service/internal/service"
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
	// Dummy next question handler
	c.JSON(200, gin.H{"message": "get next question"})
}

func (h *SessionHandler) SubmitSession(c *gin.Context) {
	// Dummy submit session handler
	c.JSON(200, gin.H{"message": "submit session"})
}

func (h *SessionHandler) PauseSession(c *gin.Context) {
	// Dummy pause session handler
	c.JSON(200, gin.H{"message": "pause session"})
}

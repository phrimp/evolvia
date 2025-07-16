package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"quiz-service/internal/models"
	"quiz-service/internal/service"
)

type AnswerHandler struct {
	Service *service.AnswerService
}

func NewAnswerHandler(s *service.AnswerService) *AnswerHandler {
	return &AnswerHandler{Service: s}
}

func (h *AnswerHandler) CreateAnswer(c *gin.Context) {
	var answer models.QuizAnswer
	if err := c.ShouldBindJSON(&answer); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Service.CreateAnswer(context.Background(), &answer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, answer)
}

func (h *AnswerHandler) GetAnswersBySession(c *gin.Context) {
	sessionID := c.Param("id")
	answers, err := h.Service.GetAnswersBySession(context.Background(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, answers)
}

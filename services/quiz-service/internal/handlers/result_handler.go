package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"quiz-service/internal/models"
	"quiz-service/internal/service"
)

type ResultHandler struct {
	Service *service.ResultService
}

func NewResultHandler(s *service.ResultService) *ResultHandler {
	return &ResultHandler{Service: s}
}

func (h *ResultHandler) GetResultBySession(c *gin.Context) {
	sessionID := c.Param("id")
	result, err := h.Service.GetResultBySession(context.Background(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Result not found"})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *ResultHandler) GetResultsByUser(c *gin.Context) {
	userID := c.Param("id")
	results, err := h.Service.GetResultsByUser(context.Background(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, results)
}

func (h *ResultHandler) GetResultsByQuiz(c *gin.Context) {
	quizID := c.Param("id")
	results, err := h.Service.GetResultsByQuiz(context.Background(), quizID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, results)
}

func (h *ResultHandler) CreateResult(c *gin.Context) {
	var result models.QuizResult
	if err := c.ShouldBindJSON(&result); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Service.CreateResult(context.Background(), &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, result)
}

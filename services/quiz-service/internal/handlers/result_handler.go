package handlers

import (
	"context"
	"net/http"
	"quiz-service/internal/models"
	"quiz-service/internal/service"

	"github.com/gin-gonic/gin"
)

type ResultHandler struct {
	Service        *service.ResultService
	SessionService *service.SessionService
}

func NewResultHandler(s *service.ResultService, sessionService *service.SessionService) *ResultHandler {
	return &ResultHandler{Service: s, SessionService: sessionService}
}

func (h *ResultHandler) GetResultBySession(c *gin.Context) {
	sessionID := c.Param("sessionID")
	result, err := h.Service.GetResultBySession(context.Background(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Result not found"})
		return
	}
	detail_answers := []models.CachedAnswer{}
	if detail_answers_cache, ok := h.SessionService.GetCachedAnswers(sessionID); ok {
		detail_answers = detail_answers_cache
	}

	c.JSON(http.StatusOK, gin.H{
		"result":         result,
		"detail_answers": detail_answers,
	})
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
	if _, err := h.Service.CreateResult(context.Background(), &result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, result)
}

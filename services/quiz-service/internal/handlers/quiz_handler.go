package handlers

import (
	"context"
	"fmt"
	"net/http"

	"quiz-service/internal/models"
	"quiz-service/internal/service"

	"github.com/gin-gonic/gin"
)

type QuizHandler struct {
	Service *service.QuizService
}

func NewQuizHandler(s *service.QuizService) *QuizHandler {
	return &QuizHandler{Service: s}
}

func (h *QuizHandler) ListQuizzes(c *gin.Context) {
	quizzes, err := h.Service.ListQuizzes(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, quizzes)
}

func (h *QuizHandler) GetQuiz(c *gin.Context) {
	id := c.Param("id")
	quiz, err := h.Service.GetQuiz(context.Background(), id)
	fmt.Println("Fetching quiz with ID:", id)
	fmt.Println("Quiz details:", quiz)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Quiz not found"})
		return
	}
	c.JSON(http.StatusOK, quiz)
}

func (h *QuizHandler) CreateQuiz(c *gin.Context) {
	var quiz models.Quiz
	if err := c.ShouldBindJSON(&quiz); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Service.CreateQuiz(context.Background(), &quiz); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, quiz)
}

func (h *QuizHandler) UpdateQuiz(c *gin.Context) {
	id := c.Param("id")
	var update map[string]interface{}
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Service.UpdateQuiz(context.Background(), id, update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func (h *QuizHandler) DeleteQuiz(c *gin.Context) {
	id := c.Param("id")
	if err := h.Service.DeleteQuiz(context.Background(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

package handlers

import (
	"net/http"
	"quiz-service/internal/service"

	"github.com/gin-gonic/gin"
)

// DEPRECATED: AnswerHandler is deprecated in favor of cache-based answer handling in SessionHandler
// Individual answers are now managed through SessionService caching instead of direct database operations
// Use SessionHandler.GetSessionAnswers() for retrieving session answers
// This handler is maintained for backward compatibility only
type AnswerHandler struct {
	Service *service.AnswerService
}

// DEPRECATED: Use SessionHandler methods instead for answer management
func NewAnswerHandler(s *service.AnswerService) *AnswerHandler {
	return &AnswerHandler{Service: s}
}

// DEPRECATED: Individual answer creation is now handled automatically in SessionHandler.SubmitAnswer()
// Answers are cached in memory instead of being directly created via this endpoint
func (h *AnswerHandler) CreateAnswer(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"error":   "Direct answer creation is deprecated",
		"message": "Answers are now automatically cached when submitted through session endpoints",
		"migration_info": gin.H{
			"use_instead": "POST /protected/quizz/session/{id}/answer",
			"reason":      "Individual answers are now cached automatically during session interaction",
		},
	})
}

// DEPRECATED: Use SessionHandler.GetSessionAnswers() which retrieves from cache
// This method accesses potentially stale database data
func (h *AnswerHandler) GetAnswersBySession(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"error":   "Direct answer retrieval is deprecated",
		"message": "Answer retrieval is now available through session endpoints with enhanced caching",
		"migration_info": gin.H{
			"use_instead": "GET /protected/quizz/session/{id}/answers",
			"benefits":    []string{"Real-time data", "Enhanced metadata", "Better performance"},
		},
	})
}

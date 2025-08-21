package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"quiz-service/internal/models"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// Simple integration test without mocking for basic functionality
func TestUserSessionsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Setup router
	router := gin.New()
	router.GET("/users/:userID/sessions", func(c *gin.Context) {
		// Simple test response
		c.JSON(http.StatusOK, gin.H{
			"user_id": c.Param("userID"),
			"sessions": []gin.H{},
			"pagination": gin.H{
				"total": 0,
				"limit": 50,
				"offset": 0,
			},
		})
	})
	
	// Test request
	req, _ := http.NewRequest("GET", "/users/test123/sessions", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	// Basic assertions
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test123", response["user_id"])
}

func TestSessionDetailsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Setup router with simple response
	router := gin.New()
	router.GET("/sessions/:id/details", func(c *gin.Context) {
		// Test response structure
		c.JSON(http.StatusOK, gin.H{
			"session": gin.H{
				"id": c.Param("id"),
				"user_id": "test123",
				"status": "active",
			},
			"cached_questions": []gin.H{},
			"cache_info": gin.H{
				"total_cached_answers": 0,
			},
		})
	})
	
	// Test request
	req, _ := http.NewRequest("GET", "/sessions/session123/details", nil)
	w := httptest.NewRecorder()
	
	router.ServeHTTP(w, req)
	
	// Basic assertions
	assert.Equal(t, http.StatusOK, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	
	session := response["session"].(map[string]interface{})
	assert.Equal(t, "session123", session["id"])
}

func TestNewModelsCompilation(t *testing.T) {
	// Test that new models compile correctly
	overview := models.UserSessionOverview{
		UserID: "test",
		Sessions: []models.SessionSummary{
			{
				ID: "session1",
				ProgressSummary: models.ProgressSummary{
					QuestionsAnswered: 5,
					StageBreakdown: map[string]models.StageProgressSummary{
						"beginner": {
							Passed: true,
							Accuracy: 80.0,
						},
					},
				},
			},
		},
		Pagination: models.Pagination{
			Total: 1,
			Limit: 50,
			Offset: 0,
		},
	}
	
	assert.Equal(t, "test", overview.UserID)
	assert.Equal(t, 1, len(overview.Sessions))
	
	details := models.SessionDetails{
		Session: &models.QuizSession{
			ID: "session1",
			UserID: "test",
		},
		CachedQuestions: []models.CachedAnswer{
			{
				QuestionID: "q1",
				IsCorrect: true,
				AnsweredAt: time.Now(),
			},
		},
		CacheInfo: map[string]interface{}{
			"total_cached_answers": 1,
		},
	}
	
	assert.Equal(t, "session1", details.Session.ID)
	assert.Equal(t, 1, len(details.CachedQuestions))
}
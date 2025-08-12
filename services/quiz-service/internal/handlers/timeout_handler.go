package handlers

import (
	"net/http"
	"strconv"
	"quiz-service/internal/service"

	"github.com/gin-gonic/gin"
)

// TimeoutHandler handles session timeout-related HTTP requests
type TimeoutHandler struct {
	SessionService *service.SessionService
}

// NewTimeoutHandler creates a new timeout handler
func NewTimeoutHandler(sessionService *service.SessionService) *TimeoutHandler {
	return &TimeoutHandler{
		SessionService: sessionService,
	}
}

// GetSessionTimeoutInfo returns timeout information for a session
func (h *TimeoutHandler) GetSessionTimeoutInfo(c *gin.Context) {
	sessionID := c.Param("id")
	
	timeoutInfo := h.SessionService.GetSessionTimeoutInfo(sessionID)
	if timeoutInfo == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Session timeout information not found",
		})
		return
	}
	
	// Include current question state information
	questionInfo := h.SessionService.GetCurrentQuestionInfo(sessionID)
	
	response := gin.H{
		"timeout_info": timeoutInfo,
	}
	
	if questionInfo != nil {
		response["current_question"] = gin.H{
			"question_id":      questionInfo.CurrentQuestionID,
			"question_started": questionInfo.QuestionStarted,
			"is_answered":      questionInfo.IsAnswered,
			"question_type":    questionInfo.QuestionType,
			"stage":           questionInfo.Stage,
		}
	}
	
	c.JSON(http.StatusOK, response)
}

// ExtendSessionTimeout extends the timeout for a session
func (h *TimeoutHandler) ExtendSessionTimeout(c *gin.Context) {
	sessionID := c.Param("id")
	
	// Get additional minutes from request body or query parameter
	additionalMinutesStr := c.DefaultQuery("minutes", "15") // Default 15 minutes extension
	if body := c.PostForm("minutes"); body != "" {
		additionalMinutesStr = body
	}
	
	additionalMinutes, err := strconv.Atoi(additionalMinutesStr)
	if err != nil || additionalMinutes <= 0 || additionalMinutes > 120 { // Max 2 hour extension
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid additional minutes. Must be between 1 and 120.",
		})
		return
	}
	
	// Extend the timeout
	h.SessionService.ExtendSessionTimeout(sessionID, additionalMinutes)
	
	// Get updated timeout info
	timeoutInfo := h.SessionService.GetSessionTimeoutInfo(sessionID)
	
	c.JSON(http.StatusOK, gin.H{
		"message":      "Session timeout extended successfully",
		"extended_by":  additionalMinutes,
		"timeout_info": timeoutInfo,
	})
}

// GetCurrentQuestionState returns the current question state for a session
func (h *TimeoutHandler) GetCurrentQuestionState(c *gin.Context) {
	sessionID := c.Param("id")
	
	questionInfo := h.SessionService.GetCurrentQuestionInfo(sessionID)
	if questionInfo == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No active question found for session",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"question_state": gin.H{
			"session_id":       questionInfo.SessionID,
			"question_id":      questionInfo.CurrentQuestionID,
			"question_started": questionInfo.QuestionStarted,
			"is_answered":      questionInfo.IsAnswered,
			"answer_submitted": questionInfo.AnswerSubmitted,
			"question_type":    questionInfo.QuestionType,
			"stage":           questionInfo.Stage,
		},
	})
}

// GetQuestionStateStats returns overall question state cache statistics
func (h *TimeoutHandler) GetQuestionStateStats(c *gin.Context) {
	stats := h.SessionService.GetQuestionStateStats()
	
	c.JSON(http.StatusOK, gin.H{
		"question_state_stats": stats,
	})
}
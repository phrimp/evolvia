package handlers

import (
	"context"
	"net/http"
	"quiz-service/internal/models"
	"quiz-service/internal/service"
	"strconv"
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

// GetSession retrieves session information
func (h *SessionHandler) GetSession(c *gin.Context) {
	id := c.Param("id")
	session, err := h.Service.GetSession(context.Background(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}
	c.JSON(http.StatusOK, session)
}

// CreateSession creates a new adaptive quiz session
func (h *SessionHandler) CreateSession(c *gin.Context) {
	var req struct {
		QuizID    string   `json:"quiz_id" binding:"required"`
		SkillID   string   `json:"skill_id" binding:"required"`
		SkillName string   `json:"skill_name"`
		SkillTags []string `json:"skill_tags"`
		// New fields for initial mastery (first-time only)
		CurrentBloomLevel string `json:"current_bloom_level"` // e.g., "apply", "analyze"
		MasteryScore      int    `json:"mastery_score"`       // 0-10 scale
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Get user ID from header (set by auth middleware)
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User ID is required",
		})
		return
	}

	// Set default skill name if not provided
	if req.SkillName == "" {
		req.SkillName = req.SkillID
	}

	// Create session with skill validation
	session, err := h.Service.CreateSessionWithSkillValidationAndMastery(
		context.Background(),
		req.QuizID,
		userID,
		req.SkillID,
		req.SkillTags,
		req.SkillName,
		req.CurrentBloomLevel,
		req.MasteryScore,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create session",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"session":   session,
		"message":   "Session created successfully",
		"next_step": "Call /next endpoint to get first question",
	})
}

// CreateSimpleSession creates a session without skill validation (backward compatibility)
func (h *SessionHandler) CreateSimpleSession(c *gin.Context) {
	var session models.QuizSession
	if err := c.ShouldBindJSON(&session); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session.UserID = c.GetHeader("X-User-ID")
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

// UpdateSession updates session information
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
	c.JSON(http.StatusOK, gin.H{"message": "Session updated successfully"})
}

// SubmitAnswer handles answer submission with adaptive logic
func (h *SessionHandler) SubmitAnswer(c *gin.Context) {
	sessionID := c.Param("id")

	var answerData struct {
		QuestionID string `json:"question_id" binding:"required"`
		UserAnswer string `json:"user_answer" binding:"required"`
		IsCorrect  bool   `json:"is_correct"`
		TimeSpent  int    `json:"time_spent_seconds"`
	}

	if err := c.ShouldBindJSON(&answerData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid answer format",
			"details": err.Error(),
		})
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to process answer",
			"details": err.Error(),
		})
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

	// Return comprehensive adaptive result
	response := gin.H{
		"answer_processed": true,
		"is_correct":       result.IsCorrect,
		"points_earned":    result.PointsEarned,
		"stage_update":     result.StageUpdate,
		"is_complete":      result.IsComplete,
	}

	if result.StageUpdate {
		response["next_stage"] = result.NextStage
		response["stage_message"] = "Congratulations! Moving to next difficulty level"
	}

	if result.IsComplete {
		response["completion_message"] = "Quiz completed! All stages finished"
	}

	c.JSON(http.StatusOK, response)
}

// NextQuestion gets the next question based on adaptive criteria
func (h *SessionHandler) NextQuestion(c *gin.Context) {
	sessionID := c.Param("id")

	// Get next question based on adaptive logic
	question, err := h.Service.GetNextQuestion(context.Background(), sessionID)
	if err != nil {
		// Check if session is complete
		if err.Error() == "session is already completed" {
			c.JSON(http.StatusOK, gin.H{
				"completed": true,
				"message":   "Quiz session has been completed",
			})
			return
		}

		c.JSON(http.StatusNotFound, gin.H{
			"error":   "No next question available",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"question": question,
		"message":  "Next question retrieved successfully",
	})
}

// SubmitSession completes and submits the session
func (h *SessionHandler) SubmitSession(c *gin.Context) {
	sessionID := c.Param("id")
	var submitData struct {
		CompletionType string  `json:"completion_type"`
		FinalScore     float64 `json:"final_score"`
		ForceComplete  bool    `json:"force_complete"` // Allow manual completion
	}

	if err := c.ShouldBindJSON(&submitData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set default completion type
	if submitData.CompletionType == "" {
		submitData.CompletionType = "manual_submit"
	}

	result, err := h.Service.SubmitSession(
		context.Background(),
		sessionID,
		submitData.CompletionType,
		submitData.FinalScore,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to submit session",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result":  result,
		"message": "Session submitted successfully",
		"summary": h.generateSessionSummary(result),
	})
}

// PauseSession pauses an active session
func (h *SessionHandler) PauseSession(c *gin.Context) {
	sessionID := c.Param("id")
	var pauseData struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&pauseData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if pauseData.Reason == "" {
		pauseData.Reason = "user_requested"
	}

	err := h.Service.PauseSession(context.Background(), sessionID, pauseData.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to pause session",
			"details": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Session paused successfully",
		"reason":  pauseData.Reason,
	})
}

// ResumeSession resumes a paused session
func (h *SessionHandler) ResumeSession(c *gin.Context) {
	sessionID := c.Param("id")

	err := h.Service.ResumeSession(context.Background(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to resume session",
			"details": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "Session resumed successfully",
	})
}

// GetSessionStatus returns current adaptive session status
func (h *SessionHandler) GetSessionStatus(c *gin.Context) {
	sessionID := c.Param("id")

	status, err := h.Service.GetSessionStatus(context.Background(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Session not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    status,
		"timestamp": time.Now(),
	})
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
			"error":   "Failed to get pool info",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pool_info": poolInfo,
		"quiz_id":   quizID,
		"skill_id":  skillID,
	})
}

// PreloadQuestions allows pre-loading questions for a stage
func (h *SessionHandler) PreloadQuestions(c *gin.Context) {
	var request struct {
		QuizID     string   `json:"quiz_id" binding:"required"`
		SkillID    string   `json:"skill_id" binding:"required"`
		Stage      string   `json:"stage" binding:"required"`
		Count      int      `json:"count"`
		ExcludeIDs []string `json:"exclude_ids"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Default count
	if request.Count == 0 {
		request.Count = 5
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to preload questions",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"questions": questions,
		"count":     len(questions),
		"stage":     request.Stage,
		"message":   "Questions preloaded successfully",
	})
}

// GetSessionAnswers retrieves all answers for a session
func (h *SessionHandler) GetSessionAnswers(c *gin.Context) {
	sessionID := c.Param("id")

	if h.AnswerService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Answer service not available",
		})
		return
	}

	answers, err := h.AnswerService.GetAnswersBySession(context.Background(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get answers",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"answers":    answers,
		"count":      len(answers),
		"session_id": sessionID,
	})
}

// GetSessionProgress provides detailed progress information
func (h *SessionHandler) GetSessionProgress(c *gin.Context) {
	sessionID := c.Param("id")

	session, err := h.Service.GetSession(context.Background(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	progress := h.calculateDetailedProgress(session)

	c.JSON(http.StatusOK, gin.H{
		"progress":   progress,
		"session_id": sessionID,
		"timestamp":  time.Now(),
	})
}

// ValidateSessionAccess checks if user has access to session
func (h *SessionHandler) ValidateSessionAccess(c *gin.Context) {
	sessionID := c.Param("id")
	userID := c.GetHeader("X-User-ID")

	session, err := h.Service.GetSession(context.Background(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	if session.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":      true,
		"session_id": sessionID,
		"user_id":    userID,
	})
}

// GetSessionStatistics provides session statistics
func (h *SessionHandler) GetSessionStatistics(c *gin.Context) {
	sessionID := c.Param("id")

	session, err := h.Service.GetSession(context.Background(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	stats := h.generateSessionStatistics(session)

	c.JSON(http.StatusOK, gin.H{
		"statistics": stats,
		"session_id": sessionID,
	})
}

// Helper methods

func (h *SessionHandler) generateSessionSummary(result *models.QuizResult) map[string]interface{} {
	return map[string]interface{}{
		"final_percentage":    result.Percentage,
		"badge_level":         result.BadgeLevel,
		"questions_attempted": result.QuestionsAttempted,
		"questions_correct":   result.QuestionsCorrect,
		"accuracy":            float64(result.QuestionsCorrect) / float64(result.QuestionsAttempted) * 100,
		"completion_type":     result.CompletionType,
	}
}

func (h *SessionHandler) calculateDetailedProgress(session *models.QuizSession) map[string]interface{} {
	totalPossibleQuestions := 15 // 5 per stage
	progressPercentage := float64(session.TotalQuestionsAsked) / float64(totalPossibleQuestions) * 100

	stageProgress := make(map[string]interface{})
	for stage, progress := range session.StageProgress {
		accuracy := 0.0
		if progress.Attempted > 0 {
			accuracy = float64(progress.Correct) / float64(progress.Attempted) * 100
		}

		stageProgress[stage] = map[string]interface{}{
			"attempted":   progress.Attempted,
			"correct":     progress.Correct,
			"accuracy":    accuracy,
			"passed":      progress.Passed,
			"score":       progress.Score,
			"in_recovery": progress.RecoveryRound > 0,
		}
	}

	return map[string]interface{}{
		"overall_progress":   progressPercentage,
		"current_stage":      session.CurrentStage,
		"questions_answered": session.TotalQuestionsAsked,
		"stage_breakdown":    stageProgress,
		"current_score":      session.FinalScore,
		"session_duration":   time.Since(session.StartTime).Minutes(),
		"status":             session.Status,
	}
}

func (h *SessionHandler) generateSessionStatistics(session *models.QuizSession) map[string]interface{} {
	stats := map[string]interface{}{
		"session_id":       session.ID,
		"quiz_id":          session.QuizID,
		"user_id":          session.UserID,
		"start_time":       session.StartTime,
		"current_stage":    session.CurrentStage,
		"total_questions":  session.TotalQuestionsAsked,
		"current_score":    session.FinalScore,
		"status":           session.Status,
		"questions_used":   len(session.QuestionsUsed),
		"session_duration": time.Since(session.StartTime).String(),
	}

	// Add stage-specific statistics
	stageStats := make(map[string]interface{})
	for stage, progress := range session.StageProgress {
		stageStats[stage] = map[string]interface{}{
			"questions_attempted": progress.Attempted,
			"correct_answers":     progress.Correct,
			"current_score":       progress.Score,
			"is_passed":           progress.Passed,
			"recovery_rounds":     progress.RecoveryRound,
		}
	}
	stats["stage_statistics"] = stageStats

	// Add skill information if available
	if session.Metadata != nil {
		if skillID, ok := session.Metadata["skill_id"]; ok {
			stats["skill_id"] = skillID
		}
		if skillName, ok := session.Metadata["skill_name"]; ok {
			stats["skill_name"] = skillName
		}
		if skillTags, ok := session.Metadata["skill_tags"]; ok {
			stats["skill_tags"] = skillTags
		}
	}

	return stats
}

// Health check endpoint for session service
func (h *SessionHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service":   "quiz-session-service",
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.0.0",
	})
}

// GetBatchSessions retrieves multiple sessions (for admin purposes)
func (h *SessionHandler) GetBatchSessions(c *gin.Context) {
	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 50
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		offset = 0
	}

	// Note: This would require implementing batch retrieval in the service
	// For now, return a placeholder response
	c.JSON(http.StatusOK, gin.H{
		"message": "Batch session retrieval not yet implemented",
		"limit":   limit,
		"offset":  offset,
	})
}

package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"quiz-service/internal/integrity"
	"quiz-service/internal/models"
	"quiz-service/internal/selection"
	"quiz-service/internal/service"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type SessionHandler struct {
	Service          *service.SessionService
	QuestionService  *service.QuestionService
	IntegrityMonitor *integrity.TimeIntegrityMonitor
}

func NewSessionHandler(s *service.SessionService, qs *service.QuestionService) *SessionHandler {
	return &SessionHandler{
		Service:          s,
		QuestionService:  qs,
		IntegrityMonitor: integrity.NewTimeIntegrityMonitor(nil), // Use default config
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

// CreateGlobalSession creates a new adaptive session using global configuration (no quiz dependency)
func (h *SessionHandler) CreateGlobalSession(c *gin.Context) {
	// Generate request tracking ID for logging
	requestStart := time.Now()
	requestID := fmt.Sprintf("req_%d", time.Now().UnixNano())
	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")

	log.Printf("[SESSION_HANDLER] [%s] → POST /create-global-session from IP: %s", requestID, clientIP)
	log.Printf("[SESSION_HANDLER] [%s] User-Agent: %s", requestID, userAgent)

	var req struct {
		ConfigID  string `json:"config_id"` // Optional: uses default if empty
		SkillID   string `json:"skill_id" binding:"required"`
		SkillName string `json:"skill_name"`

		// Categorized tags for weighted selection
		PrimaryTags   []string `json:"primary_tags"`   // Core skill concepts
		SecondaryTags []string `json:"secondary_tags"` // Supporting concepts
		RelatedTags   []string `json:"related_tags"`   // Peripheral concepts

		// Optional: Backward compatibility with single tag list
		SkillTags []string `json:"skill_tags"` // Legacy field

		// Tag weight configuration
		TagWeights struct {
			PrimaryWeight   float64 `json:"primary_weight"`
			SecondaryWeight float64 `json:"secondary_weight"`
			RelatedWeight   float64 `json:"related_weight"`
			ExactMatchBonus float64 `json:"exact_match_bonus"`
		} `json:"tag_weights"`

		// Initial mastery fields
		CurrentBloomLevel    string   `json:"current_bloom_level"`
		PreferredBloomLevels []string `json:"preferred_bloom_levels"`
		MasteryScore         int      `json:"mastery_score"`
	}

	// Parse and validate request body
	log.Printf("[SESSION_HANDLER] [%s] Parsing request body", requestID)
	parseStart := time.Now()
	if err := c.ShouldBindJSON(&req); err != nil {
		parseDuration := time.Since(parseStart)
		log.Printf("[SESSION_HANDLER] [%s] ❌ ERROR: Invalid request format (took %v): %v",
			requestID, parseDuration, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Invalid request format",
			"details":    err.Error(),
			"request_id": requestID,
		})
		return
	}
	parseDuration := time.Since(parseStart)
	log.Printf("[SESSION_HANDLER] [%s] Request parsed successfully (took %v)", requestID, parseDuration)
	log.Printf("[SESSION_HANDLER] [%s] Request parameters - SkillID: '%s', ConfigID: '%s', MasteryScore: %d",
		requestID, req.SkillID, req.ConfigID, req.MasteryScore)

	// Extract and validate user ID
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		log.Printf("[SESSION_HANDLER] [%s] ❌ ERROR: Missing X-User-ID header", requestID)
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "User ID is required",
			"request_id": requestID,
		})
		return
	}
	log.Printf("[SESSION_HANDLER] [%s] Authenticated user: %s", requestID, userID)

	// Handle backward compatibility - if old skill_tags field is used
	if len(req.PrimaryTags) == 0 && len(req.SkillTags) > 0 {
		// Put all tags as primary for backward compatibility
		req.PrimaryTags = req.SkillTags
		log.Printf("[SESSION_HANDLER] [%s] Using legacy skill_tags field, treating %d tags as primary",
			requestID, len(req.SkillTags))
	}

	// Validate we have at least some tags
	totalTags := len(req.PrimaryTags) + len(req.SecondaryTags) + len(req.RelatedTags)
	if totalTags == 0 {
		log.Printf("[SESSION_HANDLER] [%s] ❌ ERROR: No tags provided in request", requestID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "At least one tag (primary, secondary, or related) is required",
			"request_id": requestID,
		})
		return
	}
	log.Printf("[SESSION_HANDLER] [%s] Tag validation passed - Total: %d (P:%d, S:%d, R:%d)",
		requestID, totalTags, len(req.PrimaryTags), len(req.SecondaryTags), len(req.RelatedTags))

	// Set default weights if not provided
	weightsProvided := req.TagWeights.PrimaryWeight > 0 || req.TagWeights.SecondaryWeight > 0 ||
		req.TagWeights.RelatedWeight > 0 || req.TagWeights.ExactMatchBonus > 0

	if req.TagWeights.PrimaryWeight == 0 {
		req.TagWeights.PrimaryWeight = 3.0
	}
	if req.TagWeights.SecondaryWeight == 0 {
		req.TagWeights.SecondaryWeight = 1.5
	}
	if req.TagWeights.RelatedWeight == 0 {
		req.TagWeights.RelatedWeight = 0.5
	}
	if req.TagWeights.ExactMatchBonus == 0 {
		req.TagWeights.ExactMatchBonus = 2.0
	}

	if weightsProvided {
		log.Printf("[SESSION_HANDLER] [%s] Using provided tag weights - P:%.1f, S:%.1f, R:%.1f, EB:%.1f",
			requestID, req.TagWeights.PrimaryWeight, req.TagWeights.SecondaryWeight,
			req.TagWeights.RelatedWeight, req.TagWeights.ExactMatchBonus)
	} else {
		log.Printf("[SESSION_HANDLER] [%s] Using default tag weights - P:%.1f, S:%.1f, R:%.1f, EB:%.1f",
			requestID, req.TagWeights.PrimaryWeight, req.TagWeights.SecondaryWeight,
			req.TagWeights.RelatedWeight, req.TagWeights.ExactMatchBonus)
	}

	// Set default skill name
	if req.SkillName == "" {
		req.SkillName = req.SkillID
		log.Printf("[SESSION_HANDLER] [%s] Using SkillID as skill name: '%s'", requestID, req.SkillName)
	} else {
		log.Printf("[SESSION_HANDLER] [%s] Using provided skill name: '%s'", requestID, req.SkillName)
	}

	// Create enhanced skill info
	enhancedSkillInfo := &selection.EnhancedSkillInfo{
		ID:            req.SkillID,
		Name:          req.SkillName,
		PrimaryTags:   req.PrimaryTags,
		SecondaryTags: req.SecondaryTags,
		RelatedTags:   req.RelatedTags,
		TagWeights: selection.TagWeightConfig{
			PrimaryWeight:   req.TagWeights.PrimaryWeight,
			SecondaryWeight: req.TagWeights.SecondaryWeight,
			RelatedWeight:   req.TagWeights.RelatedWeight,
			ExactMatchBonus: req.TagWeights.ExactMatchBonus,
		},
	}

	// Log enhanced skill info creation
	log.Printf("[SESSION_HANDLER] [%s] Creating enhanced skill info object", requestID)
	log.Printf("[SESSION_HANDLER] [%s] Tag distribution - Primary: %d, Secondary: %d, Related: %d",
		requestID, len(req.PrimaryTags), len(req.SecondaryTags), len(req.RelatedTags))
	log.Printf("[SESSION_HANDLER] [%s] Tag details - Primary: %v", requestID, req.PrimaryTags)
	log.Printf("[SESSION_HANDLER] [%s] Tag details - Secondary: %v", requestID, req.SecondaryTags)
	log.Printf("[SESSION_HANDLER] [%s] Tag details - Related: %v", requestID, req.RelatedTags)

	// Determine bloom levels - prefer new format, fallback to legacy
	var bloomLevels []string
	if len(req.PreferredBloomLevels) > 0 {
		bloomLevels = req.PreferredBloomLevels
		log.Printf("[SESSION_HANDLER] [%s] Using preferred Bloom levels: %v", requestID, bloomLevels)
	} else if req.CurrentBloomLevel != "" {
		bloomLevels = []string{req.CurrentBloomLevel}
		log.Printf("[SESSION_HANDLER] [%s] Using current Bloom level: '%s'", requestID, req.CurrentBloomLevel)
	} else {
		log.Printf("[SESSION_HANDLER] [%s] No Bloom levels specified, will use service defaults", requestID)
	}

	// Create global session
	log.Printf("[SESSION_HANDLER] [%s] Calling SessionService.CreateGlobalSession", requestID)
	sessionCreateStart := time.Now()
	session, err := h.Service.CreateGlobalSession(
		context.Background(),
		userID,
		enhancedSkillInfo,
		bloomLevels,
		req.MasteryScore,
		req.ConfigID, // Uses default if empty
	)
	sessionCreateDuration := time.Since(sessionCreateStart)
	if err != nil {
		log.Printf("[SESSION_HANDLER] [%s] ❌ ERROR: Session creation failed (took %v): %v",
			requestID, sessionCreateDuration, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":       "Failed to create global session",
			"details":     err.Error(),
			"request_id":  requestID,
			"duration_ms": sessionCreateDuration.Milliseconds(),
		})
		return
	}
	log.Printf("[SESSION_HANDLER] [%s] ✅ Session created successfully (took %v) - ID: %s",
		requestID, sessionCreateDuration, session.ID)

	// Return successful response
	totalRequestDuration := time.Since(requestStart)
	log.Printf("[SESSION_HANDLER] [%s] ✅ SUCCESS: Global session creation completed", requestID)
	log.Printf("[SESSION_HANDLER] [%s] Performance summary - Total: %v, SessionCreate: %v, Parse: %v",
		requestID, totalRequestDuration, sessionCreateDuration, parseDuration)
	log.Printf("[SESSION_HANDLER] [%s] Final session details - ID: %s, UserID: %s, ConfigID: %s",
		requestID, session.ID, userID, session.ConfigID)

	responseData := gin.H{
		"message":    "Global session created successfully",
		"session":    session,
		"mode":       "global", // Indicate this is using global configuration
		"request_id": requestID,
		"performance": gin.H{
			"total_duration_ms":            totalRequestDuration.Milliseconds(),
			"session_creation_duration_ms": sessionCreateDuration.Milliseconds(),
			"request_parse_duration_ms":    parseDuration.Milliseconds(),
		},
		"metadata": gin.H{
			"client_ip":  clientIP,
			"total_tags": totalTags,
			"tag_distribution": gin.H{
				"primary":   len(req.PrimaryTags),
				"secondary": len(req.SecondaryTags),
				"related":   len(req.RelatedTags),
			},
			"weights_provided": weightsProvided,
			"bloom_levels":     bloomLevels,
		},
	}

	c.JSON(http.StatusCreated, responseData)
	log.Printf("[SESSION_HANDLER] [%s] → Response sent (201 Created) - SessionID: %s", requestID, session.ID)
}

// UpdateSession updates session information
func (h *SessionHandler) UpdateSession(c *gin.Context) {
	id := c.Param("id")
	var update map[string]any
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
		// For true/false questions, support boolean answers
		BooleanAnswer *bool `json:"boolean_answer,omitempty"`
	}

	if err := c.ShouldBindJSON(&answerData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid answer format",
			"details": err.Error(),
		})
		return
	}

	// Fetch the question for validation and Bloom scoring
	question, err := h.QuestionService.GetQuestion(context.Background(), answerData.QuestionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Question not found",
			"details": err.Error(),
		})
		return
	}

	// Validate question structure
	if err := question.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid question structure",
			"details": err.Error(),
		})
		return
	}

	// Ensure question has Bloom scores calculated using centralized service
	question.EnsureBloomScoresWithService(h.Service.GetBloomScoringService())

	// Validate and determine correctness based on question type
	var isAnswerCorrect bool
	switch models.QuestionType(question.Type) {
	case models.QuestionTypeTrueFalse:
		// For true/false questions, support both string and boolean answers
		if answerData.BooleanAnswer != nil {
			isAnswerCorrect = question.IsCorrectBooleanAnswer(*answerData.BooleanAnswer)
			// Convert boolean to string for consistent storage
			if *answerData.BooleanAnswer {
				answerData.UserAnswer = "true"
			} else {
				answerData.UserAnswer = "false"
			}
		} else {
			isAnswerCorrect = question.IsCorrectAnswer(answerData.UserAnswer)
		}
	case models.QuestionTypeMultipleChoice, models.QuestionTypeSingleChoice:
		isAnswerCorrect = question.IsCorrectAnswer(answerData.UserAnswer)
	default:
		// Fallback for legacy questions
		isAnswerCorrect = question.IsCorrectAnswer(answerData.UserAnswer)
	}

	// Override the provided isCorrect with our validation
	answerData.IsCorrect = isAnswerCorrect

	// Validate timing integrity before processing answer
	violations := h.IntegrityMonitor.ValidateQuestionTiming(
		sessionID,
		question,
		answerData.TimeSpent,
		answerData.IsCorrect,
	)

	// Handle critical violations (terminate session)
	for _, violation := range violations {
		if violation.Severity == "critical" {
			// Log critical violation
			fmt.Printf("[INTEGRITY VIOLATION] Critical timing violation in session %s: %s\n",
				sessionID, violation.Description)

			// Terminate session for critical violations
			_ = h.Service.PauseSession(context.Background(), sessionID,
				fmt.Sprintf("integrity_violation_%s", violation.Type))

			c.JSON(http.StatusForbidden, gin.H{
				"error":     "Session terminated due to integrity violation",
				"violation": violation,
				"action":    "session_terminated",
			})
			return
		}
	}

	// Process answer through adaptive logic with question object
	result, err := h.Service.ProcessAnswer(
		context.Background(),
		sessionID,
		answerData.QuestionID,
		question,
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

	// Answer is now cached automatically in ProcessAnswer method with stage and recovery information
	// Update the cached answer with timing information from the handler
	if cachedAnswers, exists := h.Service.GetCachedAnswers(sessionID); exists && len(cachedAnswers) > 0 {
		// Get the last cached answer (the one we just added)
		lastAnswer := &cachedAnswers[len(cachedAnswers)-1]
		lastAnswer.TimeSpentSeconds = answerData.TimeSpent
		// Note: We could update the cache here, but for now timing from handler is less critical
	}

	// Return comprehensive adaptive result with integrity and question type information
	response := gin.H{
		"answer_processed": true,
		"is_correct":       result.IsCorrect,
		"points_earned":    result.PointsEarned,
		"stage_update":     result.StageUpdate,
		"is_complete":      result.IsComplete,
		"question_type":    question.Type,
		"user_answer":      answerData.UserAnswer,
	}

	// Add sensitive fields that were hidden in GET /next-question for security
	// These are now safe to reveal after the user has submitted their answer
	sensitiveFields := question.GetSensitiveFields()
	for key, value := range sensitiveFields {
		response[key] = value
	}

	if result.StageUpdate {
		response["next_stage"] = result.NextStage
		response["stage_message"] = "Congratulations! Moving to next difficulty level"
	}

	if result.IsComplete {
		response["completion_message"] = "Quiz completed! All stages finished"
	}

	// Add integrity monitoring information for non-critical violations
	if len(violations) > 0 {
		response["integrity_warnings"] = violations
		response["integrity_message"] = "Timing patterns are being monitored for quiz integrity"
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

	// Validate question is not nil before calling methods
	if question == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal error: question data is invalid",
			"details": "Retrieved question is nil",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"question": question.ToSafeResponse(), // Use sanitized response to hide sensitive fields
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
		submitData.CompletionType = models.ManualSubmit
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

	// Validate result is not nil before generating summary
	if result == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Internal error: session result is invalid",
			"details": "Retrieved result is nil",
		})
		return
	}

	detail_answers := []models.CachedAnswer{}
	if detail_answers_cache, ok := h.Service.GetCachedAnswers(sessionID); ok {
		detail_answers = detail_answers_cache
	}

	c.JSON(http.StatusOK, gin.H{
		"result":         result,
		"detail_answers": detail_answers,
		"message":        "Session submitted successfully",
		"summary":        h.generateSessionSummary(result),
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
	// Generate request tracking ID
	handlerStart := time.Now()
	requestID := fmt.Sprintf("status_handler_%d", time.Now().UnixNano())
	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	userID := c.GetHeader("X-User-ID")

	sessionID := c.Param("id")
	log.Printf("[SESSION_STATUS_HANDLER] [%s] → GET /session/%s/status from IP: %s",
		requestID, sessionID, clientIP)
	log.Printf("[SESSION_STATUS_HANDLER] [%s] User-Agent: %s, UserID: %s",
		requestID, userAgent, userID)

	// Validate session ID parameter
	if sessionID == "" {
		log.Printf("[SESSION_STATUS_HANDLER] [%s] ❌ ERROR: Missing session ID parameter", requestID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Session ID is required",
			"request_id": requestID,
		})
		return
	}
	log.Printf("[SESSION_STATUS_HANDLER] [%s] Processing status request for session: %s",
		requestID, sessionID)

	// Call service layer
	log.Printf("[SESSION_STATUS_HANDLER] [%s] Calling SessionService.GetSessionStatus", requestID)
	serviceCallStart := time.Now()
	status, err := h.Service.GetSessionStatus(context.Background(), sessionID)
	serviceCallDuration := time.Since(serviceCallStart)

	if err != nil {
		log.Printf("[SESSION_STATUS_HANDLER] [%s] ❌ ERROR: Service call failed (took %v): %v",
			requestID, serviceCallDuration, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error":       "Session not found",
			"details":     err.Error(),
			"request_id":  requestID,
			"duration_ms": serviceCallDuration.Milliseconds(),
		})
		return
	}

	log.Printf("[SESSION_STATUS_HANDLER] [%s] Service call successful (took %v)",
		requestID, serviceCallDuration)

	// Log key status information
	log.Printf("[SESSION_STATUS_HANDLER] [%s] Status details - CurrentStage: %v, IsComplete: %v",
		requestID, status["current_stage"], status["is_complete"])
	if timeElapsed, exists := status["time_elapsed"]; exists {
		log.Printf("[SESSION_STATUS_HANDLER] [%s] Timing - Elapsed: %vs, Remaining: %vs",
			requestID, timeElapsed, status["time_remaining"])
	}
	if skillInfo, exists := status["skill_info"]; exists {
		if skill, ok := skillInfo.(map[string]interface{}); ok {
			// Safely access Tags to prevent panic
			tagsLen := 0
			if tags, tagsOk := skill["Tags"]; tagsOk {
				if tagsSlice, isSlice := tags.([]string); isSlice {
					tagsLen = len(tagsSlice)
				}
			}
			log.Printf("[SESSION_STATUS_HANDLER] [%s] Skill info - Name: %v, Tags: %d",
				requestID, skill["Name"], tagsLen)
		}
	}

	// Prepare response
	totalHandlerDuration := time.Since(handlerStart)
	responseData := gin.H{
		"status":     status,
		"timestamp":  time.Now(),
		"request_id": requestID,
		"performance": gin.H{
			"handler_duration_ms":      totalHandlerDuration.Milliseconds(),
			"service_call_duration_ms": serviceCallDuration.Milliseconds(),
		},
		"metadata": gin.H{
			"client_ip": clientIP,
			"user_id":   userID,
		},
	}

	log.Printf("[SESSION_STATUS_HANDLER] [%s] ✅ Status request completed successfully (took %v)",
		requestID, totalHandlerDuration)
	log.Printf("[SESSION_STATUS_HANDLER] [%s] Performance - Handler: %v, Service: %v",
		requestID, totalHandlerDuration, serviceCallDuration)

	c.JSON(http.StatusOK, responseData)
	log.Printf("[SESSION_STATUS_HANDLER] [%s] → Response sent (200 OK) - SessionID: %s",
		requestID, sessionID)
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

// GetSessionAnswers retrieves all cached answers for a session
func (h *SessionHandler) GetSessionAnswers(c *gin.Context) {
	sessionID := c.Param("id")

	// Get cached answers from session service
	cachedAnswers, exists := h.Service.GetCachedAnswers(sessionID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "No cached answers found for session",
			"message": "Session may not exist or answers have expired from cache",
		})
		return
	}

	// Convert cached answers to API-compatible format
	answers := make([]models.QuizAnswer, len(cachedAnswers))
	for i, cachedAnswer := range cachedAnswers {
		answers[i] = cachedAnswer.ToCachedAnswerResponse()
		answers[i].SessionID = sessionID // Set session ID for response
	}

	c.JSON(http.StatusOK, gin.H{
		"answers":           answers,
		"count":             len(answers),
		"session_id":        sessionID,
		"source":            "cache",
		"includes_timing":   true,
		"includes_metadata": true,
		"cache_info": gin.H{
			"total_cached": h.Service.GetCachedAnswerCount(sessionID),
			"retention":    "30 minutes after session completion",
		},
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

// generateSessionSummary creates session summary with comprehensive calculation logging
func (h *SessionHandler) generateSessionSummary(result *models.QuizResult) map[string]interface{} {
	summaryStart := time.Now()
	summaryID := fmt.Sprintf("summary_%d", time.Now().UnixNano())
	log.Printf("[SESSION_SUMMARY] [%s] Starting session summary generation", summaryID)

	// Defensive nil check (though caller should validate)
	if result == nil {
		log.Printf("[SESSION_SUMMARY] [%s] ERROR: Result is nil", summaryID)
		return map[string]interface{}{
			"error": "Invalid result data",
		}
	}

	log.Printf("[SESSION_SUMMARY] [%s] Input data - SessionID: %s, UserID: %s, FinalScore: %.2f",
		summaryID, result.SessionID, result.UserID, result.FinalScore)
	log.Printf("[SESSION_SUMMARY] [%s] Question stats - Attempted: %d, Correct: %d, BadgeLevel: %s",
		summaryID, result.QuestionsAttempted, result.QuestionsCorrect, result.BadgeLevel)

	// Calculate accuracy safely to avoid division by zero
	log.Printf("[SESSION_SUMMARY] [%s] Calculating accuracy", summaryID)
	accuracy := 0.0
	if result.QuestionsAttempted > 0 {
		accuracy = float64(result.QuestionsCorrect) / float64(result.QuestionsAttempted) * 100
		log.Printf("[SESSION_SUMMARY] [%s] Accuracy calculated: %.2f%% (%d/%d)",
			summaryID, accuracy, result.QuestionsCorrect, result.QuestionsAttempted)
	} else {
		log.Printf("[SESSION_SUMMARY] [%s] No questions attempted, accuracy remains 0%%", summaryID)
	}

	// Create summary object
	summary := map[string]interface{}{
		"final_percentage":    result.Percentage,
		"badge_level":         result.BadgeLevel,
		"questions_attempted": result.QuestionsAttempted,
		"questions_correct":   result.QuestionsCorrect,
		"accuracy":            accuracy,
		"completion_type":     result.CompletionType,
		"calculation_metadata": map[string]interface{}{
			"summary_id":             summaryID,
			"generation_duration_ms": time.Since(summaryStart).Milliseconds(),
			"accuracy_formula":       "(correct / attempted) * 100",
			"input_validation":       "passed",
		},
	}

	summaryDuration := time.Since(summaryStart)
	log.Printf("[SESSION_SUMMARY] [%s] ✅ Session summary generated successfully (took %v)",
		summaryID, summaryDuration)
	log.Printf("[SESSION_SUMMARY] [%s] Summary values - Percentage: %.2f%%, Badge: %s, Accuracy: %.2f%%",
		summaryID, result.Percentage, result.BadgeLevel, accuracy)

	return summary
}

// calculateDetailedProgress computes detailed session progress with comprehensive calculation logging
func (h *SessionHandler) calculateDetailedProgress(session *models.QuizSession) map[string]interface{} {
	progressStart := time.Now()
	progressID := fmt.Sprintf("progress_%d", time.Now().UnixNano())
	log.Printf("[DETAILED_PROGRESS] [%s] Starting detailed progress calculation for session %s",
		progressID, session.ID)

	// Calculate overall progress
	totalPossibleQuestions := 15 // 5 per stage
	log.Printf("[DETAILED_PROGRESS] [%s] Progress calculation - Questions asked: %d, Total possible: %d",
		progressID, session.TotalQuestionsAsked, totalPossibleQuestions)
	progressPercentage := float64(session.TotalQuestionsAsked) / float64(totalPossibleQuestions) * 100
	log.Printf("[DETAILED_PROGRESS] [%s] Overall progress: %.2f%% (%d/%d questions)",
		progressID, progressPercentage, session.TotalQuestionsAsked, totalPossibleQuestions)

	// Calculate stage-specific progress
	log.Printf("[DETAILED_PROGRESS] [%s] Calculating stage progress for %d stages",
		progressID, len(session.StageProgress))
	stageProgress := make(map[string]interface{})
	stagesProcessed := 0

	for stage, progress := range session.StageProgress {
		log.Printf("[DETAILED_PROGRESS] [%s] Processing stage '%s' - Attempted: %d, Correct: %d, Passed: %v",
			progressID, stage, progress.Attempted, progress.Correct, progress.Passed)

		// Calculate stage accuracy
		accuracy := 0.0
		if progress.Attempted > 0 {
			accuracy = float64(progress.Correct) / float64(progress.Attempted) * 100
			log.Printf("[DETAILED_PROGRESS] [%s] Stage '%s' accuracy: %.2f%% (%d/%d)",
				progressID, stage, accuracy, progress.Correct, progress.Attempted)
		} else {
			log.Printf("[DETAILED_PROGRESS] [%s] Stage '%s' not attempted, accuracy: 0%%",
				progressID, stage)
		}

		// Create stage progress entry
		stageEntry := map[string]interface{}{
			"attempted":      progress.Attempted,
			"correct":        progress.Correct,
			"accuracy":       accuracy,
			"passed":         progress.Passed,
			"score":          progress.Score,
			"in_recovery":    progress.RecoveryRound > 0,
			"recovery_round": progress.RecoveryRound,
			"calculation_details": map[string]interface{}{
				"accuracy_formula": "(correct / attempted) * 100",
				"raw_correct":      progress.Correct,
				"raw_attempted":    progress.Attempted,
				"recovery_status":  progress.RecoveryRound > 0,
			},
		}
		stageProgress[stage] = stageEntry
		stagesProcessed++

		log.Printf("[DETAILED_PROGRESS] [%s] Stage '%s' completed - Score: %.2f, InRecovery: %v",
			progressID, stage, progress.Score, progress.RecoveryRound > 0)
	}

	log.Printf("[DETAILED_PROGRESS] [%s] Processed %d stages successfully", progressID, stagesProcessed)

	// Calculate session duration
	sessionDurationMinutes := time.Since(session.StartTime).Minutes()
	log.Printf("[DETAILED_PROGRESS] [%s] Session duration: %.2f minutes (started: %v)",
		progressID, sessionDurationMinutes, session.StartTime)

	// Create comprehensive progress report
	progressReport := map[string]interface{}{
		"overall_progress":   progressPercentage,
		"current_stage":      session.CurrentStage,
		"questions_answered": session.TotalQuestionsAsked,
		"stage_breakdown":    stageProgress,
		"current_score":      session.FinalScore,
		"session_duration":   sessionDurationMinutes,
		"status":             session.Status,
		"calculation_metadata": map[string]interface{}{
			"progress_id":              progressID,
			"calculation_duration_ms":  time.Since(progressStart).Milliseconds(),
			"total_possible_questions": totalPossibleQuestions,
			"stages_processed":         stagesProcessed,
			"progress_formula":         "(questions_answered / total_possible) * 100",
			"session_start_time":       session.StartTime,
		},
	}

	progressDuration := time.Since(progressStart)
	log.Printf("[DETAILED_PROGRESS] [%s] ✅ Detailed progress calculation completed (took %v)",
		progressID, progressDuration)
	log.Printf("[DETAILED_PROGRESS] [%s] Final results - Progress: %.2f%%, CurrentStage: %s, Score: %.2f",
		progressID, progressPercentage, session.CurrentStage, session.FinalScore)

	return progressReport
}

func (h *SessionHandler) generateSessionStatistics(session *models.QuizSession) map[string]interface{} {
	stats := map[string]interface{}{
		"session_id":       session.ID,
		"config_id":        session.ConfigID,
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

// GetAnswerCacheStats provides cache monitoring information
func (h *SessionHandler) GetAnswerCacheStats(c *gin.Context) {
	// Check admin access (simplified check)
	adminMode := c.GetHeader("X-Admin-Mode") == "true"
	if !adminMode {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Admin access required for cache statistics",
		})
		return
	}

	stats := h.Service.GetAnswerCacheStats()

	c.JSON(http.StatusOK, gin.H{
		"cache_statistics": stats,
		"timestamp":        time.Now(),
		"description":      "In-memory answer cache statistics for performance monitoring",
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

// GetSessionIntegrityReport provides detailed integrity analysis for a session
func (h *SessionHandler) GetSessionIntegrityReport(c *gin.Context) {
	sessionID := c.Param("id")

	// Validate session exists
	session, err := h.Service.GetSession(context.Background(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Check admin access (in production, implement proper admin check)
	userID := c.GetHeader("X-User-ID")
	adminMode := c.GetHeader("X-Admin-Mode") == "true"

	// Allow session owner or admin to view integrity report
	if session.UserID != userID && !adminMode {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Generate integrity report
	report := h.IntegrityMonitor.GetSessionIntegrityReport(sessionID)

	c.JSON(http.StatusOK, gin.H{
		"integrity_report": report,
		"session_info": gin.H{
			"session_id": sessionID,
			"user_id":    session.UserID,
			"status":     session.Status,
			"start_time": session.StartTime,
		},
	})
}

// GetUserSessions retrieves all sessions for a user with pagination
func (h *SessionHandler) GetUserSessions(c *gin.Context) {
	userID := c.Param("userID")

	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	status := c.Query("status")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 50
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		offset = 0
	}

	// Get sessions from service
	overview, err := h.Service.GetUserSessions(context.Background(), userID, limit, offset, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve user sessions",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, overview)
}

// GetSessionDetails retrieves detailed session information with cached questions
func (h *SessionHandler) GetSessionDetails(c *gin.Context) {
	sessionID := c.Param("id")

	details, err := h.Service.GetSessionDetails(context.Background(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Session not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, details)
}

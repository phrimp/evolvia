package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"quiz-service/internal/adaptive"
	"quiz-service/internal/event"
	"quiz-service/internal/models"
	"quiz-service/internal/repository"
	"quiz-service/internal/selection"
	"quiz-service/internal/timeout"
	"slices"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SessionService handles quiz session operations
type SessionService struct {
	Repo                      *repository.SessionRepository
	ConfigService             *ConfigService // NEW: For global configurations
	QuestionRepo              *repository.QuestionRepository
	ResultService             *ResultService // FIXED: Use service layer instead of direct repo access
	EventPublisher            *event.EventPublisher
	adaptiveManager           *adaptive.Manager
	poolManager               *selection.PoolManager
	sessionSkillCache         map[string]*selection.SkillInfo
	sessionEnhancedSkillCache map[string]*selection.EnhancedSkillInfo
	answerCache               *models.SessionAnswerCache        // NEW: Cache for individual answers
	questionStateCache        *models.SessionQuestionStateCache // NEW: Current question state tracking
	timeoutManager            *timeout.SessionTimeoutManager    // NEW: Session timeout management
}

// NewSessionService creates a new session service
func NewSessionService(
	repo *repository.SessionRepository,
	questionRepo *repository.QuestionRepository,
	configService *ConfigService,
	resultService *ResultService, // FIXED: Add ResultService dependency
) *SessionService {
	service := &SessionService{
		Repo:                      repo,
		ConfigService:             configService,
		QuestionRepo:              questionRepo,
		ResultService:             resultService, // FIXED: Initialize ResultService
		adaptiveManager:           adaptive.NewManager(nil),
		poolManager:               selection.NewPoolManager(questionRepo),
		sessionSkillCache:         make(map[string]*selection.SkillInfo),
		sessionEnhancedSkillCache: make(map[string]*selection.EnhancedSkillInfo),
	}

	// Initialize answer cache using config-based retention
	service.answerCache = service.createAnswerCacheFromConfig()

	// Initialize question state cache for current question validation
	service.questionStateCache = models.NewSessionQuestionStateCache(2 * time.Hour)

	// Initialize timeout manager with session timeout callback
	service.timeoutManager = timeout.NewSessionTimeoutManager(
		service.handleSessionTimeout,
		service.getTimeoutConfigFromGlobalConfig(),
	)

	return service
}

// SetEventPublisher sets the event publisher
func (s *SessionService) SetEventPublisher(publisher *event.EventPublisher) {
	s.EventPublisher = publisher
}

// GetSession retrieves a session by ID
func (s *SessionService) GetSession(ctx context.Context, id string) (*models.QuizSession, error) {
	return s.Repo.FindByID(ctx, id)
}

// CreateGlobalSession creates session with global configuration (no quiz dependency)
func (s *SessionService) CreateGlobalSession(
	ctx context.Context,
	userID string,
	skillInfo *selection.EnhancedSkillInfo,
	preferredBloomLevels []string,
	masteryScore int,
	configID string, // Optional: if empty, uses default config
) (*models.QuizSession, error) {
	// Session creation tracking
	sessionCreationStart := time.Now()
	sessionCreationID := fmt.Sprintf("session_creation_%d", time.Now().UnixNano())

	log.Printf("[SESSION_CREATION] [%s] Starting global session creation for user: %s, skill: %s (%s)", 
		sessionCreationID, userID, skillInfo.ID, skillInfo.Name)
	log.Printf("[SESSION_CREATION] [%s] Request parameters - ConfigID: '%s', MasteryScore: %d, BloomLevels: %v", 
		sessionCreationID, configID, masteryScore, preferredBloomLevels)
	log.Printf("[SESSION_CREATION] [%s] Skill tags - Primary: %v, Secondary: %v, Related: %v", 
		sessionCreationID, skillInfo.PrimaryTags, skillInfo.SecondaryTags, skillInfo.RelatedTags)
	log.Printf("[SESSION_CREATION] [%s] Tag weights - Primary: %.1f, Secondary: %.1f, Related: %.1f, ExactBonus: %.1f", 
		sessionCreationID, skillInfo.TagWeights.PrimaryWeight, skillInfo.TagWeights.SecondaryWeight, 
		skillInfo.TagWeights.RelatedWeight, skillInfo.TagWeights.ExactMatchBonus)

	// Step 1: Get global configuration
	log.Printf("[SESSION_CREATION] [%s] Step 1: Retrieving global configuration", sessionCreationID)
	configStart := time.Now()
	config, err := s.ConfigService.GetConfigForSession(ctx, configID)
	configDuration := time.Since(configStart)
	if err != nil {
		log.Printf("[SESSION_CREATION] [%s] ERROR: Failed to get global configuration (took %v): %v", 
			sessionCreationID, configDuration, err)
		return nil, fmt.Errorf("failed to get global configuration: %w", err)
	}
	log.Printf("[SESSION_CREATION] [%s] Successfully retrieved config ID: '%s' (took %v)", 
		sessionCreationID, config.ID, configDuration)
	log.Printf("[SESSION_CREATION] [%s] Config details - TotalDuration: %ds, StageConfigs: %d", 
		sessionCreationID, config.TotalDurationSeconds, len(config.StageConfig))

	// Step 2: Check for past results (existing logic)
	log.Printf("[SESSION_CREATION] [%s] Step 2: Checking for past results to determine starting levels", sessionCreationID)
	pastResultsStart := time.Now()
	var startingBloomLevel string
	var startingDifficulty string
	var foundPastResults bool

	if s.ResultService != nil {
		pastResults, err := s.ResultService.GetResultsByUser(ctx, userID)
		pastResultsDuration := time.Since(pastResultsStart)
		if err == nil && len(pastResults) > 0 {
			log.Printf("[SESSION_CREATION] [%s] Found %d past results for user (took %v)", 
				sessionCreationID, len(pastResults), pastResultsDuration)
			for i, result := range pastResults {
				if session, err := s.Repo.FindByID(ctx, result.SessionID); err == nil {
					if metadata := session.Metadata; metadata != nil {
						if sid, ok := metadata["skill_id"].(string); ok && sid == skillInfo.ID {
							startingBloomLevel = s.deriveBloomFromResult(&result)
							startingDifficulty = s.deriveDifficultyFromResult(&result)
							foundPastResults = true
							log.Printf("[SESSION_CREATION] [%s] Found matching past result #%d: Bloom='%s', Difficulty='%s', Score=%.1f", 
								sessionCreationID, i+1, startingBloomLevel, startingDifficulty, result.FinalScore)
							break
						}
					}
				}
			}
			if !foundPastResults {
				log.Printf("[SESSION_CREATION] [%s] No matching past results found for skill '%s'", 
					sessionCreationID, skillInfo.ID)
			}
		} else if err != nil {
			log.Printf("[SESSION_CREATION] [%s] Warning: Error retrieving past results (took %v): %v", 
				sessionCreationID, pastResultsDuration, err)
		} else {
			log.Printf("[SESSION_CREATION] [%s] No past results found for user (took %v)", 
				sessionCreationID, pastResultsDuration)
		}
	} else {
		log.Printf("[SESSION_CREATION] [%s] ResultService not available, skipping past results check", sessionCreationID)
	}

	// Step 3: Set defaults if no past results
	log.Printf("[SESSION_CREATION] [%s] Step 3: Setting starting levels and defaults", sessionCreationID)
	var bloomLevels []string
	if startingBloomLevel == "" {
		if len(preferredBloomLevels) > 0 {
			bloomLevels = preferredBloomLevels
			startingBloomLevel = preferredBloomLevels[0]
			log.Printf("[SESSION_CREATION] [%s] Using preferred Bloom levels: %v (starting: '%s')", 
				sessionCreationID, bloomLevels, startingBloomLevel)
		} else {
			bloomLevels = []string{"remember"}
			startingBloomLevel = "remember"
			log.Printf("[SESSION_CREATION] [%s] Using default Bloom level: 'remember'", sessionCreationID)
		}
	} else {
		bloomLevels = []string{startingBloomLevel}
		log.Printf("[SESSION_CREATION] [%s] Using Bloom level from past results: '%s'", 
			sessionCreationID, startingBloomLevel)
	}

	if masteryScore > 0 {
		startingDifficulty = s.mapMasteryScoreToStage(masteryScore)
		log.Printf("[SESSION_CREATION] [%s] Mapped mastery score %d to difficulty: '%s'", 
			sessionCreationID, masteryScore, startingDifficulty)
	} else if startingDifficulty == "" {
		startingDifficulty = "easy"
		log.Printf("[SESSION_CREATION] [%s] Using default difficulty: 'easy'", sessionCreationID)
	} else {
		log.Printf("[SESSION_CREATION] [%s] Using difficulty from past results: '%s'", 
			sessionCreationID, startingDifficulty)
	}

	// Step 4: Validate global question pool with enhanced skill info
	log.Printf("[SESSION_CREATION] [%s] Step 4: Validating global question pool", sessionCreationID)
	poolValidationStart := time.Now()
	standardSkillInfo := &selection.SkillInfo{
		ID:   skillInfo.ID,
		Name: skillInfo.Name,
		Tags: s.mergeTags(skillInfo),
	}
	log.Printf("[SESSION_CREATION] [%s] Merged tags for validation: %v (total: %d)", 
		sessionCreationID, standardSkillInfo.Tags, len(standardSkillInfo.Tags))

	isValid, validation, err := s.poolManager.ValidateGlobalPoolWithBloom(ctx, standardSkillInfo, "temp_session")
	poolValidationDuration := time.Since(poolValidationStart)
	if err != nil {
		log.Printf("[SESSION_CREATION] [%s] ERROR: Pool validation failed (took %v): %v", 
			sessionCreationID, poolValidationDuration, err)
		return nil, fmt.Errorf("failed to validate global question pool: %w", err)
	}
	if !isValid {
		log.Printf("[SESSION_CREATION] [%s] ERROR: Insufficient questions in pool (took %v): %v", 
			sessionCreationID, poolValidationDuration, validation.Warnings)
		return nil, fmt.Errorf("insufficient questions in global pool: %v", validation.Warnings)
	}
	log.Printf("[SESSION_CREATION] [%s] Pool validation successful (took %v): %d available questions", 
		sessionCreationID, poolValidationDuration, validation.TotalQuestions)

	initialStage := s.mapBloomToStage(startingBloomLevel)
	log.Printf("[SESSION_CREATION] [%s] Step 5: Creating session object - Initial stage: '%s'", 
		sessionCreationID, initialStage)

	// Generate session token and prepare session object
	sessionToken := s.generateSessionToken()
	sessionStartTime := time.Now()

	// Create session with global configuration
	session := &models.QuizSession{
		ConfigID:     config.ID, // Use ConfigID instead of QuizID
		UserID:       userID,
		SessionToken: sessionToken,
		StartTime:    sessionStartTime,
		Status:       "active",
		CurrentStage: initialStage,
		StageProgress: map[string]models.StageProgress{
			"easy":   {Attempted: 0, Correct: 0, Passed: false, Score: 0},
			"medium": {Attempted: 0, Correct: 0, Passed: false, Score: 0},
			"hard":   {Attempted: 0, Correct: 0, Passed: false, Score: 0},
		},
		TotalQuestionsAsked: 0,
		QuestionsUsed:       []string{},
		FinalScore:          0,
		Metadata: map[string]any{
			"skill_id":               skillInfo.ID,
			"skill_name":             skillInfo.Name,
			"primary_tags":           skillInfo.PrimaryTags,
			"secondary_tags":         skillInfo.SecondaryTags,
			"related_tags":           skillInfo.RelatedTags,
			"tag_weights":            skillInfo.TagWeights,
			"starting_bloom_level":   startingBloomLevel,
			"preferred_bloom_levels": bloomLevels,
			"starting_difficulty":    startingDifficulty,
			"global_config":          config.StageConfig, // Store config instead of quiz config
			"quiz_start_time":        sessionStartTime.Unix(),
			"session_creation_id":    sessionCreationID,
			"past_results_found":     foundPastResults,
			"creation_context": map[string]any{
				"requested_config_id":     configID,
				"requested_mastery_score": masteryScore,
				"requested_bloom_levels":  preferredBloomLevels,
				"total_tags_count":        len(standardSkillInfo.Tags),
			},
		},
	}

	log.Printf("[SESSION_CREATION] [%s] Session object created - Token: %s", 
		sessionCreationID, sessionToken)
	log.Printf("[SESSION_CREATION] [%s] Metadata - SkillID: '%s', InitialStage: '%s', BloomLevel: '%s'", 
		sessionCreationID, skillInfo.ID, initialStage, startingBloomLevel)

	// Step 6: Persist session to database
	log.Printf("[SESSION_CREATION] [%s] Step 6: Persisting session to database", sessionCreationID)
	dbCreateStart := time.Now()
	err = s.Repo.Create(ctx, session)
	dbCreateDuration := time.Since(dbCreateStart)
	if err != nil {
		log.Printf("[SESSION_CREATION] [%s] ERROR: Failed to create session in database (took %v): %v", 
			sessionCreationID, dbCreateDuration, err)
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	log.Printf("[SESSION_CREATION] [%s] Session successfully created in database (took %v) - ID: %s", 
		sessionCreationID, dbCreateDuration, session.ID)

	// Step 7: Cache skill info and generate question pools
	log.Printf("[SESSION_CREATION] [%s] Step 7: Caching skill info and generating question pools", sessionCreationID)
	cacheStart := time.Now()

	// Cache skill info
	s.sessionSkillCache[session.ID] = standardSkillInfo
	s.sessionEnhancedSkillCache[session.ID] = skillInfo
	cacheDuration := time.Since(cacheStart)
	log.Printf("[SESSION_CREATION] [%s] Skill info cached (took %v) - Standard: %d tags, Enhanced: P:%d S:%d R:%d", 
		sessionCreationID, cacheDuration, len(standardSkillInfo.Tags), 
		len(skillInfo.PrimaryTags), len(skillInfo.SecondaryTags), len(skillInfo.RelatedTags))

	// Generate and cache pools for the session using optimized batch generation
	poolGenStart := time.Now()
	if err := s.generateSessionPoolOptimized(ctx, session.ID, session); err != nil {
		poolGenDuration := time.Since(poolGenStart)
		log.Printf("[SESSION_CREATION] [%s] Warning: Failed to pre-generate pools (took %v): %v", 
			sessionCreationID, poolGenDuration, err)
		// Don't fail session creation if pool generation fails - pools can be generated lazily
	} else {
		poolGenDuration := time.Since(poolGenStart)
		log.Printf("[SESSION_CREATION] [%s] Question pools successfully pre-generated (took %v)", 
			sessionCreationID, poolGenDuration)
	}

	// Step 8: Initialize session timeout management
	log.Printf("[SESSION_CREATION] [%s] Step 8: Starting first question timeout", sessionCreationID)
	timeoutStart := time.Now()

	// Start first question timeout countdown - user must request first question within configured time
	// NOTE: Main session timeout will start ONLY when first question is requested
	s.timeoutManager.StartFirstQuestionTimeout(session.ID)
	timeoutDuration := time.Since(timeoutStart)
	log.Printf("[SESSION_CREATION] [%s] First question timeout started (took %v)", 
		sessionCreationID, timeoutDuration)

	// Step 9: Publish session creation event
	log.Printf("[SESSION_CREATION] [%s] Step 9: Publishing session creation event", sessionCreationID)
	eventStart := time.Now()

	if s.EventPublisher != nil {
		eventData := map[string]any{
			"session_id":           session.ID,
			"config_id":            config.ID,
			"user_id":              userID,
			"skill_id":             skillInfo.ID,
			"skill_name":           skillInfo.Name,
			"tag_distribution":     s.getTagDistribution(skillInfo),
			"starting_bloom_level": startingBloomLevel,
			"starting_difficulty":  startingDifficulty,
			"global_config":        true, // Flag to indicate this is a global config session
			"creation_context": map[string]any{
				"session_creation_id":    sessionCreationID,
				"total_creation_time_ms": time.Since(sessionCreationStart).Milliseconds(),
				"past_results_found":     foundPastResults,
				"pool_validation_passed": isValid,
				"requested_config_id":    configID,
				"mastery_score":          masteryScore,
				"preferred_bloom_levels": preferredBloomLevels,
			},
			"performance_metrics": map[string]any{
				"config_retrieval_ms":    configDuration.Milliseconds(),
				"past_results_check_ms":  time.Since(pastResultsStart).Milliseconds(),
				"pool_validation_ms":     poolValidationDuration.Milliseconds(),
				"db_creation_ms":         dbCreateDuration.Milliseconds(),
				"total_creation_ms":      time.Since(sessionCreationStart).Milliseconds(),
			},
		}
		s.EventPublisher.Publish("quiz.session.created", eventData)
		eventDuration := time.Since(eventStart)
		log.Printf("[SESSION_CREATION] [%s] Event published successfully (took %v)", 
			sessionCreationID, eventDuration)
	} else {
		log.Printf("[SESSION_CREATION] [%s] Warning: EventPublisher not available, skipping event", sessionCreationID)
	}

	// Session creation completed successfully
	totalCreationTime := time.Since(sessionCreationStart)
	log.Printf("[SESSION_CREATION] [%s] ✅ SUCCESS: Global session created successfully", sessionCreationID)
	log.Printf("[SESSION_CREATION] [%s] Final details - SessionID: %s, UserID: %s, SkillID: %s", 
		sessionCreationID, session.ID, userID, skillInfo.ID)
	log.Printf("[SESSION_CREATION] [%s] Performance summary - Total: %v, DB: %v, Pools: %v, Config: %v", 
		sessionCreationID, totalCreationTime, dbCreateDuration, 
		time.Since(poolGenStart), configDuration)
	log.Printf("[SESSION_CREATION] [%s] Session ready for first question request", sessionCreationID)

	return session, nil
}

// Helper: Map Bloom taxonomy level to stage
func (s *SessionService) mapBloomToStage(bloomLevel string) string {
	bloomToStageMap := map[string]string{
		"remember":   "easy",
		"understand": "easy",
		"apply":      "medium",
		"analyze":    "medium",
		"evaluate":   "hard",
		"create":     "hard",
	}

	if stage, ok := bloomToStageMap[strings.ToLower(bloomLevel)]; ok {
		return stage
	}
	return "easy" // Default
}

// Helper: Derive Bloom level from past result
func (s *SessionService) deriveBloomFromResult(result *models.QuizResult) string {
	// Check highest stage completed successfully
	if breakdown, ok := result.StageBreakdown["hard"]; ok && breakdown.Passed {
		return "evaluate" // High performance
	}
	if breakdown, ok := result.StageBreakdown["medium"]; ok && breakdown.Passed {
		return "apply" // Medium performance
	}
	return "understand" // Default to lower level
}

// Helper: Derive difficulty from past result
func (s *SessionService) deriveDifficultyFromResult(result *models.QuizResult) string {
	// Use configuration-based thresholds
	config, err := s.ConfigService.GetDefaultConfig(context.Background())
	if err != nil {
		// Fallback to hardcoded values
		return s.getHardcodedDifficultyFromResult(result)
	}

	if result.Percentage >= config.ScoringConfig.DifficultyThresholds.MediumToHard {
		return "hard"
	} else if result.Percentage >= config.ScoringConfig.DifficultyThresholds.EasyToMedium {
		return "medium"
	}
	return "easy"
}

// Fallback hardcoded difficulty mapping
func (s *SessionService) getHardcodedDifficultyFromResult(result *models.QuizResult) string {
	if result.Percentage >= 80 {
		return "hard"
	} else if result.Percentage >= 60 {
		return "medium"
	}
	return "easy"
}

// mapMasteryScoreToStage maps mastery score to difficulty stage using config
func (s *SessionService) mapMasteryScoreToStage(masteryScore int) string {
	config, err := s.ConfigService.GetDefaultConfig(context.Background())
	if err != nil {
		// Fallback to hardcoded values
		return s.getHardcodedMasteryStage(masteryScore)
	}

	if masteryScore <= config.ScoringConfig.DifficultyThresholds.MasteryEasy {
		return "easy"
	} else if masteryScore <= config.ScoringConfig.DifficultyThresholds.MasteryMedium {
		return "medium"
	} else {
		return "hard"
	}
}

// Fallback hardcoded mastery mapping
func (s *SessionService) getHardcodedMasteryStage(masteryScore int) string {
	if masteryScore <= 3 {
		return "easy"
	} else if masteryScore <= 7 {
		return "medium"
	} else {
		return "hard"
	}
}

// UpdateSession updates session fields
func (s *SessionService) UpdateSession(ctx context.Context, id string, update map[string]any) error {
	return s.Repo.Update(ctx, id, update)
}

// ProcessAnswer handles answer submission with adaptive logic
func (s *SessionService) ProcessAnswer(
	ctx context.Context,
	sessionID string,
	questionID string,
	question *models.Question,
	userAnswer string,
	isCorrect bool,
) (*adaptive.AnswerResult, error) {
	// Validate that answer corresponds to current question
	if err := s.questionStateCache.ValidateAnswerForCurrentQuestion(sessionID, questionID); err != nil {
		return nil, fmt.Errorf("current question validation failed: %w", err)
	}

	// Mark question as answered to prevent duplicate submissions
	if err := s.questionStateCache.MarkQuestionAnswered(sessionID); err != nil {
		return nil, fmt.Errorf("failed to mark question as answered: %w", err)
	}

	// Get session
	session, err := s.Repo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Reconstruct adaptive session
	adaptiveSession := s.reconstructAdaptiveSession(session)

	// Process answer through adaptive manager with question object
	result, err := s.adaptiveManager.ProcessAnswer(adaptiveSession, question, isCorrect)
	if err != nil {
		return nil, err
	}

	// Update session with new state
	s.updateSessionFromAdaptive(session, adaptiveSession, result)

	// Add question to used list
	if !s.isQuestionUsed(questionID, session.QuestionsUsed) {
		session.QuestionsUsed = append(session.QuestionsUsed, questionID)
	}

	// Save updated session
	update := bson.M{
		"current_stage":         session.CurrentStage,
		"stage_progress":        session.StageProgress,
		"total_questions_asked": session.TotalQuestionsAsked,
		"questions_used":        session.QuestionsUsed,
		"final_score":           session.FinalScore,
		"status":                session.Status,
	}

	err = s.Repo.Update(ctx, sessionID, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	// Publish answer event
	if s.EventPublisher != nil {
		s.EventPublisher.Publish("quiz.question.answered", map[string]any{
			"session_id":    sessionID,
			"question_id":   questionID,
			"is_correct":    isCorrect,
			"points_earned": result.PointsEarned,
			"stage":         session.CurrentStage,
			"stage_update":  result.StageUpdate,
		})
	}

	// Clear current question state after successful processing
	// This allows the session to move to the next question
	s.questionStateCache.ClearCurrentQuestion(sessionID)

	return result, nil
}

// GetNextQuestion gets the next question based on adaptive criteria
func (s *SessionService) GetNextQuestion(ctx context.Context, sessionID string) (*models.Question, error) {
	// Get session
	session, err := s.Repo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Check if session is complete
	if session.Status == "completed" {
		return nil, fmt.Errorf("session is already completed")
	}

	// Get skill info from session
	skillInfo := s.getSkillInfoFromSession(session)
	if skillInfo == nil {
		return nil, fmt.Errorf("skill information not found for session")
	}

	// Cancel first question timeout and start main session countdown when first question is requested
	s.timeoutManager.CancelTimeout(sessionID)

	// Start main session timeout now that user has requested first question
	s.startMainSessionTimeout(sessionID)

	// Use cache-based pool selection
	question, err := s.getQuestionFromCachedPool(ctx, sessionID, session)
	if err == nil {
		// Set current question state for validation - prevent question hoarding
		if err := s.questionStateCache.SetCurrentQuestion(sessionID, question.ID, question.Type, session.CurrentStage); err != nil {
			return nil, fmt.Errorf("cannot set current question: %w", err)
		}
		return question, nil
	}
	// Log cache miss and continue with dynamic selection
	fmt.Printf("Cache-based pool selection failed, using dynamic selection: %v\n", err)

	// Dynamic selection with Bloom's criteria
	adaptiveSession := s.reconstructAdaptiveSession(session)
	criteria, err := s.adaptiveManager.GetNextQuestionCriteria(adaptiveSession)
	if err != nil {
		return nil, err
	}

	// Select with Bloom's distribution
	var questions []models.Question
	// Check if this is a true global session (created with CreateGlobalSession) or legacy
	if metadata := session.Metadata; metadata != nil {
		if _, hasGlobalConfig := metadata["global_config"]; hasGlobalConfig {
			// True global session - use global question selection
			questions, err = s.selectGlobalQuestionsWithBloomCriteria(ctx, skillInfo, criteria)
		} else {
			// Legacy session - use quiz-based method (ConfigID contains quizID)
			questions, err = s.selectQuestionsWithBloomCriteria(ctx, session.ConfigID, skillInfo, criteria)
		}
	} else {
		// Very old session without metadata - use quiz-based method
		questions, err = s.selectQuestionsWithBloomCriteria(ctx, session.ConfigID, skillInfo, criteria)
	}
	if err != nil {
		return nil, err
	}

	if len(questions) == 0 {
		return nil, fmt.Errorf("no available questions for current stage")
	}

	// Validate question pool before selection
	if err := s.validateQuestionPool(questions, 1); err != nil {
		return nil, fmt.Errorf("question pool validation failed: %w", err)
	}

	// Select first valid question
	selectedQuestion := &questions[0]

	// Perform comprehensive question validation
	if err := s.validateQuestionEligibility(selectedQuestion, session, string(criteria.Stage)); err != nil {
		return nil, fmt.Errorf("question validation failed: %w", err)
	}

	// Update session with used question to prevent future repetition
	err = s.addQuestionToUsed(ctx, sessionID, selectedQuestion.ID)
	if err != nil {
		// Log error but don't fail - question selection is more important
		fmt.Printf("Warning: Failed to update used questions for session %s: %v\n", sessionID, err)
	}

	// Set current question state for validation - prevent question hoarding
	if err := s.questionStateCache.SetCurrentQuestion(sessionID, selectedQuestion.ID, selectedQuestion.Type, session.CurrentStage); err != nil {
		return nil, fmt.Errorf("cannot set current question: %w", err)
	}

	return selectedQuestion, nil
}

// SubmitSession completes and submits the session
func (s *SessionService) SubmitSession(
	ctx context.Context,
	sessionID string,
	completionType string,
	finalScore float64,
) (*models.QuizResult, error) {
	exist, err := s.ResultService.GetResultBySession(ctx, sessionID)
	if exist != nil {
		log.Printf("SESSION SUBMIT ERROR: this session is completed but check this detail too: %s", err)
		return nil, fmt.Errorf("this session is completed")
	}
	// Get session
	session, err := s.Repo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Calculate final score if not provided
	if finalScore == 0 {
		adaptiveSession := s.reconstructAdaptiveSession(session)
		if adaptiveSession == nil {
			return nil, fmt.Errorf("no session found")
		}
		finalScore = s.adaptiveManager.CalculateFinalScore(adaptiveSession)
	}

	// Update session
	update := bson.M{
		"status":           "completed",
		"completion_type":  completionType,
		"final_score":      finalScore,
		"end_time":         time.Now(),
		"duration_seconds": int(time.Since(session.StartTime).Seconds()),
	}

	err = s.Repo.Update(ctx, sessionID, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	// Create result
	result := s.createQuizResult(session, completionType, finalScore)

	// Store result if repository is available
	if s.ResultService != nil {
		err = s.ResultService.CreateResult(ctx, result)
		if err != nil {
			fmt.Printf("Failed to store result: %v\n", err)
		}
	}

	// Mark session cache as completed for proper retention timing
	s.MarkSessionCacheCompleted(sessionID)

	// Clear current question state as session is completed
	s.questionStateCache.ClearCurrentQuestion(sessionID)

	// Publish enhanced events
	if s.EventPublisher != nil {
		// Enhanced skills event with rich learning analytics
		skillsData := s.extractKnowledgeData(session, result)
		s.EventPublisher.Publish("quiz_completed", skillsData)
	}

	return result, nil
}

// PauseSession pauses an active session
func (s *SessionService) PauseSession(ctx context.Context, sessionID string, reason string) error {
	update := bson.M{
		"status":       "paused",
		"pause_reason": reason,
		"pause_time":   time.Now(),
	}

	err := s.Repo.Update(ctx, sessionID, update)
	if err != nil {
		return fmt.Errorf("failed to pause session: %w", err)
	}

	// Publish pause event
	if s.EventPublisher != nil {
		s.EventPublisher.Publish("quiz.session.paused", map[string]interface{}{
			"session_id": sessionID,
			"reason":     reason,
		})
	}

	return nil
}

// ResumeSession resumes a paused session
func (s *SessionService) ResumeSession(ctx context.Context, sessionID string) error {
	update := bson.M{
		"status":      "active",
		"resume_time": time.Now(),
	}

	err := s.Repo.Update(ctx, sessionID, update)
	if err != nil {
		return fmt.Errorf("failed to resume session: %w", err)
	}

	// Publish resume event
	if s.EventPublisher != nil {
		s.EventPublisher.Publish("quiz.session.resumed", map[string]interface{}{
			"session_id": sessionID,
		})
	}

	return nil
}

// GetSessionStatus returns current session status
func (s *SessionService) GetSessionStatus(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	session, err := s.Repo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	adaptiveSession := s.reconstructAdaptiveSession(session)
	summary := s.adaptiveManager.GetSessionSummary(adaptiveSession)

	// Add additional info
	summary["time_elapsed"] = int(time.Since(session.StartTime).Seconds())
	summary["time_remaining"] = s.calculateTimeRemaining(session)
	summary["skill_info"] = s.getSkillInfoFromSession(session)

	return summary, nil
}

// GetQuizPoolInfo provides information about question distribution
func (s *SessionService) GetQuizPoolInfo(ctx context.Context, quizID string, skillID string) (map[string]interface{}, error) {
	skillInfo := s.getSkillInfo(skillID)

	distribution, err := s.poolManager.GetQuestionDistributionWithBloom(ctx, quizID, skillInfo)
	if err != nil {
		return nil, err
	}

	// Validate pool
	isValid, validation, _ := s.poolManager.ValidateQuizPoolWithBloom(ctx, quizID, skillInfo)

	distribution["is_valid_for_adaptive"] = isValid
	distribution["validation"] = validation

	return distribution, nil
}

// SelectQuestionsForStage batch selects questions for a stage
func (s *SessionService) SelectQuestionsForStage(
	ctx context.Context,
	quizID string,
	skillID string,
	stage string,
	count int,
	excludeIDs []string,
) ([]models.Question, error) {
	skillInfo := s.getSkillInfo(skillID)

	// Get Bloom's distribution for the stage
	bloomDist := s.getBloomDistribution(stage)

	result, err := s.poolManager.SelectAdaptiveQuestionsWithBloom(
		ctx,
		quizID,
		skillInfo,
		stage,
		count,
		excludeIDs,
		bloomDist,
	)
	if err != nil {
		return nil, err
	}

	return result.Questions, nil
}

// Helper methods

// reconstructAdaptiveSession rebuilds adaptive session state with comprehensive logging
func (s *SessionService) reconstructAdaptiveSession(session *models.QuizSession) *adaptive.AdaptiveSession {
	reconstructStart := time.Now()
	reconstructID := fmt.Sprintf("reconstruct_%d", time.Now().UnixNano())
	log.Printf("[ADAPTIVE_RECONSTRUCT] [%s] Starting adaptive session reconstruction for %s", 
		reconstructID, session.ID)

	if session.ID == "" {
		log.Printf("[ADAPTIVE_RECONSTRUCT] [%s] ERROR: Session ID is empty", reconstructID)
		return nil
	}

	// Create new adaptive session
	adaptiveSession := adaptive.NewAdaptiveSession(session.ID)
	log.Printf("[ADAPTIVE_RECONSTRUCT] [%s] Created new adaptive session object", reconstructID)

	// Map current stage
	log.Printf("[ADAPTIVE_RECONSTRUCT] [%s] Mapping current stage: %s", reconstructID, session.CurrentStage)
	switch session.CurrentStage {
	case "easy":
		adaptiveSession.CurrentStage = adaptive.StageEasy
		log.Printf("[ADAPTIVE_RECONSTRUCT] [%s] Mapped to StageEasy", reconstructID)
	case "medium":
		adaptiveSession.CurrentStage = adaptive.StageMedium
		log.Printf("[ADAPTIVE_RECONSTRUCT] [%s] Mapped to StageMedium", reconstructID)
	case "hard":
		adaptiveSession.CurrentStage = adaptive.StageHard
		log.Printf("[ADAPTIVE_RECONSTRUCT] [%s] Mapped to StageHard", reconstructID)
	default:
		log.Printf("[ADAPTIVE_RECONSTRUCT] [%s] Warning: Unknown stage '%s', defaulting to StageEasy", 
			reconstructID, session.CurrentStage)
		adaptiveSession.CurrentStage = adaptive.StageEasy
	}

	// Map stage progress with detailed logging
	log.Printf("[ADAPTIVE_RECONSTRUCT] [%s] Mapping %d stage progress entries", 
		reconstructID, len(session.StageProgress))
	stagesMapped := 0
	for stage, progress := range session.StageProgress {
		log.Printf("[ADAPTIVE_RECONSTRUCT] [%s] Processing stage '%s' - Attempted: %d, Correct: %d, Passed: %v, Score: %.2f", 
			reconstructID, stage, progress.Attempted, progress.Correct, progress.Passed, progress.Score)

		var adaptiveStage adaptive.Stage
		switch stage {
		case "easy":
			adaptiveStage = adaptive.StageEasy
		case "medium":
			adaptiveStage = adaptive.StageMedium
		case "hard":
			adaptiveStage = adaptive.StageHard
		default:
			log.Printf("[ADAPTIVE_RECONSTRUCT] [%s] Warning: Skipping unknown stage '%s'", 
				reconstructID, stage)
			continue
		}

		adaptiveSession.StageStatuses[adaptiveStage] = &adaptive.StageStatus{
			Stage:          adaptiveStage,
			QuestionsAsked: progress.Attempted,
			CorrectAnswers: progress.Correct,
			InRecovery:     progress.RecoveryRound > 0,
			RecoveryRound:  progress.RecoveryRound,
			Passed:         progress.Passed,
			Score:          progress.Score,
		}
		stagesMapped++
		log.Printf("[ADAPTIVE_RECONSTRUCT] [%s] Mapped stage '%s' - InRecovery: %v, RecoveryRound: %d", 
			reconstructID, stage, progress.RecoveryRound > 0, progress.RecoveryRound)
	}
	log.Printf("[ADAPTIVE_RECONSTRUCT] [%s] Successfully mapped %d stages", reconstructID, stagesMapped)

	// Set aggregate session data
	adaptiveSession.TotalQuestionsAsked = session.TotalQuestionsAsked
	log.Printf("[ADAPTIVE_RECONSTRUCT] [%s] Set TotalQuestionsAsked: %d", 
		reconstructID, session.TotalQuestionsAsked)

	// Ensure consistent question tracking between session and adaptive manager
	adaptiveSession.UsedQuestionIDs = make([]string, len(session.QuestionsUsed))
	copy(adaptiveSession.UsedQuestionIDs, session.QuestionsUsed)
	log.Printf("[ADAPTIVE_RECONSTRUCT] [%s] Copied %d used question IDs", 
		reconstructID, len(session.QuestionsUsed))

	adaptiveSession.TotalScore = session.FinalScore
	adaptiveSession.IsComplete = session.Status == "completed"
	log.Printf("[ADAPTIVE_RECONSTRUCT] [%s] Set final state - TotalScore: %.2f, IsComplete: %v (status: %s)", 
		reconstructID, session.FinalScore, adaptiveSession.IsComplete, session.Status)

	reconstructDuration := time.Since(reconstructStart)
	log.Printf("[ADAPTIVE_RECONSTRUCT] [%s] ✅ Adaptive session reconstruction completed (took %v)", 
		reconstructID, reconstructDuration)
	log.Printf("[ADAPTIVE_RECONSTRUCT] [%s] Final state - CurrentStage: %v, StageStatuses: %d, UsedQuestions: %d", 
		reconstructID, adaptiveSession.CurrentStage, len(adaptiveSession.StageStatuses), len(adaptiveSession.UsedQuestionIDs))

	return adaptiveSession
}

func (s *SessionService) updateSessionFromAdaptive(
	session *models.QuizSession,
	adaptiveSession *adaptive.AdaptiveSession,
	result *adaptive.AnswerResult,
) {
	// Update current stage
	session.CurrentStage = string(adaptiveSession.CurrentStage)

	// Update stage progress
	for stage, status := range adaptiveSession.StageStatuses {
		session.StageProgress[string(stage)] = models.StageProgress{
			Attempted:     status.QuestionsAsked,
			Correct:       status.CorrectAnswers,
			Passed:        status.Passed,
			RecoveryRound: status.RecoveryRound,
			Score:         status.Score,
		}
	}

	session.TotalQuestionsAsked = adaptiveSession.TotalQuestionsAsked
	session.FinalScore = adaptiveSession.TotalScore

	if adaptiveSession.IsComplete {
		session.Status = "completed"
		session.CompletionType = "adaptive_complete"
	}
}

func (s *SessionService) selectQuestionsWithBloomCriteria(
	ctx context.Context,
	quizID string,
	skillInfo *selection.SkillInfo,
	criteria *adaptive.QuestionRequest,
) ([]models.Question, error) {
	session, _ := s.Repo.FindByID(ctx, criteria.SessionID)

	difficulty := s.mapStageToDifficulty(criteria.Stage)

	// Use custom Bloom distribution if user has preferred bloom levels
	var bloomDist map[string]float64
	if session != nil && session.Metadata != nil {
		if preferredLevels, ok := s.extractStringSlice(session.Metadata["preferred_bloom_levels"]); ok && len(preferredLevels) > 0 {
			bloomDist = s.getCustomBloomDistribution(preferredLevels)
		} else if startingBloomLevel, ok := session.Metadata["starting_bloom_level"].(string); ok && startingBloomLevel != "" {
			bloomDist = s.getCustomBloomDistribution([]string{startingBloomLevel})
		} else {
			bloomDist = s.getBloomDistribution(difficulty)
		}
	} else {
		bloomDist = s.getBloomDistribution(difficulty)
	}

	// Check if we have enhanced skill info
	enhancedSkillInfo := s.getEnhancedSkillInfoFromSession(session)

	if enhancedSkillInfo != nil {
		// Use enhanced selection with tag weights
		var result *selection.SelectionResult
		var err error

		if criteria.IsRecovery {
			result, err = s.poolManager.SelectRecoveryQuestionsWithEnhancedWeights(
				ctx, quizID, enhancedSkillInfo, difficulty, 1, criteria.ExcludeIDs, bloomDist,
			)
		} else {
			result, err = s.poolManager.SelectAdaptiveQuestionsWithEnhancedWeights(
				ctx, quizID, enhancedSkillInfo, difficulty, 1, criteria.ExcludeIDs, bloomDist,
			)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to select questions with enhanced weights: %w", err)
		}

		// Log selection quality for monitoring
		if result != nil && len(result.Questions) > 0 {
			s.logSelectionQuality(session.ID, result)
		}

		return result.Questions, nil
	}

	// Fallback to original logic if no enhanced info
	var result *selection.SelectionResult
	var err error

	if criteria.IsRecovery {
		result, err = s.poolManager.SelectRecoveryQuestionsWithBloom(
			ctx, quizID, skillInfo, difficulty, 1, criteria.ExcludeIDs, bloomDist,
		)
	} else {
		result, err = s.poolManager.SelectAdaptiveQuestionsWithBloom(
			ctx, quizID, skillInfo, difficulty, 1, criteria.ExcludeIDs, bloomDist,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to select questions: %w", err)
	}

	return result.Questions, nil
}

func (s *SessionService) selectGlobalQuestionsWithBloomCriteria(
	ctx context.Context,
	skillInfo *selection.SkillInfo,
	criteria *adaptive.QuestionRequest,
) ([]models.Question, error) {
	session, _ := s.Repo.FindByID(ctx, criteria.SessionID)

	difficulty := s.mapStageToDifficulty(criteria.Stage)

	// Use custom Bloom distribution if user has preferred bloom levels
	var bloomDist map[string]float64
	if session != nil && session.Metadata != nil {
		if preferredLevels, ok := s.extractStringSlice(session.Metadata["preferred_bloom_levels"]); ok && len(preferredLevels) > 0 {
			bloomDist = s.getCustomBloomDistribution(preferredLevels)
		} else if startingBloomLevel, ok := session.Metadata["starting_bloom_level"].(string); ok && startingBloomLevel != "" {
			bloomDist = s.getCustomBloomDistribution([]string{startingBloomLevel})
		} else {
			bloomDist = s.getBloomDistribution(difficulty)
		}
	} else {
		bloomDist = s.getBloomDistribution(difficulty)
	}

	// Check if we have enhanced skill info
	enhancedSkillInfo := s.getEnhancedSkillInfoFromSession(session)

	if enhancedSkillInfo != nil {
		// Use enhanced selection with tag weights - global version
		var result *selection.SelectionResult
		var err error

		if criteria.IsRecovery {
			result, err = s.poolManager.SelectRecoveryQuestionsWithEnhancedWeightsGlobal(
				ctx, enhancedSkillInfo, difficulty, 1, criteria.ExcludeIDs, bloomDist,
			)
		} else {
			result, err = s.poolManager.SelectAdaptiveQuestionsWithEnhancedWeightsGlobal(
				ctx, enhancedSkillInfo, difficulty, 1, criteria.ExcludeIDs, bloomDist,
			)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to select global questions with enhanced weights: %w", err)
		}

		// Log selection quality for monitoring
		if result != nil && len(result.Questions) > 0 {
			s.logSelectionQuality(session.ID, result)
		}

		return result.Questions, nil
	}

	// Fallback to standard global logic if no enhanced info
	var result *selection.SelectionResult
	var err error

	if criteria.IsRecovery {
		result, err = s.poolManager.SelectRecoveryQuestionsWithBloomGlobal(
			ctx, skillInfo, difficulty, 1, criteria.ExcludeIDs, bloomDist,
		)
	} else {
		result, err = s.poolManager.SelectAdaptiveQuestionsWithBloomGlobal(
			ctx, skillInfo, difficulty, 1, criteria.ExcludeIDs, bloomDist,
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to select global questions: %w", err)
	}

	return result.Questions, nil
}

func (s *SessionService) getEnhancedSkillInfoFromSession(session *models.QuizSession) *selection.EnhancedSkillInfo {
	if session == nil || session.Metadata == nil {
		return nil
	}

	metadata := session.Metadata

	// Check cache first
	if cached, ok := s.sessionEnhancedSkillCache[session.ID]; ok {
		return cached
	}

	// Reconstruct from metadata
	enhancedInfo := &selection.EnhancedSkillInfo{
		TagWeights: selection.TagWeightConfig{
			PrimaryWeight:   3.0, // Default
			SecondaryWeight: 1.5, // Default
			RelatedWeight:   0.5, // Default
			ExactMatchBonus: 2.0, // Default
		},
	}

	// Extract basic info
	if id, ok := metadata["skill_id"].(string); ok {
		enhancedInfo.ID = id
	}
	if name, ok := metadata["skill_name"].(string); ok {
		enhancedInfo.Name = name
	}

	// Extract categorized tags
	if primaryTags, ok := s.extractStringSlice(metadata["primary_tags"]); ok {
		enhancedInfo.PrimaryTags = primaryTags
	}
	if secondaryTags, ok := s.extractStringSlice(metadata["secondary_tags"]); ok {
		enhancedInfo.SecondaryTags = secondaryTags
	}
	if relatedTags, ok := s.extractStringSlice(metadata["related_tags"]); ok {
		enhancedInfo.RelatedTags = relatedTags
	}

	// Extract tag weights if present
	if weights, ok := metadata["tag_weights"].(map[string]interface{}); ok {
		if pw, ok := weights["primary_weight"].(float64); ok {
			enhancedInfo.TagWeights.PrimaryWeight = pw
		}
		if sw, ok := weights["secondary_weight"].(float64); ok {
			enhancedInfo.TagWeights.SecondaryWeight = sw
		}
		if rw, ok := weights["related_weight"].(float64); ok {
			enhancedInfo.TagWeights.RelatedWeight = rw
		}
		if eb, ok := weights["exact_match_bonus"].(float64); ok {
			enhancedInfo.TagWeights.ExactMatchBonus = eb
		}
	}

	// Cache if we have valid info
	if enhancedInfo.ID != "" {
		s.sessionEnhancedSkillCache[session.ID] = enhancedInfo
		return enhancedInfo
	}

	return nil
}

// Add helper methods
func (s *SessionService) extractStringSlice(data interface{}) ([]string, bool) {
	switch v := data.(type) {
	case []string:
		return v, true
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result, true
	}
	return nil, false
}

func (s *SessionService) mergeTags(skillInfo *selection.EnhancedSkillInfo) []string {
	// Merge all tags for backward compatibility
	allTags := make([]string, 0)
	allTags = append(allTags, skillInfo.PrimaryTags...)
	allTags = append(allTags, skillInfo.SecondaryTags...)
	allTags = append(allTags, skillInfo.RelatedTags...)
	return allTags
}

func (s *SessionService) getTagDistribution(skillInfo *selection.EnhancedSkillInfo) map[string]int {
	return map[string]int{
		"primary":   len(skillInfo.PrimaryTags),
		"secondary": len(skillInfo.SecondaryTags),
		"related":   len(skillInfo.RelatedTags),
	}
}

func (s *SessionService) logSelectionQuality(sessionID string, result *selection.SelectionResult) {
	// Log selection quality metrics for monitoring
	fmt.Printf("[Selection Quality] Session: %s, Candidates: %d, Avg Match: %.2f\n",
		sessionID, result.TotalCandidates, result.AverageMatch)

	if s.EventPublisher != nil {
		s.EventPublisher.Publish("quiz.selection.quality", map[string]interface{}{
			"session_id":       sessionID,
			"total_candidates": result.TotalCandidates,
			"average_match":    result.AverageMatch,
			"bloom_coverage":   result.BloomCoverage,
			"tag_coverage":     result.TagCoverage,
		})
	}
}

// Add this helper method for custom Bloom distribution
func (s *SessionService) getCustomBloomDistribution(targetBlooms []string) map[string]float64 {
	// Create base distribution
	dist := map[string]float64{
		"remember":   0.1,
		"understand": 0.1,
		"apply":      0.1,
		"analyze":    0.1,
		"evaluate":   0.1,
		"create":     0.1,
	}

	// Distribute 30% among target levels
	if len(targetBlooms) > 0 {
		targetWeight := 0.3 / float64(len(targetBlooms))
		for _, target := range targetBlooms {
			if _, ok := dist[strings.ToLower(target)]; ok {
				dist[strings.ToLower(target)] = targetWeight
			}
		}
	}

	// Normalize to sum to 1.0
	total := 0.0
	for _, v := range dist {
		total += v
	}
	if total > 0 {
		for k := range dist {
			dist[k] = dist[k] / total
		}
	}

	return dist
}

func (s *SessionService) getSkillInfoFromSession(session *models.QuizSession) *selection.SkillInfo {
	// Check cache first
	if cached, ok := s.sessionSkillCache[session.ID]; ok {
		return cached
	}

	// Reconstruct from metadata
	if metadata := session.Metadata; metadata != nil {
		skillInfo := &selection.SkillInfo{}

		if id, ok := metadata["skill_id"].(string); ok {
			skillInfo.ID = id
		}
		if name, ok := metadata["skill_name"].(string); ok {
			skillInfo.Name = name
		}
		if tags, ok := metadata["skill_tags"].([]interface{}); ok {
			skillInfo.Tags = make([]string, len(tags))
			for i, tag := range tags {
				if str, ok := tag.(string); ok {
					skillInfo.Tags[i] = str
				}
			}
		} else if tags, ok := metadata["skill_tags"].([]string); ok {
			skillInfo.Tags = tags
		}

		if skillInfo.ID != "" {
			s.sessionSkillCache[session.ID] = skillInfo
			return skillInfo
		}
	}

	// Fallback to default
	return s.getSkillInfo(s.extractSkillID(session))
}

func (s *SessionService) getSkillInfo(skillID string) *selection.SkillInfo {
	// Default skill info - in production, this would call skill service
	return &selection.SkillInfo{
		ID:   skillID,
		Name: "Unknown Skill",
		Tags: []string{},
	}
}

// getQuestionFromCachedPool retrieves a question from the cache-based pool manager
func (s *SessionService) getQuestionFromCachedPool(
	ctx context.Context,
	sessionID string,
	session *models.QuizSession,
) (*models.Question, error) {
	// Generate cache key based on session and current stage
	cacheKey := s.generatePoolCacheKey(sessionID, session)

	// Check if pool exists in cache
	pool := s.getPoolFromCache(cacheKey)
	if pool == nil {
		// Pool not in cache - generate it lazily
		if err := s.generateSessionPool(ctx, sessionID, session); err != nil {
			return nil, fmt.Errorf("failed to generate pool for session %s: %w", sessionID, err)
		}
		pool = s.getPoolFromCache(cacheKey)
		if pool == nil {
			return nil, fmt.Errorf("pool generation failed for key: %s", cacheKey)
		}
	}

	// Get current stage criteria
	adaptiveSession := s.reconstructAdaptiveSession(session)
	criteria, err := s.adaptiveManager.GetNextQuestionCriteria(adaptiveSession)
	if err != nil {
		return nil, fmt.Errorf("failed to get question criteria: %w", err)
	}

	// Try each question in the cached pool
	for _, question := range pool.Questions {
		// Check if question is already used
		if s.isQuestionUsed(question.ID, session.QuestionsUsed) {
			continue
		}

		// Perform comprehensive validation
		if err := s.validateQuestionEligibility(&question, session, string(criteria.Stage)); err != nil {
			fmt.Printf("Warning: Cached question failed validation: %s, error: %v\n", question.ID, err)
			continue
		}

		// Update session with used question immediately
		updateErr := s.addQuestionToUsed(ctx, session.ID, question.ID)
		if updateErr != nil {
			fmt.Printf("Warning: Failed to update used questions for session %s: %v\n", session.ID, updateErr)
		}

		fmt.Printf("Selected cached question %s for stage %s from pool %s\n", question.ID, criteria.Stage, cacheKey)
		return &question, nil
	}

	return nil, fmt.Errorf("no available questions in cached pool %s: all questions used or failed validation", cacheKey)
}

func (s *SessionService) getBloomDistribution(difficulty string) map[string]float64 {
	// Get global configuration for Bloom distributions
	config, err := s.ConfigService.GetDefaultConfig(context.Background())
	if err != nil || config.StageConfig == nil {
		// Fallback to hardcoded values if config unavailable
		return s.getHardcodedBloomDistribution(difficulty)
	}

	// Use configuration-based distribution
	if stageConfig, exists := config.StageConfig[difficulty]; exists {
		return map[string]float64{
			"remember":   stageConfig.InitialBloomDistribution.Remember,
			"understand": stageConfig.InitialBloomDistribution.Understand,
			"apply":      stageConfig.InitialBloomDistribution.Apply,
			"analyze":    stageConfig.InitialBloomDistribution.Analyze,
			"evaluate":   stageConfig.InitialBloomDistribution.Evaluate,
			"create":     stageConfig.InitialBloomDistribution.Create,
		}
	}

	return s.getHardcodedBloomDistribution(difficulty)
}

// Fallback hardcoded distribution
func (s *SessionService) getHardcodedBloomDistribution(difficulty string) map[string]float64 {
	switch difficulty {
	case "easy":
		return map[string]float64{
			"remember": 0.5, "understand": 0.3, "apply": 0.2,
		}
	case "medium":
		return map[string]float64{
			"understand": 0.3, "apply": 0.4, "analyze": 0.3,
		}
	case "hard":
		return map[string]float64{
			"apply": 0.2, "analyze": 0.4, "evaluate": 0.3, "create": 0.1,
		}
	default:
		return map[string]float64{
			"remember": 0.2, "understand": 0.2, "apply": 0.2,
			"analyze": 0.2, "evaluate": 0.2,
		}
	}
}

// getRecoveryBloomDistribution returns recovery-specific Bloom distributions
func (s *SessionService) getRecoveryBloomDistribution(difficulty string) map[string]float64 {
	// Get global configuration for recovery Bloom distributions
	config, err := s.ConfigService.GetDefaultConfig(context.Background())
	if err != nil || config.StageConfig == nil {
		// Fallback to hardcoded recovery values
		return s.getHardcodedRecoveryBloomDistribution(difficulty)
	}

	// Use configuration-based recovery distribution
	if stageConfig, exists := config.StageConfig[difficulty]; exists {
		return map[string]float64{
			"remember":   stageConfig.RecoveryBloomDistribution.Remember,
			"understand": stageConfig.RecoveryBloomDistribution.Understand,
			"apply":      stageConfig.RecoveryBloomDistribution.Apply,
			"analyze":    stageConfig.RecoveryBloomDistribution.Analyze,
			"evaluate":   stageConfig.RecoveryBloomDistribution.Evaluate,
			"create":     stageConfig.RecoveryBloomDistribution.Create,
		}
	}

	return s.getHardcodedRecoveryBloomDistribution(difficulty)
}

// Fallback hardcoded recovery distribution
func (s *SessionService) getHardcodedRecoveryBloomDistribution(difficulty string) map[string]float64 {
	switch difficulty {
	case "easy":
		return map[string]float64{
			"remember": 0.6, "understand": 0.3, "apply": 0.1,
		}
	case "medium":
		return map[string]float64{
			"remember": 0.4, "understand": 0.3, "apply": 0.2, "analyze": 0.1,
		}
	case "hard":
		return map[string]float64{
			"remember": 0.25, "understand": 0.25, "apply": 0.25, "analyze": 0.15, "evaluate": 0.1,
		}
	default:
		return map[string]float64{
			"remember": 0.4, "understand": 0.3, "apply": 0.2, "analyze": 0.1,
		}
	}
}

func (s *SessionService) mapStageToDifficulty(stage adaptive.Stage) string {
	switch stage {
	case adaptive.StageEasy:
		return "easy"
	case adaptive.StageMedium:
		return "medium"
	case adaptive.StageHard:
		return "hard"
	default:
		return "easy"
	}
}

// generatePoolCacheKey generates a cache key for session-specific pools
func (s *SessionService) generatePoolCacheKey(sessionID string, session *models.QuizSession) string {
	stage := session.CurrentStage
	progress := session.StageProgress[stage]

	if progress.RecoveryRound > 0 {
		return fmt.Sprintf("session_%s_%s_recovery", sessionID, stage)
	}
	return fmt.Sprintf("session_%s_%s_initial", sessionID, stage)
}

// getPoolFromCache retrieves pool from global cache
func (s *SessionService) getPoolFromCache(cacheKey string) *selection.QuizPool {
	if pool, exists := selection.GetPoolFromCache(cacheKey); exists {
		return pool
	}
	return nil
}

// generateSessionPool creates and caches pools for a session
func (s *SessionService) generateSessionPool(ctx context.Context, sessionID string, session *models.QuizSession) error {
	skillInfo := s.getSkillInfoFromSession(session)
	if skillInfo == nil {
		return fmt.Errorf("skill information not found for session")
	}

	// Generate pools for all stages
	stages := []string{"easy", "medium", "hard"}
	for _, stage := range stages {
		// Generate initial pool
		initialKey := fmt.Sprintf("session_%s_%s_initial", sessionID, stage)
		if err := s.generateStagePool(ctx, initialKey, skillInfo, stage, false); err != nil {
			return fmt.Errorf("failed to generate initial pool for stage %s: %w", stage, err)
		}

		// Generate recovery pool
		recoveryKey := fmt.Sprintf("session_%s_%s_recovery", sessionID, stage)
		if err := s.generateStagePool(ctx, recoveryKey, skillInfo, stage, true); err != nil {
			return fmt.Errorf("failed to generate recovery pool for stage %s: %w", stage, err)
		}
	}

	return nil
}

// generateStagePool generates a pool for a specific stage and caches it
func (s *SessionService) generateStagePool(ctx context.Context, cacheKey string, skillInfo *selection.SkillInfo, stage string, isRecovery bool) error {
	poolManager := selection.NewPoolManager(s.QuestionRepo)

	// Determine bloom distribution based on stage and recovery status
	var bloomDistribution map[string]float64
	if isRecovery {
		bloomDistribution = s.getRecoveryBloomDistribution(stage)
	} else {
		bloomDistribution = s.getBloomDistribution(stage)
	}

	// Get pool size from configuration
	poolSize := s.getPoolSizeFromConfig(isRecovery)

	// Select questions for the pool
	result, err := poolManager.SelectAdaptiveQuestionsWithBloomGlobal(
		ctx,
		skillInfo,
		stage,
		poolSize,
		[]string{}, // No exclusions for pool generation
		bloomDistribution,
	)
	if err != nil {
		return fmt.Errorf("failed to select questions for pool: %w", err)
	}

	// Create pool structure
	pool := &selection.QuizPool{
		ID:                fmt.Sprintf("session_pool_%s", cacheKey),
		SkillID:           skillInfo.ID,
		SkillTags:         skillInfo.Tags,
		Questions:         result.Questions,
		TotalCount:        len(result.Questions),
		BloomDistribution: result.BloomCoverage,
	}

	// Cache the pool
	selection.SetPoolInCache(cacheKey, pool)

	fmt.Printf("Generated and cached pool %s with %d questions\n", cacheKey, pool.TotalCount)
	return nil
}

func (s *SessionService) isQuestionUsed(questionID string, usedIDs []string) bool {
	return slices.Contains(usedIDs, questionID)
}

// addQuestionToUsed adds a question ID to the session's used questions list
func (s *SessionService) addQuestionToUsed(ctx context.Context, sessionID, questionID string) error {
	// Update session's QuestionsUsed array in database
	update := bson.M{
		"$addToSet": bson.M{
			"questions_used": questionID,
		},
	}

	return s.Repo.Update(ctx, sessionID, update)
}

func (s *SessionService) generateSessionToken() string {
	return fmt.Sprintf("session_%s_%d", primitive.NewObjectID().Hex(), time.Now().UnixNano())
}

func (s *SessionService) extractSkillID(session *models.QuizSession) string {
	if metadata := session.Metadata; metadata != nil {
		if skillID, ok := metadata["skill_id"].(string); ok {
			return skillID
		}
	}
	return ""
}

// calculateTimeRemaining calculates remaining time for session with detailed logging
func (s *SessionService) calculateTimeRemaining(session *models.QuizSession) int {
	log.Printf("[TIME_CALCULATION] Starting time remaining calculation for session %s", session.ID)
	calcStart := time.Now()

	// Default 60 minutes total time
	totalTime := 3600
	log.Printf("[TIME_CALCULATION] Default total time: %d seconds (60 minutes)", totalTime)

	// Check for custom total time in metadata
	if metadata := session.Metadata; metadata != nil {
		log.Printf("[TIME_CALCULATION] Checking metadata for custom total time")
		if config, ok := metadata["quiz_config"].(map[string]interface{}); ok {
			log.Printf("[TIME_CALCULATION] Found quiz_config in metadata")
			if duration, ok := config["total_duration_seconds"].(int); ok {
				totalTime = duration
				log.Printf("[TIME_CALCULATION] Using custom total time from metadata: %d seconds", totalTime)
			} else {
				log.Printf("[TIME_CALCULATION] total_duration_seconds not found in config, using default")
			}
		} else {
			log.Printf("[TIME_CALCULATION] quiz_config not found in metadata, using default")
		}
	} else {
		log.Printf("[TIME_CALCULATION] No metadata found, using default total time")
	}

	// Calculate elapsed and remaining time
	elapsed := int(time.Since(session.StartTime).Seconds())
	remaining := totalTime - elapsed
	log.Printf("[TIME_CALCULATION] Time breakdown - Total: %ds, Elapsed: %ds, Raw remaining: %ds", 
		totalTime, elapsed, remaining)

	if remaining < 0 {
		log.Printf("[TIME_CALCULATION] Time exceeded, setting remaining to 0 (was %d)", remaining)
		remaining = 0
	}

	calcDuration := time.Since(calcStart)
	log.Printf("[TIME_CALCULATION] Time calculation completed (took %v) - Final remaining: %ds (%.1f minutes)", 
		calcDuration, remaining, float64(remaining)/60.0)

	return remaining
}

// buildBloomBreakdown creates comprehensive Bloom taxonomy performance breakdown
func (s *SessionService) buildBloomBreakdown(session *adaptive.AdaptiveSession) models.BloomBreakdown {
	breakdown := models.BloomBreakdown{}

	// Convert session performance to result format
	for level, perf := range session.BloomPerformance {
		// Calculate final metrics
		perf.CalculateMetrics()

		levelPerf := models.BloomLevelPerformance{
			QuestionsAttempted:   perf.QuestionsAttempted,
			QuestionsCorrect:     perf.QuestionsCorrect,
			ActualScore:          perf.ActualScore,
			PossibleScore:        perf.PossibleScore,
			AccuracyPercentage:   perf.AccuracyPercentage,
			ScorePercentage:      perf.ScorePercentage,
			AverageQuestionScore: perf.AverageQuestionScore,
			EfficiencyRating:     perf.EfficiencyRating,
			TotalTimeSpent:       perf.TotalTimeSpent,
			AverageTimePerQ:      perf.AverageTimePerQ,
		}

		// Assign to appropriate field
		switch level {
		case "remember":
			breakdown.Remember = levelPerf
		case "understand":
			breakdown.Understand = levelPerf
		case "apply":
			breakdown.Apply = levelPerf
		case "analyze":
			breakdown.Analyze = levelPerf
		case "evaluate":
			breakdown.Evaluate = levelPerf
		case "create":
			breakdown.Create = levelPerf
		}
	}

	// Generate cognitive profile
	breakdown.Summary = s.generateCognitiveProfile(session.BloomPerformance)

	return breakdown
}

// generateCognitiveProfile analyzes overall cognitive performance patterns
func (s *SessionService) generateCognitiveProfile(performance map[string]*adaptive.BloomLevelPerformance) models.CognitiveProfile {
	type levelScore struct {
		name       string
		percentage float64
		complexity int
	}

	// Cognitive complexity weights for each Bloom level
	complexityWeights := map[string]int{
		"remember": 1, "understand": 2, "apply": 3,
		"analyze": 4, "evaluate": 5, "create": 6,
	}

	var levels []levelScore
	totalActual := 0.0
	totalPossible := 0.0
	weightedComplexity := 0.0

	// Analyze performance per level
	for level, perf := range performance {
		if perf.QuestionsAttempted > 0 {
			levels = append(levels, levelScore{
				name:       level,
				percentage: perf.ScorePercentage,
				complexity: complexityWeights[level],
			})

			totalActual += perf.ActualScore
			totalPossible += perf.PossibleScore
			weightedComplexity += perf.ScorePercentage * float64(complexityWeights[level])
		}
	}

	// Sort by performance for analysis
	sort.Slice(levels, func(i, j int) bool {
		return levels[i].percentage > levels[j].percentage
	})

	// Identify patterns
	var strengths, growthAreas, recommendations []string

	for i, level := range levels {
		if i < 2 && level.percentage >= 75 {
			strengths = append(strengths, level.name)
		}
		if i >= len(levels)-2 && level.percentage < 60 {
			growthAreas = append(growthAreas, level.name)

			// Generate recommendations
			recommendations = append(recommendations,
				s.generateLearningRecommendation(level.name, level.percentage))
		}
	}

	avgComplexity := 0.0
	if len(levels) > 0 {
		avgComplexity = weightedComplexity / float64(len(levels))
	}

	overallPercentage := 0.0
	if totalPossible > 0 {
		overallPercentage = (totalActual / totalPossible) * 100
	}

	return models.CognitiveProfile{
		DominantStrengths:       strengths,
		GrowthAreas:             growthAreas,
		CognitiveComplexity:     avgComplexity,
		OverallPercentage:       overallPercentage,
		LearningRecommendations: recommendations,
	}
}

// generateLearningRecommendation creates personalized learning suggestions
func (s *SessionService) generateLearningRecommendation(bloomLevel string, percentage float64) string {
	recommendations := map[string]string{
		"remember":   "Focus on memorization techniques and factual recall exercises",
		"understand": "Practice explaining concepts in your own words and creating summaries",
		"apply":      "Work on practical exercises and real-world problem-solving scenarios",
		"analyze":    "Practice breaking down complex problems into components and identifying patterns",
		"evaluate":   "Work on critical thinking exercises and making evidence-based judgments",
		"create":     "Engage in project-based learning and original content creation",
	}

	base := recommendations[bloomLevel]

	if percentage < 40 {
		return base + " - Start with foundational concepts and guided practice"
	} else if percentage < 60 {
		return base + " - Focus on structured practice with immediate feedback"
	} else {
		return base + " - Practice with increasing complexity and independence"
	}
}

func (s *SessionService) createQuizResult(session *models.QuizSession, completionType string, finalScore float64) *models.QuizResult {
	// Calculate badge level using configuration
	badgeLevel := ""
	if session.CompletionType != models.ManualSubmit {
		badgeLevel = s.calculateBadgeLevelFromConfig(finalScore)
	} else {
		badgeLevel = "Unidentified"
	}

	// Build stage breakdown
	stageBreakdown := make(map[string]models.StageBreakdown)
	for stage, progress := range session.StageProgress {
		percentage := 0.0
		if progress.Attempted > 0 {
			percentage = (float64(progress.Correct) / float64(progress.Attempted)) * 100
		}
		stageBreakdown[stage] = models.StageBreakdown{
			Attempted:    progress.Attempted,
			Correct:      progress.Correct,
			Score:        progress.Score,
			Percentage:   percentage,
			Passed:       progress.Passed,
			RecoveryUsed: progress.RecoveryRound > 0,
		}
	}

	// Calculate totals
	totalAttempted := 0
	totalCorrect := 0
	for _, progress := range session.StageProgress {
		totalAttempted += progress.Attempted
		totalCorrect += progress.Correct
	}

	// Build comprehensive Bloom breakdown
	adaptiveSession := s.reconstructAdaptiveSession(session)
	bloomBreakdown := s.buildBloomBreakdown(adaptiveSession)

	// Calculate average time per question safely to prevent +Inf values
	totalTimeSeconds := int(time.Since(session.StartTime).Seconds())
	averageTimePerQuestion := 0.0
	if totalAttempted > 0 {
		averageTimePerQuestion = float64(totalTimeSeconds) / float64(totalAttempted)
	}

	// Validate finalScore for infinite values before creating result
	if math.IsInf(finalScore, 0) || math.IsNaN(finalScore) {
		fmt.Printf("[SessionService] WARNING: Invalid final score detected (%v), defaulting to 0\n", finalScore)
		finalScore = 0.0
	}

	return &models.QuizResult{
		SessionID:          session.ID,
		UserID:             session.UserID,
		ConfigID:           session.ConfigID,
		FinalScore:         finalScore,
		Percentage:         finalScore,
		BadgeLevel:         badgeLevel,
		QuestionsAttempted: totalAttempted,
		QuestionsCorrect:   totalCorrect,
		StageBreakdown:     stageBreakdown,
		TimeBreakdown: models.TimeBreakdown{
			TotalTimeSeconds:       totalTimeSeconds,
			AverageTimePerQuestion: averageTimePerQuestion,
		},
		BloomBreakdown: bloomBreakdown,
		CompletionType: completionType,
		CreatedAt:      time.Now(),
	}
}

// CacheAnswer stores an answer in the session cache instead of database
func (s *SessionService) CacheAnswer(sessionID string, answer *models.QuizAnswer, questionType, bloomLevel string) {
	cachedAnswer := models.ConvertQuizAnswerToCached(answer, questionType, bloomLevel)
	s.answerCache.AddAnswer(sessionID, cachedAnswer)
}

// GetCachedAnswers retrieves cached answers for a session
func (s *SessionService) GetCachedAnswers(sessionID string) ([]models.CachedAnswer, bool) {
	return s.answerCache.GetAnswers(sessionID)
}

// GetCachedAnswerCount returns the number of cached answers for a session
func (s *SessionService) GetCachedAnswerCount(sessionID string) int {
	return s.answerCache.GetAnswerCount(sessionID)
}

// MarkSessionCacheCompleted marks a session as completed in cache for retention timing
func (s *SessionService) MarkSessionCacheCompleted(sessionID string) {
	s.answerCache.MarkSessionCompleted(sessionID)
}

// extractKnowledgeData creates rich skills analytics from session and result data for skills.events exchange
func (s *SessionService) extractKnowledgeData(session *models.QuizSession, result *models.QuizResult) map[string]interface{} {
	// Extract skill progressions with Bloom level analysis
	skillProgressions := make(map[string]interface{})
	skillID := s.extractSkillID(session)

	if skillID != "" {
		skillProgressions[skillID] = map[string]interface{}{
			"bloom_breakdown":   result.BloomBreakdown,
			"mastery_level":     s.calculateMasteryLevel(result.FinalScore),
			"improvement":       s.calculateImprovement(session, result),
			"stage_performance": s.analyzeStagePerformance(session),
		}
	}

	// Build cognitive profile from session patterns
	cognitiveProfile := map[string]interface{}{
		"analytical_strength": s.calculateAnalyticalStrength(&result.BloomBreakdown),
		"memory_retention":    s.calculateMemoryRetention(&result.BloomBreakdown),
		"problem_solving":     s.calculateProblemSolving(&result.BloomBreakdown),
		"adaptation_speed":    s.calculateAdaptationSpeed(session),
	}

	// Extract learning patterns
	learningPatterns := map[string]interface{}{
		"optimal_difficulty":       s.determineOptimalDifficulty(session),
		"preferred_question_types": s.analyzeQuestionTypePreferences(session),
		"time_per_question":        s.calculateAverageTimePerQuestion(session),
		"recovery_effectiveness":   s.analyzeRecoveryEffectiveness(session),
	}

	// Build comprehensive skills event payload for skills.events exchange
	return map[string]any{
		"user_id":    session.UserID,
		"session_id": session.ID,
		"config_id":  session.ConfigID,
		"timestamp":  time.Now(),
		"event_type": "quiz_completion",
		"exchange":   "skills.events",
		"skills_data": map[string]any{
			"skill_progressions": skillProgressions,
			"cognitive_profile":  cognitiveProfile,
			"learning_patterns":  learningPatterns,
		},
		"performance_metrics": map[string]any{
			"final_score":         result.FinalScore,
			"badge_level":         result.BadgeLevel,
			"stage_breakdown":     result.StageBreakdown,
			"bloom_breakdown":     result.BloomBreakdown,
			"time_breakdown":      result.TimeBreakdown,
			"completion_type":     result.CompletionType,
			"duration_seconds":    int(time.Since(session.StartTime).Seconds()),
			"questions_attempted": result.QuestionsAttempted,
			"questions_correct":   result.QuestionsCorrect,
		},
		"session_metadata": map[string]any{
			"skill_id":         skillID,
			"total_questions":  session.TotalQuestionsAsked,
			"stages_completed": s.countCompletedStages(session),
			"recovery_rounds":  s.countRecoveryRounds(session),
		},
	}
}

// Helper methods for knowledge analytics
func (s *SessionService) calculateMasteryLevel(score float64) string {
	return s.calculateBadgeLevelFromConfig(score)
}

// calculateBadgeLevelFromConfig determines badge level using global configuration
func (s *SessionService) calculateBadgeLevelFromConfig(score float64) string {
	config, err := s.ConfigService.GetDefaultConfig(context.Background())
	if err != nil || config.ScoringConfig.BadgeThresholds.Expert == 0 {
		// Fallback to hardcoded values
		return s.getHardcodedBadgeLevel(score)
	}

	// Use configuration-based thresholds
	if score >= config.ScoringConfig.BadgeThresholds.Expert {
		return "expert"
	} else if score >= config.ScoringConfig.BadgeThresholds.Proficient {
		return "proficient"
	} else if score >= config.ScoringConfig.BadgeThresholds.Intermediate {
		return "intermediate"
	}
	return "beginner"
}

// Fallback hardcoded badge levels
func (s *SessionService) getHardcodedBadgeLevel(score float64) string {
	if score >= 90 {
		return "expert"
	} else if score >= 75 {
		return "proficient"
	} else if score >= 60 {
		return "intermediate"
	}
	return "beginner"
}

// createAnswerCacheFromConfig creates answer cache using configuration-based retention
func (s *SessionService) createAnswerCacheFromConfig() *models.SessionAnswerCache {
	config, err := s.ConfigService.GetDefaultConfig(context.Background())
	if err != nil {
		// Fallback to hardcoded 30-minute retention
		return models.NewSessionAnswerCache(30 * time.Minute)
	}

	retention := time.Duration(config.CacheConfig.AnswerCacheRetentionMinutes) * time.Minute
	return models.NewSessionAnswerCache(retention)
}

// getPoolSizeFromConfig returns pool size from configuration
func (s *SessionService) getPoolSizeFromConfig(isRecovery bool) int {
	config, err := s.ConfigService.GetDefaultConfig(context.Background())
	if err != nil {
		// Fallback to hardcoded values
		if isRecovery {
			return 25 // Recovery pool size
		}
		return 50 // Initial pool size
	}

	if isRecovery {
		return config.PoolConfig.RecoveryPoolSize
	}
	return config.PoolConfig.InitialPoolSize
}

// getTimeoutConfigFromGlobalConfig creates timeout config from global configuration
func (s *SessionService) getTimeoutConfigFromGlobalConfig() *timeout.TimeoutConfig {
	config, err := s.ConfigService.GetDefaultConfig(context.Background())
	if err != nil {
		// Fallback to default timeout configuration
		return timeout.DefaultTimeoutConfig()
	}

	return &timeout.TimeoutConfig{
		SessionTimeoutMinutes:    int(config.TotalDurationSeconds / 60), // Convert to minutes
		QuestionTimeoutMinutes:   30,                                    // 30 minutes max per question
		FirstQuestionTimeout:     300,                                   // 5 minutes to get first question
		InactivityTimeoutMinutes: 15,                                    // 15 minutes inactivity
	}
}

// handleSessionTimeout handles session timeout events
func (s *SessionService) handleSessionTimeout(sessionID string) {
	ctx := context.Background()

	// Get session to check current state
	session, err := s.Repo.FindByID(ctx, sessionID)
	if err != nil {
		fmt.Printf("Error getting session %s for timeout handling: %v\n", sessionID, err)
		return
	}

	if session == nil {
		fmt.Printf("Session %s not found during timeout handling\n", sessionID)
		return
	}

	// Only timeout active sessions
	if session.Status != "active" {
		fmt.Printf("Session %s is not active (status: %s), skipping timeout\n", sessionID, session.Status)
		return
	}

	fmt.Printf("Handling timeout for session %s (user: %s)\n", sessionID, session.UserID)

	// Update session status to timed out
	update := bson.M{
		"status":          "timeout",
		"completion_type": "timeout",
		"end_time":        time.Now(),
	}

	err = s.Repo.Update(ctx, sessionID, update)
	if err != nil {
		fmt.Printf("Error updating session %s status to timeout: %v\n", sessionID, err)
	}

	// Mark session cache as completed for proper cleanup
	s.MarkSessionCacheCompleted(sessionID)

	// Publish timeout event
	if s.EventPublisher != nil {
		s.EventPublisher.Publish("quiz.session.timeout", map[string]any{
			"session_id":     sessionID,
			"user_id":        session.UserID,
			"timeout_reason": "session_timeout",
			"timestamp":      time.Now(),
		})
	}

	// Clean up session caches
	delete(s.sessionSkillCache, sessionID)
	delete(s.sessionEnhancedSkillCache, sessionID)

	// Clear current question state for timed out session
	s.questionStateCache.ClearCurrentQuestion(sessionID)

	fmt.Printf("Session %s successfully timed out and cleaned up\n", sessionID)
}

// GetSessionTimeoutInfo returns timeout information for a session
func (s *SessionService) GetSessionTimeoutInfo(sessionID string) *timeout.TimeoutInfo {
	return s.timeoutManager.GetTimeoutInfo(sessionID)
}

// ExtendSessionTimeout extends session timeout (e.g., when user is active)
func (s *SessionService) ExtendSessionTimeout(sessionID string, additionalMinutes int) {
	additionalTime := time.Duration(additionalMinutes) * time.Minute
	s.timeoutManager.ExtendTimeout(sessionID, additionalTime)
}

// startMainSessionTimeout starts the main session countdown timer
func (s *SessionService) startMainSessionTimeout(sessionID string) {
	ctx := context.Background()

	// Get session to determine timeout duration
	session, err := s.Repo.FindByID(ctx, sessionID)
	if err != nil {
		fmt.Printf("Error getting session %s for starting main timeout: %v\n", sessionID, err)
		return
	}

	if session == nil {
		fmt.Printf("Session %s not found when starting main timeout\n", sessionID)
		return
	}

	// Get global config to determine session timeout duration
	config, err := s.ConfigService.GetConfigForSession(ctx, session.ConfigID)
	if err != nil {
		fmt.Printf("Error getting config for session %s timeout: %v\n", sessionID, err)
		// Use default timeout
		config = &models.GlobalQuizConfig{
			TotalDurationSeconds: 3600, // 1 hour default
		}
	}

	// Start main session timeout based on configuration
	sessionTimeout := time.Duration(config.TotalDurationSeconds) * time.Second
	s.timeoutManager.StartSessionTimeout(sessionID, sessionTimeout)

	fmt.Printf("Main session timeout started for %s (duration: %v) - countdown begins now\n", sessionID, sessionTimeout)
}

func (s *SessionService) calculateImprovement(session *models.QuizSession, result *models.QuizResult) float64 {
	// Calculate improvement based on stage progression
	// This could be enhanced with historical data comparison
	if len(session.StageProgress) == 0 {
		return 0.0
	}

	// Simple improvement calculation based on stage progression
	totalStages := float64(len(session.StageProgress))
	passedStages := 0.0

	for _, progress := range session.StageProgress {
		if progress.Passed {
			passedStages++
		}
	}

	return (passedStages / totalStages) * 0.3 // 30% max improvement score
}

func (s *SessionService) analyzeStagePerformance(session *models.QuizSession) map[string]any {
	stagePerf := make(map[string]any)

	for stage, progress := range session.StageProgress {
		accuracy := 0.0
		if progress.Attempted > 0 {
			accuracy = float64(progress.Correct) / float64(progress.Attempted)
		}

		stagePerf[stage] = map[string]any{
			"accuracy":        accuracy,
			"attempts":        progress.Attempted,
			"passed":          progress.Passed,
			"recovery_rounds": progress.RecoveryRound,
			"efficiency":      s.calculateStageEfficiency(progress),
		}
	}

	return stagePerf
}

func (s *SessionService) calculateStageEfficiency(progress models.StageProgress) float64 {
	if progress.Attempted == 0 {
		return 0.0
	}

	// Efficiency based on correct answers vs attempts and recovery usage
	baseEfficiency := float64(progress.Correct) / float64(progress.Attempted)

	// Penalty for recovery rounds (indicates difficulty or mistakes)
	recoveryPenalty := float64(progress.RecoveryRound) * 0.1

	return math.Max(0.0, baseEfficiency-recoveryPenalty)
}

func (s *SessionService) calculateAnalyticalStrength(bloomBreakdown *models.BloomBreakdown) float64 {
	// Focus on higher-order thinking skills
	analytical := 0.0
	total := 0.0

	// Analyze level
	if bloomBreakdown.Analyze.QuestionsAttempted > 0 {
		analytical += float64(bloomBreakdown.Analyze.QuestionsCorrect)
		total += float64(bloomBreakdown.Analyze.QuestionsAttempted)
	}

	// Evaluate level
	if bloomBreakdown.Evaluate.QuestionsAttempted > 0 {
		analytical += float64(bloomBreakdown.Evaluate.QuestionsCorrect)
		total += float64(bloomBreakdown.Evaluate.QuestionsAttempted)
	}

	// Create level
	if bloomBreakdown.Create.QuestionsAttempted > 0 {
		analytical += float64(bloomBreakdown.Create.QuestionsCorrect)
		total += float64(bloomBreakdown.Create.QuestionsAttempted)
	}

	if total == 0 {
		return 0.5 // Neutral score if no analytical questions
	}

	return analytical / total
}

func (s *SessionService) calculateMemoryRetention(bloomBreakdown *models.BloomBreakdown) float64 {
	// Focus on remember level
	if bloomBreakdown.Remember.QuestionsAttempted > 0 {
		return float64(bloomBreakdown.Remember.QuestionsCorrect) / float64(bloomBreakdown.Remember.QuestionsAttempted)
	}

	return 0.5 // Neutral score if no memory questions
}

func (s *SessionService) calculateProblemSolving(bloomBreakdown *models.BloomBreakdown) float64 {
	// Focus on apply and analyze levels
	problemSolving := 0.0
	total := 0.0

	// Apply level
	if bloomBreakdown.Apply.QuestionsAttempted > 0 {
		problemSolving += float64(bloomBreakdown.Apply.QuestionsCorrect)
		total += float64(bloomBreakdown.Apply.QuestionsAttempted)
	}

	// Analyze level
	if bloomBreakdown.Analyze.QuestionsAttempted > 0 {
		problemSolving += float64(bloomBreakdown.Analyze.QuestionsCorrect)
		total += float64(bloomBreakdown.Analyze.QuestionsAttempted)
	}

	if total == 0 {
		return 0.5 // Neutral score
	}

	return problemSolving / total
}

func (s *SessionService) calculateAdaptationSpeed(session *models.QuizSession) float64 {
	// Measure how quickly user adapts between difficulty stages
	if len(session.StageProgress) <= 1 {
		return 0.5 // Neutral if single stage
	}

	// Simple metric: fewer questions needed per stage indicates faster adaptation
	totalQuestions := session.TotalQuestionsAsked
	stages := len(session.StageProgress)

	questionsPerStage := float64(totalQuestions) / float64(stages)

	// Lower questions per stage = faster adaptation (normalize to 0-1)
	// Assume 10 questions per stage is average (0.5), fewer is better
	if questionsPerStage <= 5 {
		return 1.0
	} else if questionsPerStage >= 15 {
		return 0.0
	} else {
		return 1.0 - ((questionsPerStage - 5) / 10)
	}
}

func (s *SessionService) determineOptimalDifficulty(session *models.QuizSession) string {
	// Analyze performance across stages to determine optimal difficulty
	bestStage := ""
	bestScore := 0.0

	for stage, progress := range session.StageProgress {
		if progress.Attempted > 0 {
			score := float64(progress.Correct) / float64(progress.Attempted)
			if score > bestScore {
				bestScore = score
				bestStage = stage
			}
		}
	}

	if bestStage == "" {
		return "moderate"
	}

	return bestStage
}

func (s *SessionService) analyzeQuestionTypePreferences(session *models.QuizSession) []string {
	// This would require tracking question types answered
	// For now, return common types - could be enhanced with actual tracking
	return []string{"multiple_choice", "single_choice", "true_false"}
}

func (s *SessionService) calculateAverageTimePerQuestion(session *models.QuizSession) float64 {
	if session.TotalQuestionsAsked == 0 {
		return 0.0
	}

	totalTime := time.Since(session.StartTime).Seconds()
	return totalTime / float64(session.TotalQuestionsAsked)
}

func (s *SessionService) analyzeRecoveryEffectiveness(session *models.QuizSession) float64 {
	totalRecoveryRounds := 0
	successfulRecoveries := 0

	for _, progress := range session.StageProgress {
		if progress.RecoveryRound > 0 {
			totalRecoveryRounds += progress.RecoveryRound
			if progress.Passed {
				successfulRecoveries++
			}
		}
	}

	if totalRecoveryRounds == 0 {
		return 1.0 // No recovery needed = perfect
	}

	return float64(successfulRecoveries) / float64(len(session.StageProgress))
}

func (s *SessionService) countCompletedStages(session *models.QuizSession) int {
	completed := 0
	for _, progress := range session.StageProgress {
		if progress.Passed {
			completed++
		}
	}
	return completed
}

func (s *SessionService) countRecoveryRounds(session *models.QuizSession) int {
	total := 0
	for _, progress := range session.StageProgress {
		total += progress.RecoveryRound
	}
	return total
}

// RemoveSessionCache removes all cached data for a session
func (s *SessionService) RemoveSessionCache(sessionID string) {
	s.answerCache.RemoveSession(sessionID)
}

// GetAnswerCacheStats returns cache statistics for monitoring
func (s *SessionService) GetAnswerCacheStats() map[string]any {
	return s.answerCache.GetCacheStats()
}

// validateQuestionEligibility performs comprehensive question validation
func (s *SessionService) validateQuestionEligibility(question *models.Question, session *models.QuizSession, stage string) error {
	if question == nil {
		return fmt.Errorf("question is nil")
	}

	// Validate question ID
	if question.ID == "" {
		return fmt.Errorf("question has empty ID")
	}

	// Check if question is already used
	if s.isQuestionUsed(question.ID, session.QuestionsUsed) {
		return fmt.Errorf("question already used in session: %s", question.ID)
	}

	// Validate question structure
	if err := question.Validate(); err != nil {
		return fmt.Errorf("question validation failed: %w", err)
	}

	// Ensure question has required fields for adaptive logic
	if question.BloomLevel == "" {
		return fmt.Errorf("question missing Bloom level: %s", question.ID)
	}

	// Validate question has appropriate stage scoring
	stageScore := question.GetScoreForStage(stage)
	if stageScore <= 0 {
		return fmt.Errorf("question has invalid score for stage %s: %d", stage, stageScore)
	}

	// Check question type compatibility
	if !s.isQuestionTypeSupported(question.Type) {
		return fmt.Errorf("unsupported question type: %s", question.Type)
	}

	return nil
}

// isQuestionTypeSupported checks if the question type is supported
func (s *SessionService) isQuestionTypeSupported(questionType string) bool {
	supportedTypes := map[string]bool{
		string(models.QuestionTypeMultipleChoice): true,
		string(models.QuestionTypeSingleChoice):   true,
		string(models.QuestionTypeTrueFalse):      true,
	}

	// Check modern format first
	if supported, ok := supportedTypes[questionType]; ok {
		return supported
	}

	// Check legacy formats
	legacyTypes := map[string]bool{
		"multiple_choice": true,
		"single_choice":   true,
		"true_false":      true,
	}

	return legacyTypes[questionType]
}

// validateQuestionPool validates the question pool before selection
func (s *SessionService) validateQuestionPool(questions []models.Question, requiredCount int) error {
	if len(questions) == 0 {
		return fmt.Errorf("question pool is empty")
	}

	if len(questions) < requiredCount {
		return fmt.Errorf("insufficient questions in pool: found %d, required %d", len(questions), requiredCount)
	}

	// Validate each question in the pool
	validQuestions := 0
	for i, question := range questions {
		if err := question.Validate(); err != nil {
			fmt.Printf("Warning: Question %d in pool is invalid: %v\n", i, err)
			continue
		}

		if question.BloomLevel == "" {
			fmt.Printf("Warning: Question %s missing Bloom level\n", question.ID)
			continue
		}

		validQuestions++
	}

	if validQuestions < requiredCount {
		return fmt.Errorf("insufficient valid questions in pool: %d valid out of %d total, required %d",
			validQuestions, len(questions), requiredCount)
	}

	return nil
}

// === Question State Cache Methods ===

// GetCurrentQuestionInfo returns information about the current question for a session
func (s *SessionService) GetCurrentQuestionInfo(sessionID string) *models.SessionQuestionState {
	return s.questionStateCache.GetCurrentQuestion(sessionID)
}

// GetQuestionStateStats returns statistics about the question state cache
func (s *SessionService) GetQuestionStateStats() map[string]any {
	return s.questionStateCache.GetCacheStats()
}

// ClearSessionQuestionState manually clears the question state for a session (admin function)
func (s *SessionService) ClearSessionQuestionState(sessionID string) {
	s.questionStateCache.ClearCurrentQuestion(sessionID)
}

// GetQuestionStartTime returns when the current question was started for timing validation
func (s *SessionService) GetQuestionStartTime(sessionID string) *time.Time {
	return s.questionStateCache.GetQuestionStartTime(sessionID)
}

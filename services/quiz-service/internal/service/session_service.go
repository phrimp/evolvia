package service

import (
	"context"
	"fmt"
	"quiz-service/internal/adaptive"
	"quiz-service/internal/event"
	"quiz-service/internal/models"
	"quiz-service/internal/repository"
	"quiz-service/internal/selection"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SessionService handles quiz session operations
type SessionService struct {
	Repo                      *repository.SessionRepository
	QuizRepo                  *repository.QuizRepository
	QuestionRepo              *repository.QuestionRepository
	ResultRepo                *repository.ResultRepository
	EventPublisher            *event.EventPublisher
	adaptiveManager           *adaptive.Manager
	poolManager               *selection.PoolManager
	sessionSkillCache         map[string]*selection.SkillInfo
	sessionEnhancedSkillCache map[string]*selection.EnhancedSkillInfo // Add this
}

// NewSessionService creates a new session service
func NewSessionService(
	repo *repository.SessionRepository,
	quizRepo *repository.QuizRepository,
	questionRepo *repository.QuestionRepository,
) *SessionService {
	return &SessionService{
		Repo:                      repo,
		QuizRepo:                  quizRepo,
		QuestionRepo:              questionRepo,
		adaptiveManager:           adaptive.NewManager(nil),
		poolManager:               selection.NewPoolManager(questionRepo),
		sessionSkillCache:         make(map[string]*selection.SkillInfo),
		sessionEnhancedSkillCache: make(map[string]*selection.EnhancedSkillInfo), // Initialize
	}
}

// SetEventPublisher sets the event publisher
func (s *SessionService) SetEventPublisher(publisher *event.EventPublisher) {
	s.EventPublisher = publisher
}

// GetSession retrieves a session by ID
func (s *SessionService) GetSession(ctx context.Context, id string) (*models.QuizSession, error) {
	return s.Repo.FindByID(ctx, id)
}

// CreateSessionWithSkillValidation creates session with skill validation
func (s *SessionService) CreateSessionWithEnhancedSkillInfo(
	ctx context.Context,
	quizID string,
	userID string,
	skillInfo *selection.EnhancedSkillInfo,
	preferredBloomLevels []string,
	masteryScore int,
) (*models.QuizSession, error) {
	// Step 1: Check for past results (existing logic)
	var startingBloomLevel string
	var startingDifficulty string

	if s.ResultRepo != nil {
		pastResults, err := s.ResultRepo.FindByUser(ctx, userID)
		if err == nil && len(pastResults) > 0 {
			for _, result := range pastResults {
				if session, err := s.Repo.FindByID(ctx, result.SessionID); err == nil {
					if metadata := session.Metadata; metadata != nil {
						if sid, ok := metadata["skill_id"].(string); ok && sid == skillInfo.ID {
							startingBloomLevel = s.deriveBloomFromResult(&result)
							startingDifficulty = s.deriveDifficultyFromResult(&result)
							break
						}
					}
				}
			}
		}
	}

	// Step 2: Set defaults if no past results
	var bloomLevels []string
	if startingBloomLevel == "" {
		if len(preferredBloomLevels) > 0 {
			bloomLevels = preferredBloomLevels
			startingBloomLevel = preferredBloomLevels[0] // For backward compatibility
		} else {
			bloomLevels = []string{"remember"}
			startingBloomLevel = "remember"
		}
	} else {
		bloomLevels = []string{startingBloomLevel}
	}

	if masteryScore > 0 {
		if masteryScore <= 3 {
			startingDifficulty = "easy"
		} else if masteryScore <= 7 {
			startingDifficulty = "medium"
		} else {
			startingDifficulty = "hard"
		}
	} else {
		startingDifficulty = "easy"
	}

	// Step 3: Validate quiz pool with enhanced skill info
	quiz, err := s.QuizRepo.FindByID(ctx, quizID)
	if err != nil {
		return nil, fmt.Errorf("quiz not found: %w", err)
	}

	// Convert to standard SkillInfo for validation (backward compatibility)
	standardSkillInfo := &selection.SkillInfo{
		ID:   skillInfo.ID,
		Name: skillInfo.Name,
		Tags: s.mergeTags(skillInfo),
	}

	isValid, validation, err := s.poolManager.ValidateQuizPoolWithBloom(ctx, quizID, standardSkillInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to validate quiz pool: %w", err)
	}
	if !isValid {
		return nil, fmt.Errorf("insufficient questions in pool: %v", validation.Warnings)
	}

	initialStage := s.mapBloomToStage(startingBloomLevel)

	// Create session with enhanced metadata
	session := &models.QuizSession{
		QuizID:       quizID,
		UserID:       userID,
		SessionToken: s.generateSessionToken(),
		StartTime:    time.Now(),
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
			"quiz_config":            quiz.StageConfig,
			"quiz_start_time":        time.Now().Unix(),
		},
	}

	err = s.Repo.Create(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Cache enhanced skill info
	s.sessionSkillCache[session.ID] = standardSkillInfo
	s.sessionEnhancedSkillCache[session.ID] = skillInfo // Add new cache for enhanced info

	if s.EventPublisher != nil {
		s.EventPublisher.Publish("quiz.session.created", map[string]any{
			"session_id":           session.ID,
			"quiz_id":              quizID,
			"user_id":              userID,
			"skill_id":             skillInfo.ID,
			"tag_distribution":     s.getTagDistribution(skillInfo),
			"starting_bloom_level": startingBloomLevel,
			"starting_difficulty":  startingDifficulty,
		})
	}

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
	if result.Percentage >= 80 {
		return "hard"
	} else if result.Percentage >= 60 {
		return "medium"
	}
	return "easy"
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

	// Check for pre-generated pools first
	if pools, ok := session.Metadata["question_pools"].(map[string][]string); ok {
		question, err := s.getQuestionFromPreGeneratedPool(ctx, session, pools)
		if err == nil {
			return question, nil
		}
		// Fall back to dynamic selection if pre-generated fails
		fmt.Printf("Pre-generated pool failed, using dynamic selection: %v\n", err)
	}

	// Dynamic selection with Bloom's criteria
	adaptiveSession := s.reconstructAdaptiveSession(session)
	criteria, err := s.adaptiveManager.GetNextQuestionCriteria(adaptiveSession)
	if err != nil {
		return nil, err
	}

	// Select with Bloom's distribution
	questions, err := s.selectQuestionsWithBloomCriteria(ctx, session.QuizID, skillInfo, criteria)
	if err != nil {
		return nil, err
	}

	if len(questions) == 0 {
		return nil, fmt.Errorf("no available questions for current stage")
	}

	return &questions[0], nil
}

// SubmitSession completes and submits the session
func (s *SessionService) SubmitSession(
	ctx context.Context,
	sessionID string,
	completionType string,
	finalScore float64,
) (*models.QuizResult, error) {
	// Get session
	session, err := s.Repo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Calculate final score if not provided
	if finalScore == 0 {
		adaptiveSession := s.reconstructAdaptiveSession(session)
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
	if s.ResultRepo != nil {
		err = s.ResultRepo.Create(ctx, result)
		if err != nil {
			fmt.Printf("Failed to store result: %v\n", err)
		}
	}

	// Publish completion event
	if s.EventPublisher != nil {
		s.EventPublisher.Publish("quiz.session.completed", map[string]interface{}{
			"session_id":      sessionID,
			"user_id":         session.UserID,
			"quiz_id":         session.QuizID,
			"skill_id":        s.extractSkillID(session),
			"final_score":     finalScore,
			"completion_type": completionType,
			"duration":        int(time.Since(session.StartTime).Seconds()),
			"questions_asked": session.TotalQuestionsAsked,
		})
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

func (s *SessionService) reconstructAdaptiveSession(session *models.QuizSession) *adaptive.AdaptiveSession {
	adaptiveSession := adaptive.NewAdaptiveSession(session.ID)

	// Map current stage
	switch session.CurrentStage {
	case "easy":
		adaptiveSession.CurrentStage = adaptive.StageEasy
	case "medium":
		adaptiveSession.CurrentStage = adaptive.StageMedium
	case "hard":
		adaptiveSession.CurrentStage = adaptive.StageHard
	}

	// Map stage progress
	for stage, progress := range session.StageProgress {
		var adaptiveStage adaptive.Stage
		switch stage {
		case "easy":
			adaptiveStage = adaptive.StageEasy
		case "medium":
			adaptiveStage = adaptive.StageMedium
		case "hard":
			adaptiveStage = adaptive.StageHard
		default:
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
	}

	adaptiveSession.TotalQuestionsAsked = session.TotalQuestionsAsked
	adaptiveSession.UsedQuestionIDs = session.QuestionsUsed
	adaptiveSession.TotalScore = session.FinalScore
	adaptiveSession.IsComplete = session.Status == "completed"

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

func (s *SessionService) getQuestionFromPreGeneratedPool(
	ctx context.Context,
	session *models.QuizSession,
	pools map[string][]string,
) (*models.Question, error) {
	poolKey := s.determinePoolKey(session)

	if questionIDs, ok := pools[poolKey]; ok {
		for _, qID := range questionIDs {
			if !s.isQuestionUsed(qID, session.QuestionsUsed) {
				question, err := s.QuestionRepo.FindByID(ctx, qID)
				if err == nil {
					return question, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no available questions in pool")
}

func (s *SessionService) getBloomDistribution(difficulty string) map[string]float64 {
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

func (s *SessionService) determinePoolKey(session *models.QuizSession) string {
	stage := session.CurrentStage
	progress := session.StageProgress[stage]

	if progress.RecoveryRound > 0 {
		return fmt.Sprintf("%s_recovery", stage)
	}
	return fmt.Sprintf("%s_initial", stage)
}

func (s *SessionService) isQuestionUsed(questionID string, usedIDs []string) bool {
	for _, id := range usedIDs {
		if id == questionID {
			return true
		}
	}
	return false
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

func (s *SessionService) calculateTimeRemaining(session *models.QuizSession) int {
	// Default 60 minutes total time
	totalTime := 3600
	if metadata := session.Metadata; metadata != nil {
		if config, ok := metadata["quiz_config"].(map[string]interface{}); ok {
			if duration, ok := config["total_duration_seconds"].(int); ok {
				totalTime = duration
			}
		}
	}

	elapsed := int(time.Since(session.StartTime).Seconds())
	remaining := totalTime - elapsed
	if remaining < 0 {
		return 0
	}
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
	// Calculate badge level
	badgeLevel := "beginner"
	if finalScore >= 90 {
		badgeLevel = "expert"
	} else if finalScore >= 75 {
		badgeLevel = "proficient"
	} else if finalScore >= 60 {
		badgeLevel = "intermediate"
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

	return &models.QuizResult{
		SessionID:          session.ID,
		UserID:             session.UserID,
		QuizID:             session.QuizID,
		FinalScore:         finalScore,
		Percentage:         finalScore,
		BadgeLevel:         badgeLevel,
		QuestionsAttempted: totalAttempted,
		QuestionsCorrect:   totalCorrect,
		StageBreakdown:     stageBreakdown,
		TimeBreakdown: models.TimeBreakdown{
			TotalTimeSeconds:       int(time.Since(session.StartTime).Seconds()),
			AverageTimePerQuestion: float64(int(time.Since(session.StartTime).Seconds())) / float64(totalAttempted),
		},
		BloomBreakdown: bloomBreakdown,
		CompletionType: completionType,
		CreatedAt:      time.Now(),
	}
}

package models

import (
	"sync"
	"time"
)

// CachedAnswer represents an answer stored in memory cache instead of database
type CachedAnswer struct {
	QuestionID       string    `json:"question_id"`
	UserAnswer       string    `json:"user_answer"`
	IsCorrect        bool      `json:"is_correct"`
	PointsEarned     float64   `json:"points_earned"`
	TimeSpentSeconds int       `json:"time_spent_seconds"`
	AnsweredAt       time.Time `json:"answered_at"`
	QuestionType     string    `json:"question_type"`
	BloomLevel       string    `json:"bloom_level"`

	// Additional timing and integrity fields
	StartedAt       *time.Time `json:"started_at,omitempty"`
	ActualTimeSpent int        `json:"actual_time_spent,omitempty"`
}

// SessionAnswerCache manages cached answers for active sessions
type SessionAnswerCache struct {
	cache     map[string]*SessionAnswerData `json:"-"`
	mutex     sync.RWMutex                  `json:"-"`
	retention time.Duration                 `json:"-"`
}

// SessionAnswerData holds all cached data for a session
type SessionAnswerData struct {
	Answers     []CachedAnswer `json:"answers"`
	CreatedAt   time.Time      `json:"created_at"`
	LastAccess  time.Time      `json:"last_access"`
	SessionID   string         `json:"session_id"`
	UserID      string         `json:"user_id"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

// NewSessionAnswerCache creates a new answer cache with specified retention period
func NewSessionAnswerCache(retention time.Duration) *SessionAnswerCache {
	if retention == 0 {
		retention = 30 * time.Minute // Default: 30 minutes after session completion
	}

	cache := &SessionAnswerCache{
		cache:     make(map[string]*SessionAnswerData),
		retention: retention,
	}

	// Start background cleanup routine
	go cache.cleanupRoutine()

	return cache
}

// AddAnswer adds an answer to the session cache
func (sac *SessionAnswerCache) AddAnswer(sessionID string, answer CachedAnswer) {
	sac.mutex.Lock()
	defer sac.mutex.Unlock()

	if sac.cache[sessionID] == nil {
		sac.cache[sessionID] = &SessionAnswerData{
			Answers:    make([]CachedAnswer, 0),
			CreatedAt:  time.Now(),
			LastAccess: time.Now(),
			SessionID:  sessionID,
		}
	}

	sessionData := sac.cache[sessionID]
	sessionData.Answers = append(sessionData.Answers, answer)
	sessionData.LastAccess = time.Now()
}

// GetAnswers retrieves all cached answers for a session
func (sac *SessionAnswerCache) GetAnswers(sessionID string) ([]CachedAnswer, bool) {
	sac.mutex.RLock()
	defer sac.mutex.RUnlock()

	sessionData, exists := sac.cache[sessionID]
	if !exists {
		return nil, false
	}

	// Update last access time
	sessionData.LastAccess = time.Now()

	// Return copy of answers to prevent concurrent modification
	answers := make([]CachedAnswer, len(sessionData.Answers))
	copy(answers, sessionData.Answers)

	return answers, true
}

// GetAnswerCount returns the number of cached answers for a session
func (sac *SessionAnswerCache) GetAnswerCount(sessionID string) int {
	sac.mutex.RLock()
	defer sac.mutex.RUnlock()

	if sessionData, exists := sac.cache[sessionID]; exists {
		return len(sessionData.Answers)
	}
	return 0
}

// MarkSessionCompleted marks a session as completed for cache retention timing
func (sac *SessionAnswerCache) MarkSessionCompleted(sessionID string) {
	sac.mutex.Lock()
	defer sac.mutex.Unlock()

	if sessionData, exists := sac.cache[sessionID]; exists {
		now := time.Now()
		sessionData.CompletedAt = &now
		sessionData.LastAccess = now
	}
}

// RemoveSession removes all cached data for a session
func (sac *SessionAnswerCache) RemoveSession(sessionID string) {
	sac.mutex.Lock()
	defer sac.mutex.Unlock()

	delete(sac.cache, sessionID)
}

// GetCacheStats returns statistics about the cache
func (sac *SessionAnswerCache) GetCacheStats() map[string]interface{} {
	sac.mutex.RLock()
	defer sac.mutex.RUnlock()

	activeSessions := 0
	completedSessions := 0
	totalAnswers := 0

	for _, sessionData := range sac.cache {
		if sessionData.CompletedAt != nil {
			completedSessions++
		} else {
			activeSessions++
		}
		totalAnswers += len(sessionData.Answers)
	}

	return map[string]interface{}{
		"active_sessions":    activeSessions,
		"completed_sessions": completedSessions,
		"total_sessions":     len(sac.cache),
		"total_answers":      totalAnswers,
		"retention_minutes":  int(sac.retention.Minutes()),
	}
}

// cleanupRoutine runs in background to clean up expired sessions
func (sac *SessionAnswerCache) cleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute) // Run cleanup every 5 minutes
	defer ticker.Stop()

	for range ticker.C {
		sac.cleanup()
	}
}

// cleanup removes expired sessions from cache
func (sac *SessionAnswerCache) cleanup() {
	sac.mutex.Lock()
	defer sac.mutex.Unlock()

	now := time.Now()
	toDelete := make([]string, 0)

	for sessionID, sessionData := range sac.cache {
		// Remove if session completed and retention period expired
		if sessionData.CompletedAt != nil {
			if now.Sub(*sessionData.CompletedAt) > sac.retention {
				toDelete = append(toDelete, sessionID)
			}
		} else {
			// Remove inactive sessions older than 2 hours
			if now.Sub(sessionData.LastAccess) > 2*time.Hour {
				toDelete = append(toDelete, sessionID)
			}
		}
	}

	for _, sessionID := range toDelete {
		delete(sac.cache, sessionID)
	}
}

// ConvertQuizAnswerToCached converts a QuizAnswer to CachedAnswer
func ConvertQuizAnswerToCached(answer *QuizAnswer, questionType, bloomLevel string) CachedAnswer {
	return CachedAnswer{
		QuestionID:       answer.QuestionID,
		UserAnswer:       answer.UserAnswer,
		IsCorrect:        answer.IsCorrect,
		PointsEarned:     answer.PointsEarned,
		TimeSpentSeconds: answer.TimeSpentSeconds,
		AnsweredAt:       answer.AnsweredAt,
		QuestionType:     questionType,
		BloomLevel:       bloomLevel,
	}
}

// ToCachedAnswerResponse converts CachedAnswer to QuizAnswer format for API compatibility
func (ca *CachedAnswer) ToCachedAnswerResponse() QuizAnswer {
	return QuizAnswer{
		SessionID:        "", // Will be set by handler
		QuestionID:       ca.QuestionID,
		UserAnswer:       ca.UserAnswer,
		IsCorrect:        ca.IsCorrect,
		PointsEarned:     ca.PointsEarned,
		TimeSpentSeconds: ca.TimeSpentSeconds,
		AnsweredAt:       ca.AnsweredAt,
	}
}


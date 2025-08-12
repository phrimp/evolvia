package models

import (
	"fmt"
	"sync"
	"time"
)

// SessionQuestionState tracks the current question state for a session
type SessionQuestionState struct {
	SessionID         string     `json:"session_id"`
	CurrentQuestionID string     `json:"current_question_id"`
	QuestionStarted   time.Time  `json:"question_started"`
	IsAnswered        bool       `json:"is_answered"`
	AnswerSubmitted   *time.Time `json:"answer_submitted,omitempty"`
	QuestionType      string     `json:"question_type"`
	Stage             string     `json:"stage"`
}

// SessionQuestionStateCache manages current question states for active sessions
type SessionQuestionStateCache struct {
	states    map[string]*SessionQuestionState
	mutex     sync.RWMutex
	retention time.Duration
}

// NewSessionQuestionStateCache creates a new question state cache
func NewSessionQuestionStateCache(retention time.Duration) *SessionQuestionStateCache {
	if retention == 0 {
		retention = 2 * time.Hour // Default: 2 hours for inactive sessions
	}

	cache := &SessionQuestionStateCache{
		states:    make(map[string]*SessionQuestionState),
		retention: retention,
	}

	// Start background cleanup routine
	go cache.cleanupRoutine()

	return cache
}

// SetCurrentQuestion sets the current question for a session
// Returns error if session already has an active (unanswered) question
func (sqsc *SessionQuestionStateCache) SetCurrentQuestion(sessionID, questionID, questionType, stage string) error {
	sqsc.mutex.Lock()
	defer sqsc.mutex.Unlock()

	// Check if session already has an active question
	if existing, exists := sqsc.states[sessionID]; exists && !existing.IsAnswered {
		return fmt.Errorf("session %s already has active question %s (started %v), must answer before getting next question", 
			sessionID, existing.CurrentQuestionID, existing.QuestionStarted.Format("15:04:05"))
	}

	sqsc.states[sessionID] = &SessionQuestionState{
		SessionID:         sessionID,
		CurrentQuestionID: questionID,
		QuestionStarted:   time.Now(),
		IsAnswered:        false,
		QuestionType:      questionType,
		Stage:             stage,
	}
	
	return nil
}

// GetCurrentQuestion retrieves the current question state for a session
func (sqsc *SessionQuestionStateCache) GetCurrentQuestion(sessionID string) *SessionQuestionState {
	sqsc.mutex.RLock()
	defer sqsc.mutex.RUnlock()

	if state, exists := sqsc.states[sessionID]; exists {
		// Return a copy to prevent concurrent modification
		stateCopy := *state
		return &stateCopy
	}
	return nil
}

// MarkQuestionAnswered marks the current question as answered
func (sqsc *SessionQuestionStateCache) MarkQuestionAnswered(sessionID string) error {
	sqsc.mutex.Lock()
	defer sqsc.mutex.Unlock()

	state, exists := sqsc.states[sessionID]
	if !exists {
		return fmt.Errorf("no active question found for session %s", sessionID)
	}

	if state.IsAnswered {
		return fmt.Errorf("question already answered for session %s", sessionID)
	}

	now := time.Now()
	state.IsAnswered = true
	state.AnswerSubmitted = &now

	return nil
}

// ValidateAnswerForCurrentQuestion validates that the answer corresponds to the current question
func (sqsc *SessionQuestionStateCache) ValidateAnswerForCurrentQuestion(sessionID, questionID string) error {
	sqsc.mutex.RLock()
	defer sqsc.mutex.RUnlock()

	state, exists := sqsc.states[sessionID]
	if !exists {
		return fmt.Errorf("no active question for session %s", sessionID)
	}

	if state.CurrentQuestionID != questionID {
		return fmt.Errorf("answer question ID %s doesn't match current question %s for session %s",
			questionID, state.CurrentQuestionID, sessionID)
	}

	if state.IsAnswered {
		return fmt.Errorf("current question %s already answered for session %s",
			questionID, sessionID)
	}

	return nil
}

// ClearCurrentQuestion removes the current question state (e.g., when moving to next question)
func (sqsc *SessionQuestionStateCache) ClearCurrentQuestion(sessionID string) {
	sqsc.mutex.Lock()
	defer sqsc.mutex.Unlock()

	delete(sqsc.states, sessionID)
}

// GetQuestionStartTime returns when the current question was started
func (sqsc *SessionQuestionStateCache) GetQuestionStartTime(sessionID string) *time.Time {
	sqsc.mutex.RLock()
	defer sqsc.mutex.RUnlock()

	if state, exists := sqsc.states[sessionID]; exists {
		return &state.QuestionStarted
	}
	return nil
}

// GetActiveSessionsCount returns the number of sessions with active questions
func (sqsc *SessionQuestionStateCache) GetActiveSessionsCount() int {
	sqsc.mutex.RLock()
	defer sqsc.mutex.RUnlock()

	return len(sqsc.states)
}

// GetCacheStats returns statistics about the question state cache
func (sqsc *SessionQuestionStateCache) GetCacheStats() map[string]interface{} {
	sqsc.mutex.RLock()
	defer sqsc.mutex.RUnlock()

	activeQuestions := 0
	answeredQuestions := 0

	for _, state := range sqsc.states {
		if state.IsAnswered {
			answeredQuestions++
		} else {
			activeQuestions++
		}
	}

	return map[string]interface{}{
		"total_sessions":     len(sqsc.states),
		"active_questions":   activeQuestions,
		"answered_questions": answeredQuestions,
		"retention_hours":    int(sqsc.retention.Hours()),
	}
}

// cleanupRoutine runs in background to clean up old question states
func (sqsc *SessionQuestionStateCache) cleanupRoutine() {
	ticker := time.NewTicker(10 * time.Minute) // Run cleanup every 10 minutes
	defer ticker.Stop()

	for range ticker.C {
		sqsc.cleanup()
	}
}

// cleanup removes expired question states from cache
func (sqsc *SessionQuestionStateCache) cleanup() {
	sqsc.mutex.Lock()
	defer sqsc.mutex.Unlock()

	now := time.Now()
	toDelete := make([]string, 0)

	for sessionID, state := range sqsc.states {
		// Remove answered questions after 1 hour
		if state.IsAnswered && state.AnswerSubmitted != nil {
			if now.Sub(*state.AnswerSubmitted) > time.Hour {
				toDelete = append(toDelete, sessionID)
			}
		} else {
			// Remove unanswered questions older than retention period
			if now.Sub(state.QuestionStarted) > sqsc.retention {
				toDelete = append(toDelete, sessionID)
			}
		}
	}

	for _, sessionID := range toDelete {
		delete(sqsc.states, sessionID)
	}

	if len(toDelete) > 0 {
		fmt.Printf("Question state cache cleanup: removed %d expired states\n", len(toDelete))
	}
}


package timeout

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SessionTimeoutManager manages session timeouts with channel-based countdown timers
type SessionTimeoutManager struct {
	timers        map[string]*SessionTimer
	mutex         sync.RWMutex
	onTimeout     func(sessionID string)
	defaultConfig *TimeoutConfig
}

// SessionTimer manages individual session timeout
type SessionTimer struct {
	SessionID  string
	StartTime  time.Time
	Duration   time.Duration
	CancelFunc context.CancelFunc
	IsActive   bool
	mutex      sync.RWMutex
}

// TimeoutConfig defines timeout configuration
type TimeoutConfig struct {
	SessionTimeoutMinutes    int `json:"session_timeout_minutes"`
	QuestionTimeoutMinutes   int `json:"question_timeout_minutes"`
	FirstQuestionTimeout     int `json:"first_question_timeout"` // Seconds to get first question
	InactivityTimeoutMinutes int `json:"inactivity_timeout_minutes"`
}

// DefaultTimeoutConfig returns default timeout configuration
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		SessionTimeoutMinutes:    60,  // 1 hour total session timeout
		QuestionTimeoutMinutes:   30,  // 30 minutes per question max
		FirstQuestionTimeout:     300, // 5 minutes to get first question
		InactivityTimeoutMinutes: 15,  // 15 minutes inactivity timeout
	}
}

// NewSessionTimeoutManager creates a new session timeout manager
func NewSessionTimeoutManager(onTimeout func(string), config *TimeoutConfig) *SessionTimeoutManager {
	if config == nil {
		config = DefaultTimeoutConfig()
	}

	return &SessionTimeoutManager{
		timers:        make(map[string]*SessionTimer),
		onTimeout:     onTimeout,
		defaultConfig: config,
	}
}

// StartSessionTimeout starts main session countdown timer (called after first question is requested)
func (stm *SessionTimeoutManager) StartSessionTimeout(sessionID string, duration time.Duration) {
	stm.mutex.Lock()
	defer stm.mutex.Unlock()

	// Cancel existing timer if present
	if existing, exists := stm.timers[sessionID]; exists {
		existing.Cancel()
	}

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())

	timer := &SessionTimer{
		SessionID:  sessionID,
		StartTime:  time.Now(),
		Duration:   duration,
		CancelFunc: cancel,
		IsActive:   true,
	}

	stm.timers[sessionID] = timer

	// Start goroutine with channel-based countdown
	go stm.countdownTimer(ctx, sessionID, duration)

	fmt.Printf("Session timeout started for %s (duration: %v)\n", sessionID, duration)
}

// StartFirstQuestionTimeout starts countdown for first question delivery
func (stm *SessionTimeoutManager) StartFirstQuestionTimeout(sessionID string) {
	duration := time.Duration(stm.defaultConfig.FirstQuestionTimeout) * time.Second

	stm.mutex.Lock()
	defer stm.mutex.Unlock()

	// Cancel existing timer if present
	if existing, exists := stm.timers[sessionID]; exists {
		existing.Cancel()
	}

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())

	timer := &SessionTimer{
		SessionID:  sessionID,
		StartTime:  time.Now(),
		Duration:   duration,
		CancelFunc: cancel,
		IsActive:   true,
	}

	stm.timers[sessionID] = timer

	// Start goroutine with channel-based countdown for first question
	go stm.firstQuestionCountdown(ctx, sessionID, duration)

	fmt.Printf("First question timeout started for %s (duration: %v)\n", sessionID, duration)
}

// countdownTimer implements channel-based countdown timer
func (stm *SessionTimeoutManager) countdownTimer(ctx context.Context, sessionID string, duration time.Duration) {
	ticker := time.NewTicker(time.Minute) // Check every minute
	defer ticker.Stop()

	deadline := time.Now().Add(duration)

	for {
		select {
		case <-ctx.Done():
			// Timer cancelled
			fmt.Printf("Session timeout cancelled for %s\n", sessionID)
			return

		case <-ticker.C:
			remaining := time.Until(deadline)
			if remaining <= 0 {
				// Session timed out
				fmt.Printf("Session %s timed out after %v\n", sessionID, duration)
				stm.handleTimeout(sessionID, "session_timeout")
				return
			}

			// Log progress every 10 minutes
			if int(remaining.Minutes())%10 == 0 {
				fmt.Printf("Session %s: %v remaining\n", sessionID, remaining.Round(time.Minute))
			}

		case <-time.After(duration):
			// Final timeout
			fmt.Printf("Session %s timed out after %v\n", sessionID, duration)
			stm.handleTimeout(sessionID, "session_timeout")
			return
		}
	}
}

// firstQuestionCountdown implements countdown for first question delivery
func (stm *SessionTimeoutManager) firstQuestionCountdown(ctx context.Context, sessionID string, duration time.Duration) {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	deadline := time.Now().Add(duration)

	for {
		select {
		case <-ctx.Done():
			// Timer cancelled (first question was requested)
			fmt.Printf("First question timeout cancelled for %s (question requested)\n", sessionID)
			return

		case <-ticker.C:
			remaining := time.Until(deadline)
			if remaining <= 0 {
				// First question timeout
				fmt.Printf("Session %s: first question timeout after %v\n", sessionID, duration)
				stm.handleTimeout(sessionID, "first_question_timeout")
				return
			}

			// Log progress every minute
			if int(remaining.Seconds())%60 == 0 {
				fmt.Printf("Session %s: %v until first question timeout\n", sessionID, remaining.Round(time.Second))
			}

		case <-time.After(duration):
			// Final timeout
			fmt.Printf("Session %s: first question timeout after %v\n", sessionID, duration)
			stm.handleTimeout(sessionID, "first_question_timeout")
			return
		}
	}
}

// CancelTimeout cancels the timeout for a session (e.g., when first question is requested)
func (stm *SessionTimeoutManager) CancelTimeout(sessionID string) {
	stm.mutex.Lock()
	defer stm.mutex.Unlock()

	if timer, exists := stm.timers[sessionID]; exists {
		timer.Cancel()
		delete(stm.timers, sessionID)
		fmt.Printf("Timeout cancelled for session %s\n", sessionID)
	}
}

// ExtendTimeout extends session timeout (e.g., when user is active)
func (stm *SessionTimeoutManager) ExtendTimeout(sessionID string, additionalTime time.Duration) {
	stm.mutex.Lock()
	defer stm.mutex.Unlock()

	if timer, exists := stm.timers[sessionID]; exists {
		timer.Cancel()
		// Start new timer with extended duration
		newDuration := timer.Duration + additionalTime
		stm.mutex.Unlock() // Unlock before recursive call
		stm.StartSessionTimeout(sessionID, newDuration)
		stm.mutex.Lock() // Re-lock for defer
		fmt.Printf("Session %s timeout extended by %v (new duration: %v)\n", sessionID, additionalTime, newDuration)
	}
}

// GetTimeoutInfo returns timeout information for a session
func (stm *SessionTimeoutManager) GetTimeoutInfo(sessionID string) *TimeoutInfo {
	stm.mutex.RLock()
	defer stm.mutex.RUnlock()

	timer, exists := stm.timers[sessionID]
	if !exists || !timer.IsActive {
		return &TimeoutInfo{
			SessionID: sessionID,
			IsActive:  false,
		}
	}

	elapsed := time.Since(timer.StartTime)
	remaining := timer.Duration - elapsed

	return &TimeoutInfo{
		SessionID:     sessionID,
		IsActive:      true,
		StartTime:     timer.StartTime,
		Duration:      timer.Duration,
		ElapsedTime:   elapsed,
		RemainingTime: remaining,
		TimeoutAt:     timer.StartTime.Add(timer.Duration),
		IsExpired:     remaining <= 0,
	}
}

// TimeoutInfo provides timeout status information
type TimeoutInfo struct {
	SessionID     string        `json:"session_id"`
	IsActive      bool          `json:"is_active"`
	StartTime     time.Time     `json:"start_time,omitempty"`
	Duration      time.Duration `json:"duration,omitempty"`
	ElapsedTime   time.Duration `json:"elapsed_time,omitempty"`
	RemainingTime time.Duration `json:"remaining_time,omitempty"`
	TimeoutAt     time.Time     `json:"timeout_at,omitempty"`
	IsExpired     bool          `json:"is_expired"`
}

// handleTimeout handles timeout events
func (stm *SessionTimeoutManager) handleTimeout(sessionID string, timeoutType string) {
	stm.mutex.Lock()
	defer stm.mutex.Unlock()

	// Mark timer as inactive
	if timer, exists := stm.timers[sessionID]; exists {
		timer.mutex.Lock()
		timer.IsActive = false
		timer.mutex.Unlock()
	}

	fmt.Printf("Timeout handled for session %s (type: %s)\n", sessionID, timeoutType)

	// Call callback if provided
	if stm.onTimeout != nil {
		go stm.onTimeout(sessionID) // Non-blocking callback
	}

	// Clean up timer
	delete(stm.timers, sessionID)
}

// Cancel cancels the timer
func (st *SessionTimer) Cancel() {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	if st.CancelFunc != nil {
		st.CancelFunc()
		st.IsActive = false
	}
}

// Cleanup removes inactive timers (should be called periodically)
func (stm *SessionTimeoutManager) Cleanup() {
	stm.mutex.Lock()
	defer stm.mutex.Unlock()

	for sessionID, timer := range stm.timers {
		timer.mutex.RLock()
		isActive := timer.IsActive
		timer.mutex.RUnlock()

		if !isActive {
			delete(stm.timers, sessionID)
		}
	}

	fmt.Printf("Timeout manager cleanup completed. Active timers: %d\n", len(stm.timers))
}

// GetActiveTimerCount returns the number of active timers
func (stm *SessionTimeoutManager) GetActiveTimerCount() int {
	stm.mutex.RLock()
	defer stm.mutex.RUnlock()
	return len(stm.timers)
}


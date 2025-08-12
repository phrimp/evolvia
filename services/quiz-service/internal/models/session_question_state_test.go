package models

import (
	"testing"
	"time"
)

func TestSessionQuestionStateCache_Basic(t *testing.T) {
	cache := NewSessionQuestionStateCache(1 * time.Hour)
	
	sessionID := "test-session-123"
	questionID := "test-question-456"
	questionType := "multiple_choice"
	stage := "medium"
	
	// Test setting current question
	err := cache.SetCurrentQuestion(sessionID, questionID, questionType, stage)
	if err != nil {
		t.Fatalf("Failed to set current question: %v", err)
	}
	
	// Test getting current question
	state := cache.GetCurrentQuestion(sessionID)
	if state == nil {
		t.Fatal("Expected question state, got nil")
	}
	
	if state.SessionID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, state.SessionID)
	}
	
	if state.CurrentQuestionID != questionID {
		t.Errorf("Expected question ID %s, got %s", questionID, state.CurrentQuestionID)
	}
	
	if state.IsAnswered {
		t.Error("Question should not be marked as answered initially")
	}
}

func TestSessionQuestionStateCache_ValidationFlow(t *testing.T) {
	cache := NewSessionQuestionStateCache(1 * time.Hour)
	
	sessionID := "test-session-123"
	questionID := "test-question-456"
	wrongQuestionID := "wrong-question-789"
	questionType := "single_choice"
	stage := "easy"
	
	// Test validation without setting question - should fail
	err := cache.ValidateAnswerForCurrentQuestion(sessionID, questionID)
	if err == nil {
		t.Error("Expected validation to fail when no active question")
	}
	
	// Set current question
	err = cache.SetCurrentQuestion(sessionID, questionID, questionType, stage)
	if err != nil {
		t.Fatalf("Failed to set current question: %v", err)
	}
	
	// Test validation with wrong question ID - should fail
	err = cache.ValidateAnswerForCurrentQuestion(sessionID, wrongQuestionID)
	if err == nil {
		t.Error("Expected validation to fail with wrong question ID")
	}
	
	// Test validation with correct question ID - should succeed
	err = cache.ValidateAnswerForCurrentQuestion(sessionID, questionID)
	if err != nil {
		t.Errorf("Expected validation to succeed, got error: %v", err)
	}
	
	// Mark question as answered
	err = cache.MarkQuestionAnswered(sessionID)
	if err != nil {
		t.Errorf("Failed to mark question as answered: %v", err)
	}
	
	// Test validation after answer - should fail
	err = cache.ValidateAnswerForCurrentQuestion(sessionID, questionID)
	if err == nil {
		t.Error("Expected validation to fail after question is answered")
	}
	
	// Test double answering - should fail
	err = cache.MarkQuestionAnswered(sessionID)
	if err == nil {
		t.Error("Expected marking as answered to fail when already answered")
	}
}

func TestSessionQuestionStateCache_ClearQuestion(t *testing.T) {
	cache := NewSessionQuestionStateCache(1 * time.Hour)
	
	sessionID := "test-session-123"
	questionID := "test-question-456"
	questionType := "true_false"
	stage := "hard"
	
	// Set and verify question
	err := cache.SetCurrentQuestion(sessionID, questionID, questionType, stage)
	if err != nil {
		t.Fatalf("Failed to set current question: %v", err)
	}
	
	state := cache.GetCurrentQuestion(sessionID)
	if state == nil {
		t.Fatal("Expected question state, got nil")
	}
	
	// Clear question
	cache.ClearCurrentQuestion(sessionID)
	
	// Verify question is cleared
	state = cache.GetCurrentQuestion(sessionID)
	if state != nil {
		t.Error("Expected question state to be cleared")
	}
}

func TestSessionQuestionStateCache_Stats(t *testing.T) {
	cache := NewSessionQuestionStateCache(1 * time.Hour)
	
	// Test empty cache stats
	stats := cache.GetCacheStats()
	if stats["total_sessions"].(int) != 0 {
		t.Error("Expected 0 sessions in empty cache")
	}
	
	// Add some questions
	err := cache.SetCurrentQuestion("session1", "q1", "multiple_choice", "easy")
	if err != nil {
		t.Fatalf("Failed to set current question for session1: %v", err)
	}
	err = cache.SetCurrentQuestion("session2", "q2", "single_choice", "medium")
	if err != nil {
		t.Fatalf("Failed to set current question for session2: %v", err)
	}
	
	// Mark one as answered
	cache.MarkQuestionAnswered("session1")
	
	// Check updated stats
	stats = cache.GetCacheStats()
	if stats["total_sessions"].(int) != 2 {
		t.Errorf("Expected 2 sessions, got %d", stats["total_sessions"])
	}
	if stats["active_questions"].(int) != 1 {
		t.Errorf("Expected 1 active question, got %d", stats["active_questions"])
	}
	if stats["answered_questions"].(int) != 1 {
		t.Errorf("Expected 1 answered question, got %d", stats["answered_questions"])
	}
}

func TestSessionQuestionStateCache_PreventQuestionHoarding(t *testing.T) {
	cache := NewSessionQuestionStateCache(1 * time.Hour)
	
	sessionID := "test-session-123"
	questionID1 := "question-1"
	questionID2 := "question-2"
	questionType := "multiple_choice"
	stage := "medium"
	
	// Set first question - should succeed
	err := cache.SetCurrentQuestion(sessionID, questionID1, questionType, stage)
	if err != nil {
		t.Fatalf("Failed to set first question: %v", err)
	}
	
	// Try to set second question without answering first - should fail
	err = cache.SetCurrentQuestion(sessionID, questionID2, questionType, stage)
	if err == nil {
		t.Error("Expected error when trying to set second question without answering first")
	}
	
	// Verify first question is still active
	state := cache.GetCurrentQuestion(sessionID)
	if state == nil {
		t.Fatal("Expected question state to still exist")
	}
	if state.CurrentQuestionID != questionID1 {
		t.Errorf("Expected current question to be %s, got %s", questionID1, state.CurrentQuestionID)
	}
	
	// Answer the first question
	err = cache.MarkQuestionAnswered(sessionID)
	if err != nil {
		t.Fatalf("Failed to mark first question as answered: %v", err)
	}
	
	// Now setting second question should succeed
	err = cache.SetCurrentQuestion(sessionID, questionID2, questionType, stage)
	if err != nil {
		t.Errorf("Expected to be able to set second question after answering first, got error: %v", err)
	}
	
	// Verify second question is now active
	state = cache.GetCurrentQuestion(sessionID)
	if state == nil {
		t.Fatal("Expected question state for second question")
	}
	if state.CurrentQuestionID != questionID2 {
		t.Errorf("Expected current question to be %s, got %s", questionID2, state.CurrentQuestionID)
	}
	if state.IsAnswered {
		t.Error("Second question should not be marked as answered initially")
	}
}

func TestSessionQuestionStateCache_MultipleSessionsIndependent(t *testing.T) {
	cache := NewSessionQuestionStateCache(1 * time.Hour)
	
	session1 := "session-1"
	session2 := "session-2" 
	questionID1 := "question-1"
	questionID2 := "question-2"
	questionType := "single_choice"
	stage := "easy"
	
	// Set questions for both sessions - should succeed independently
	err := cache.SetCurrentQuestion(session1, questionID1, questionType, stage)
	if err != nil {
		t.Fatalf("Failed to set question for session1: %v", err)
	}
	
	err = cache.SetCurrentQuestion(session2, questionID2, questionType, stage)
	if err != nil {
		t.Fatalf("Failed to set question for session2: %v", err)
	}
	
	// Verify both sessions have their respective questions
	state1 := cache.GetCurrentQuestion(session1)
	if state1 == nil || state1.CurrentQuestionID != questionID1 {
		t.Errorf("Session1 should have question %s", questionID1)
	}
	
	state2 := cache.GetCurrentQuestion(session2)
	if state2 == nil || state2.CurrentQuestionID != questionID2 {
		t.Errorf("Session2 should have question %s", questionID2)
	}
	
	// Try to set new question for session1 without answering - should fail
	err = cache.SetCurrentQuestion(session1, "new-question", questionType, stage)
	if err == nil {
		t.Error("Expected error when trying to set new question for session1 without answering")
	}
	
	// Session2 should not be affected - try to set new question should also fail
	err = cache.SetCurrentQuestion(session2, "another-question", questionType, stage)
	if err == nil {
		t.Error("Expected error when trying to set new question for session2 without answering")
	}
}
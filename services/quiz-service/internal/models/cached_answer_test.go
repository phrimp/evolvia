package models

import (
	"testing"
	"time"
)

func TestSessionAnswerCache(t *testing.T) {
	// Create a new cache with 1 minute retention for testing
	cache := NewSessionAnswerCache(1 * time.Minute)
	
	sessionID := "test-session-123"
	
	// Test adding an answer
	answer := CachedAnswer{
		QuestionID:       "q1",
		UserAnswer:       "A",
		IsCorrect:        true,
		PointsEarned:     10.0,
		TimeSpentSeconds: 30,
		AnsweredAt:       time.Now(),
		QuestionType:     "single_choice",
		BloomLevel:       "understand",
	}
	
	cache.AddAnswer(sessionID, answer)
	
	// Test retrieving answers
	answers, exists := cache.GetAnswers(sessionID)
	if !exists {
		t.Fatal("Expected answers to exist in cache")
	}
	
	if len(answers) != 1 {
		t.Fatalf("Expected 1 answer, got %d", len(answers))
	}
	
	if answers[0].QuestionID != "q1" {
		t.Fatalf("Expected question ID 'q1', got '%s'", answers[0].QuestionID)
	}
	
	// Test answer count
	count := cache.GetAnswerCount(sessionID)
	if count != 1 {
		t.Fatalf("Expected answer count 1, got %d", count)
	}
	
	// Test cache stats
	stats := cache.GetCacheStats()
	if stats["active_sessions"] != 1 {
		t.Fatalf("Expected 1 active session, got %v", stats["active_sessions"])
	}
	
	if stats["total_answers"] != 1 {
		t.Fatalf("Expected 1 total answer, got %v", stats["total_answers"])
	}
	
	// Test marking session as completed
	cache.MarkSessionCompleted(sessionID)
	
	stats = cache.GetCacheStats()
	if stats["completed_sessions"] != 1 {
		t.Fatalf("Expected 1 completed session, got %v", stats["completed_sessions"])
	}
	
	// Test session removal
	cache.RemoveSession(sessionID)
	
	_, exists = cache.GetAnswers(sessionID)
	if exists {
		t.Fatal("Expected answers to be removed from cache")
	}
}

func TestConvertQuizAnswerToCached(t *testing.T) {
	quizAnswer := &QuizAnswer{
		SessionID:        "session1",
		QuestionID:       "q1",
		UserAnswer:       "B",
		IsCorrect:        false,
		PointsEarned:     0.0,
		TimeSpentSeconds: 45,
		AnsweredAt:       time.Now(),
	}
	
	cached := ConvertQuizAnswerToCached(quizAnswer, "multiple_choice", "apply")
	
	if cached.QuestionID != "q1" {
		t.Fatalf("Expected question ID 'q1', got '%s'", cached.QuestionID)
	}
	
	if cached.QuestionType != "multiple_choice" {
		t.Fatalf("Expected question type 'multiple_choice', got '%s'", cached.QuestionType)
	}
	
	if cached.BloomLevel != "apply" {
		t.Fatalf("Expected bloom level 'apply', got '%s'", cached.BloomLevel)
	}
	
	if cached.IsCorrect != false {
		t.Fatal("Expected is_correct to be false")
	}
}

func TestToCachedAnswerResponse(t *testing.T) {
	cached := &CachedAnswer{
		QuestionID:       "q1",
		UserAnswer:       "C",
		IsCorrect:        true,
		PointsEarned:     5.0,
		TimeSpentSeconds: 25,
		AnsweredAt:       time.Now(),
		QuestionType:     "single_choice",
		BloomLevel:       "remember",
	}
	
	response := cached.ToCachedAnswerResponse()
	
	if response.QuestionID != "q1" {
		t.Fatalf("Expected question ID 'q1', got '%s'", response.QuestionID)
	}
	
	if response.UserAnswer != "C" {
		t.Fatalf("Expected user answer 'C', got '%s'", response.UserAnswer)
	}
	
	if response.PointsEarned != 5.0 {
		t.Fatalf("Expected points earned 5.0, got %f", response.PointsEarned)
	}
}
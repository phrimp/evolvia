package models

import (
	"fmt"
	"testing"
)

func TestBloomScoreCalculation(t *testing.T) {
	testCases := []struct {
		bloomLevel      string
		expectedScore   int
		expectEasyScore int
		expectHardScore int
	}{
		{"remember", 10, 10, 15},
		{"understand", 15, 15, 22},
		{"apply", 20, 20, 30},
		{"analyze", 25, 25, 37},
		{"evaluate", 30, 30, 45},
		{"create", 35, 35, 52},
		{"invalid", 10, 10, 15}, // fallback to default
	}

	for _, tc := range testCases {
		t.Run(tc.bloomLevel, func(t *testing.T) {
			question := &Question{
				BloomLevel: tc.bloomLevel,
			}

			// Test base score calculation
			question.CalculateBloomScore()
			if question.BloomScore != tc.expectedScore {
				t.Errorf("Expected BloomScore %d, got %d", tc.expectedScore, question.BloomScore)
			}

			// Test stage score calculation
			question.CalculateBloomScoresByStage()

			if len(question.BloomScoresByStage) != 3 {
				t.Errorf("Expected 3 stage scores, got %d", len(question.BloomScoresByStage))
			}

			if question.BloomScoresByStage["easy"] != tc.expectEasyScore {
				t.Errorf("Expected easy score %d, got %d", tc.expectEasyScore, question.BloomScoresByStage["easy"])
			}

			if question.BloomScoresByStage["hard"] != tc.expectHardScore {
				t.Errorf("Expected hard score %d, got %d", tc.expectHardScore, question.BloomScoresByStage["hard"])
			}

			// Test GetScoreForStage method
			easyScore := question.GetScoreForStage("easy")
			if easyScore != tc.expectEasyScore {
				t.Errorf("GetScoreForStage('easy') expected %d, got %d", tc.expectEasyScore, easyScore)
			}

			hardScore := question.GetScoreForStage("hard")
			if hardScore != tc.expectHardScore {
				t.Errorf("GetScoreForStage('hard') expected %d, got %d", tc.expectHardScore, hardScore)
			}
		})
	}
}

func TestEnsureBloomScores(t *testing.T) {
	question := &Question{
		BloomLevel: "analyze",
	}

	// Initially should be zero
	if question.BloomScore != 0 {
		t.Errorf("Expected initial BloomScore to be 0, got %d", question.BloomScore)
	}

	// Call EnsureBloomScores
	question.EnsureBloomScores()

	// Should now be calculated
	if question.BloomScore != 25 {
		t.Errorf("Expected BloomScore to be 25 after EnsureBloomScores, got %d", question.BloomScore)
	}

	if len(question.BloomScoresByStage) != 3 {
		t.Errorf("Expected 3 stage scores after EnsureBloomScores, got %d", len(question.BloomScoresByStage))
	}
}

// Test question type validation and constants
func TestQuestionTypeValidation(t *testing.T) {
	testCases := []struct {
		questionType string
		expected     bool
	}{
		{"multiple_choice", true},
		{"true_false", true},
		{"single_choice", true},
		{"invalid_type", false},
		{"", false},
	}

	for _, tc := range testCases {
		t.Run(tc.questionType, func(t *testing.T) {
			result := IsValidQuestionType(tc.questionType)
			if result != tc.expected {
				t.Errorf("IsValidQuestionType('%s') = %v, expected %v", tc.questionType, result, tc.expected)
			}
		})
	}
}

func TestGetPrimaryQuestionTypes(t *testing.T) {
	types := GetPrimaryQuestionTypes()
	
	if len(types) != 3 {
		t.Errorf("Expected 3 primary question types, got %d", len(types))
	}
	
	expectedTypes := []QuestionType{
		QuestionTypeMultipleChoice,
		QuestionTypeTrueFalse,
		QuestionTypeSingleChoice,
	}
	
	for _, expectedType := range expectedTypes {
		found := false
		for _, actualType := range types {
			if actualType == expectedType {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected type '%s' not found in primary types", expectedType)
		}
	}
}

// Test factory methods for creating different question types
func TestNewTrueFalseQuestion(t *testing.T) {
	testCases := []struct {
		name          string
		content       string
		skillID       string
		bloomLevel    string
		correctAnswer bool
	}{
		{"True answer", "The sky is blue", "skill1", "remember", true},
		{"False answer", "Water boils at 50°C", "skill2", "understand", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			question := NewTrueFalseQuestion(tc.content, tc.skillID, tc.bloomLevel, tc.correctAnswer)
			
			// Validate basic properties
			if question.Content != tc.content {
				t.Errorf("Expected content '%s', got '%s'", tc.content, question.Content)
			}
			if question.Type != string(QuestionTypeTrueFalse) {
				t.Errorf("Expected type '%s', got '%s'", QuestionTypeTrueFalse, question.Type)
			}
			if question.SkillID != tc.skillID {
				t.Errorf("Expected skillID '%s', got '%s'", tc.skillID, question.SkillID)
			}
			if question.BloomLevel != tc.bloomLevel {
				t.Errorf("Expected bloomLevel '%s', got '%s'", tc.bloomLevel, question.BloomLevel)
			}
			
			// Validate options
			if len(question.Options) != 2 {
				t.Errorf("Expected 2 options for true/false question, got %d", len(question.Options))
			}
			
			trueOption := findOptionByID(question.Options, "true")
			falseOption := findOptionByID(question.Options, "false")
			
			if trueOption == nil {
				t.Error("Expected 'true' option not found")
			} else if trueOption.IsCorrect != tc.correctAnswer {
				t.Errorf("True option IsCorrect = %v, expected %v", trueOption.IsCorrect, tc.correctAnswer)
			}
			
			if falseOption == nil {
				t.Error("Expected 'false' option not found")
			} else if falseOption.IsCorrect != !tc.correctAnswer {
				t.Errorf("False option IsCorrect = %v, expected %v", falseOption.IsCorrect, !tc.correctAnswer)
			}
			
			// Validate correct answer string
			expectedAnswerStr := "false"
			if tc.correctAnswer {
				expectedAnswerStr = "true"
			}
			if question.CorrectAnswer != expectedAnswerStr {
				t.Errorf("Expected CorrectAnswer '%s', got '%s'", expectedAnswerStr, question.CorrectAnswer)
			}
			
			// Validate correct answer boolean
			if question.CorrectAnswerBoolean == nil {
				t.Error("Expected CorrectAnswerBoolean to be set")
			} else if *question.CorrectAnswerBoolean != tc.correctAnswer {
				t.Errorf("Expected CorrectAnswerBoolean %v, got %v", tc.correctAnswer, *question.CorrectAnswerBoolean)
			}
		})
	}
}

func TestNewSingleChoiceQuestion(t *testing.T) {
	options := []Option{
		{ID: "a", Text: "Option A"},
		{ID: "b", Text: "Option B"},
		{ID: "c", Text: "Option C"},
		{ID: "d", Text: "Option D"},
	}
	
	question := NewSingleChoiceQuestion("Test content", "skill1", "apply", options, "c")
	
	// Validate basic properties
	if question.Type != string(QuestionTypeSingleChoice) {
		t.Errorf("Expected type '%s', got '%s'", QuestionTypeSingleChoice, question.Type)
	}
	if question.CorrectAnswer != "c" {
		t.Errorf("Expected CorrectAnswer 'c', got '%s'", question.CorrectAnswer)
	}
	if len(question.Options) != 4 {
		t.Errorf("Expected 4 options, got %d", len(question.Options))
	}
	
	// Validate that only the correct option is marked as correct
	correctCount := 0
	for i, option := range question.Options {
		if option.ID == "c" && !option.IsCorrect {
			t.Errorf("Expected option 'c' to be marked correct, but it wasn't")
		} else if option.ID != "c" && option.IsCorrect {
			t.Errorf("Expected option '%s' to not be marked correct, but it was", option.ID)
		}
		
		if option.IsCorrect {
			correctCount++
		}
		
		// Verify original option data is preserved
		expectedText := fmt.Sprintf("Option %c", 'A'+i)
		if option.Text != expectedText {
			t.Errorf("Expected option text '%s', got '%s'", expectedText, option.Text)
		}
	}
	
	if correctCount != 1 {
		t.Errorf("Expected exactly 1 correct option, got %d", correctCount)
	}
}

func TestNewMultipleChoiceQuestion(t *testing.T) {
	options := []Option{
		{ID: "opt1", Text: "First option"},
		{ID: "opt2", Text: "Second option"},
		{ID: "opt3", Text: "Third option"},
	}
	
	question := NewMultipleChoiceQuestion("Test question", "skill2", "evaluate", options, "opt2")
	
	// Validate basic properties
	if question.Type != string(QuestionTypeMultipleChoice) {
		t.Errorf("Expected type '%s', got '%s'", QuestionTypeMultipleChoice, question.Type)
	}
	if question.CorrectAnswer != "opt2" {
		t.Errorf("Expected CorrectAnswer 'opt2', got '%s'", question.CorrectAnswer)
	}
	if len(question.Options) != 3 {
		t.Errorf("Expected 3 options, got %d", len(question.Options))
	}
	
	// Validate options are preserved exactly as provided
	for i, option := range question.Options {
		expectedOption := options[i]
		if option.ID != expectedOption.ID {
			t.Errorf("Expected option ID '%s', got '%s'", expectedOption.ID, option.ID)
		}
		if option.Text != expectedOption.Text {
			t.Errorf("Expected option text '%s', got '%s'", expectedOption.Text, option.Text)
		}
		// Multiple choice factory doesn't modify IsCorrect field
	}
}

// Test question validation methods
func TestQuestionValidation(t *testing.T) {
	t.Run("ValidateMultipleChoice", func(t *testing.T) {
		// Valid multiple choice question
		validQuestion := NewMultipleChoiceQuestion(
			"What is 2+2?",
			"math",
			"remember",
			[]Option{
				{ID: "a", Text: "3"},
				{ID: "b", Text: "4"},
				{ID: "c", Text: "5"},
			},
			"b",
		)
		
		if err := validQuestion.Validate(); err != nil {
			t.Errorf("Valid multiple choice question should not return error: %v", err)
		}
		
		// Invalid - too few options
		invalidQuestion1 := &Question{
			Content: "Test",
			Type:    string(QuestionTypeMultipleChoice),
			Options: []Option{{ID: "a", Text: "Only one"}},
			CorrectAnswer: "a",
		}
		
		if err := invalidQuestion1.Validate(); err == nil {
			t.Error("Multiple choice question with 1 option should return error")
		}
		
		// Invalid - correct answer not in options
		invalidQuestion2 := &Question{
			Content: "Test",
			Type:    string(QuestionTypeMultipleChoice),
			Options: []Option{
				{ID: "a", Text: "Option A"},
				{ID: "b", Text: "Option B"},
			},
			CorrectAnswer: "c",
		}
		
		if err := invalidQuestion2.Validate(); err == nil {
			t.Error("Multiple choice question with invalid correct answer should return error")
		}
	})
	
	t.Run("ValidateTrueFalse", func(t *testing.T) {
		// Valid true/false question
		validQuestion := NewTrueFalseQuestion("The Earth is round", "geography", "remember", true)
		
		if err := validQuestion.Validate(); err != nil {
			t.Errorf("Valid true/false question should not return error: %v", err)
		}
		
		// Invalid - wrong number of options
		invalidQuestion1 := &Question{
			Content: "Test",
			Type:    string(QuestionTypeTrueFalse),
			Options: []Option{{ID: "true", Text: "True"}}, // Missing false option
			CorrectAnswer: "true",
		}
		
		if err := invalidQuestion1.Validate(); err == nil {
			t.Error("True/false question with 1 option should return error")
		}
		
		// Invalid - missing required options
		invalidQuestion2 := &Question{
			Content: "Test",
			Type:    string(QuestionTypeTrueFalse),
			Options: []Option{
				{ID: "yes", Text: "Yes"},
				{ID: "no", Text: "No"},
			},
			CorrectAnswer: "yes",
		}
		
		if err := invalidQuestion2.Validate(); err == nil {
			t.Error("True/false question without 'true'/'false' options should return error")
		}
	})
	
	t.Run("ValidateSingleChoice", func(t *testing.T) {
		// Valid single choice question
		validQuestion := NewSingleChoiceQuestion(
			"Which is correct?",
			"skill1",
			"understand",
			[]Option{
				{ID: "a", Text: "Wrong"},
				{ID: "b", Text: "Correct"},
				{ID: "c", Text: "Also wrong"},
			},
			"b",
		)
		
		if err := validQuestion.Validate(); err != nil {
			t.Errorf("Valid single choice question should not return error: %v", err)
		}
		
		// Invalid - multiple correct options
		invalidQuestion := &Question{
			Content: "Test",
			Type:    string(QuestionTypeSingleChoice),
			Options: []Option{
				{ID: "a", Text: "First", IsCorrect: true},
				{ID: "b", Text: "Second", IsCorrect: true}, // This makes it invalid
			},
			CorrectAnswer: "a",
		}
		
		if err := invalidQuestion.Validate(); err == nil {
			t.Error("Single choice question with multiple correct options should return error")
		}
	})
}

// Test answer validation methods
func TestAnswerValidation(t *testing.T) {
	t.Run("TrueFalseAnswers", func(t *testing.T) {
		trueQuestion := NewTrueFalseQuestion("Test", "skill1", "remember", true)
		falseQuestion := NewTrueFalseQuestion("Test", "skill1", "remember", false)
		
		// Test string answers
		if !trueQuestion.IsCorrectAnswer("true") {
			t.Error("True question should accept 'true' as correct answer")
		}
		if trueQuestion.IsCorrectAnswer("false") {
			t.Error("True question should not accept 'false' as correct answer")
		}
		
		if !falseQuestion.IsCorrectAnswer("false") {
			t.Error("False question should accept 'false' as correct answer")
		}
		if falseQuestion.IsCorrectAnswer("true") {
			t.Error("False question should not accept 'true' as correct answer")
		}
		
		// Test boolean answers
		if !trueQuestion.IsCorrectBooleanAnswer(true) {
			t.Error("True question should accept boolean true as correct answer")
		}
		if trueQuestion.IsCorrectBooleanAnswer(false) {
			t.Error("True question should not accept boolean false as correct answer")
		}
		
		if !falseQuestion.IsCorrectBooleanAnswer(false) {
			t.Error("False question should accept boolean false as correct answer")
		}
		if falseQuestion.IsCorrectBooleanAnswer(true) {
			t.Error("False question should not accept boolean true as correct answer")
		}
		
		// Test boolean answers on non-true/false questions
		multipleChoiceQ := NewMultipleChoiceQuestion("Test", "skill1", "remember", 
			[]Option{{ID: "a", Text: "A"}}, "a")
		if multipleChoiceQ.IsCorrectBooleanAnswer(true) {
			t.Error("Multiple choice question should not accept boolean answers")
		}
	})
	
	t.Run("MultipleAndSingleChoiceAnswers", func(t *testing.T) {
		options := []Option{
			{ID: "a", Text: "Option A"},
			{ID: "b", Text: "Option B"},
			{ID: "c", Text: "Option C"},
		}
		
		multipleChoice := NewMultipleChoiceQuestion("Test", "skill1", "remember", options, "b")
		singleChoice := NewSingleChoiceQuestion("Test", "skill1", "remember", options, "c")
		
		// Test correct answers
		if !multipleChoice.IsCorrectAnswer("b") {
			t.Error("Multiple choice should accept correct answer 'b'")
		}
		if !singleChoice.IsCorrectAnswer("c") {
			t.Error("Single choice should accept correct answer 'c'")
		}
		
		// Test incorrect answers
		if multipleChoice.IsCorrectAnswer("a") {
			t.Error("Multiple choice should not accept incorrect answer 'a'")
		}
		if singleChoice.IsCorrectAnswer("a") {
			t.Error("Single choice should not accept incorrect answer 'a'")
		}
	})
}

// Test GetQuestionTypeInfo method
func TestGetQuestionTypeInfo(t *testing.T) {
	t.Run("TrueFalseInfo", func(t *testing.T) {
		question := NewTrueFalseQuestion("Test", "skill1", "remember", true)
		info := question.GetQuestionTypeInfo()
		
		if info["type"] != string(QuestionTypeTrueFalse) {
			t.Errorf("Expected type '%s', got '%v'", QuestionTypeTrueFalse, info["type"])
		}
		if info["options_count"] != 2 {
			t.Errorf("Expected options_count 2, got %v", info["options_count"])
		}
		if info["correct_answer"] != "true" {
			t.Errorf("Expected correct_answer 'true', got %v", info["correct_answer"])
		}
		if info["is_true_false"] != true {
			t.Errorf("Expected is_true_false true, got %v", info["is_true_false"])
		}
		if info["correct_boolean"] != true {
			t.Errorf("Expected correct_boolean true, got %v", info["correct_boolean"])
		}
	})
	
	t.Run("SingleChoiceInfo", func(t *testing.T) {
		options := []Option{
			{ID: "a", Text: "Option A"},
			{ID: "b", Text: "Option B"},
		}
		question := NewSingleChoiceQuestion("Test", "skill1", "remember", options, "b")
		info := question.GetQuestionTypeInfo()
		
		if info["type"] != string(QuestionTypeSingleChoice) {
			t.Errorf("Expected type '%s', got '%v'", QuestionTypeSingleChoice, info["type"])
		}
		if info["is_single_choice"] != true {
			t.Errorf("Expected is_single_choice true, got %v", info["is_single_choice"])
		}
		if info["correct_options_count"] != 1 {
			t.Errorf("Expected correct_options_count 1, got %v", info["correct_options_count"])
		}
	})
}

// Test GetCorrectOptions method
func TestGetCorrectOptions(t *testing.T) {
	t.Run("TrueFalseCorrectOptions", func(t *testing.T) {
		question := NewTrueFalseQuestion("Test", "skill1", "remember", true)
		correctOptions := question.GetCorrectOptions()
		
		if len(correctOptions) != 1 {
			t.Errorf("Expected 1 correct option, got %d", len(correctOptions))
		}
		
		if correctOptions[0].ID != "true" {
			t.Errorf("Expected correct option ID 'true', got '%s'", correctOptions[0].ID)
		}
	})
	
	t.Run("SingleChoiceCorrectOptions", func(t *testing.T) {
		options := []Option{
			{ID: "a", Text: "Wrong"},
			{ID: "b", Text: "Correct"},
			{ID: "c", Text: "Also wrong"},
		}
		question := NewSingleChoiceQuestion("Test", "skill1", "remember", options, "b")
		correctOptions := question.GetCorrectOptions()
		
		if len(correctOptions) != 1 {
			t.Errorf("Expected 1 correct option, got %d", len(correctOptions))
		}
		
		if correctOptions[0].ID != "b" {
			t.Errorf("Expected correct option ID 'b', got '%s'", correctOptions[0].ID)
		}
	})
}

// Helper function for tests
func findOptionByID(options []Option, id string) *Option {
	for i := range options {
		if options[i].ID == id {
			return &options[i]
		}
	}
	return nil
}

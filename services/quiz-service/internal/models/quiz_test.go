package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// QuizTestSuite defines the test suite for Quiz model
type QuizTestSuite struct {
	suite.Suite
	sampleQuiz        *Quiz
	sampleStageConfig StageConfig
}

func (suite *QuizTestSuite) SetupTest() {
	suite.sampleStageConfig = StageConfig{
		InitialQuestions:  5,
		PassingThreshold:  0.8,
		RecoveryQuestions: 3,
		RecoveryThreshold: 0.6,
		BasePoints:        100,
		RecoveryPoints:    50,
	}

	suite.sampleQuiz = &Quiz{
		ID:                   "quiz_123",
		Title:                "Advanced Mathematics",
		Description:          "A comprehensive quiz covering advanced mathematical concepts",
		SkillID:              "math_advanced_001",
		TotalDurationSeconds: 3600, // 1 hour
		MaxQuestions:         20,
		StageConfig: map[string]StageConfig{
			"remember":   suite.sampleStageConfig,
			"understand": suite.sampleStageConfig,
			"apply":      suite.sampleStageConfig,
			"analyze":    suite.sampleStageConfig,
			"evaluate":   suite.sampleStageConfig,
			"create":     suite.sampleStageConfig,
		},
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (suite *QuizTestSuite) TestQuizModel_Structure() {
	t := suite.T()

	// Test that all fields are properly set
	assert.Equal(t, "quiz_123", suite.sampleQuiz.ID)
	assert.Equal(t, "Advanced Mathematics", suite.sampleQuiz.Title)
	assert.Equal(t, "A comprehensive quiz covering advanced mathematical concepts", suite.sampleQuiz.Description)
	assert.Equal(t, "math_advanced_001", suite.sampleQuiz.SkillID)
	assert.Equal(t, 3600, suite.sampleQuiz.TotalDurationSeconds)
	assert.Equal(t, 20, suite.sampleQuiz.MaxQuestions)
	assert.Equal(t, "active", suite.sampleQuiz.Status)
	assert.NotZero(t, suite.sampleQuiz.CreatedAt)
	assert.NotZero(t, suite.sampleQuiz.UpdatedAt)
}

func (suite *QuizTestSuite) TestStageConfig_Structure() {
	t := suite.T()
	config := suite.sampleStageConfig

	assert.Equal(t, 5, config.InitialQuestions)
	assert.Equal(t, 0.8, config.PassingThreshold)
	assert.Equal(t, 3, config.RecoveryQuestions)
	assert.Equal(t, 0.6, config.RecoveryThreshold)
	assert.Equal(t, 100, config.BasePoints)
	assert.Equal(t, 50, config.RecoveryPoints)
}

func (suite *QuizTestSuite) TestQuiz_StageConfigMapping() {
	t := suite.T()

	// Test that all Bloom's taxonomy stages are configured
	bloomStages := []string{"remember", "understand", "apply", "analyze", "evaluate", "create"}

	for _, stage := range bloomStages {
		config, exists := suite.sampleQuiz.StageConfig[stage]
		assert.True(t, exists, "Stage %s should exist in configuration", stage)
		assert.Equal(t, suite.sampleStageConfig, config)
	}
}

func (suite *QuizTestSuite) TestQuiz_JSONSerialization() {
	t := suite.T()

	// Test JSON marshaling
	jsonData, err := json.Marshal(suite.sampleQuiz)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Test JSON unmarshaling
	var deserializedQuiz Quiz
	err = json.Unmarshal(jsonData, &deserializedQuiz)
	assert.NoError(t, err)

	// Compare key fields (excluding time fields due to precision differences)
	assert.Equal(t, suite.sampleQuiz.ID, deserializedQuiz.ID)
	assert.Equal(t, suite.sampleQuiz.Title, deserializedQuiz.Title)
	assert.Equal(t, suite.sampleQuiz.Description, deserializedQuiz.Description)
	assert.Equal(t, suite.sampleQuiz.SkillID, deserializedQuiz.SkillID)
	assert.Equal(t, suite.sampleQuiz.TotalDurationSeconds, deserializedQuiz.TotalDurationSeconds)
	assert.Equal(t, suite.sampleQuiz.MaxQuestions, deserializedQuiz.MaxQuestions)
	assert.Equal(t, suite.sampleQuiz.Status, deserializedQuiz.Status)
}

func (suite *QuizTestSuite) TestStageConfig_JSONSerialization() {
	t := suite.T()

	// Test JSON marshaling of StageConfig
	jsonData, err := json.Marshal(suite.sampleStageConfig)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Test JSON unmarshaling
	var deserializedConfig StageConfig
	err = json.Unmarshal(jsonData, &deserializedConfig)
	assert.NoError(t, err)
	assert.Equal(t, suite.sampleStageConfig, deserializedConfig)
}

func (suite *QuizTestSuite) TestQuiz_ValidationScenarios() {
	t := suite.T()

	testCases := []struct {
		name        string
		quiz        Quiz
		expectValid bool
		description string
	}{
		{
			name:        "ValidQuiz",
			quiz:        *suite.sampleQuiz,
			expectValid: true,
			description: "A properly configured quiz should be valid",
		},
		{
			name: "QuizWithoutTitle",
			quiz: Quiz{
				ID:                   "quiz_no_title",
				Description:          "Quiz without title",
				SkillID:              "skill_001",
				TotalDurationSeconds: 1800,
				MaxQuestions:         10,
				Status:               "draft",
			},
			expectValid: false,
			description: "Quiz without title should be invalid",
		},
		{
			name: "QuizWithNegativeDuration",
			quiz: Quiz{
				ID:                   "quiz_negative",
				Title:                "Invalid Quiz",
				Description:          "Quiz with negative duration",
				SkillID:              "skill_001",
				TotalDurationSeconds: -100,
				MaxQuestions:         10,
				Status:               "draft",
			},
			expectValid: false,
			description: "Quiz with negative duration should be invalid",
		},
		{
			name: "QuizWithZeroMaxQuestions",
			quiz: Quiz{
				ID:                   "quiz_zero_questions",
				Title:                "Invalid Quiz",
				Description:          "Quiz with zero max questions",
				SkillID:              "skill_001",
				TotalDurationSeconds: 1800,
				MaxQuestions:         0,
				Status:               "draft",
			},
			expectValid: false,
			description: "Quiz with zero max questions should be invalid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isValid := suite.validateQuiz(&tc.quiz)
			assert.Equal(t, tc.expectValid, isValid, tc.description)
		})
	}
}

func (suite *QuizTestSuite) TestStageConfig_ValidationScenarios() {
	t := suite.T()

	testCases := []struct {
		name        string
		config      StageConfig
		expectValid bool
		description string
	}{
		{
			name:        "ValidStageConfig",
			config:      suite.sampleStageConfig,
			expectValid: true,
			description: "A properly configured stage should be valid",
		},
		{
			name: "InvalidThresholds",
			config: StageConfig{
				InitialQuestions:  5,
				PassingThreshold:  1.5, // Invalid: > 1.0
				RecoveryQuestions: 3,
				RecoveryThreshold: -0.1, // Invalid: < 0.0
				BasePoints:        100,
				RecoveryPoints:    50,
			},
			expectValid: false,
			description: "Stage config with invalid thresholds should be invalid",
		},
		{
			name: "NegativeQuestions",
			config: StageConfig{
				InitialQuestions:  -1, // Invalid
				PassingThreshold:  0.8,
				RecoveryQuestions: -2, // Invalid
				RecoveryThreshold: 0.6,
				BasePoints:        100,
				RecoveryPoints:    50,
			},
			expectValid: false,
			description: "Stage config with negative questions should be invalid",
		},
		{
			name: "NegativePoints",
			config: StageConfig{
				InitialQuestions:  5,
				PassingThreshold:  0.8,
				RecoveryQuestions: 3,
				RecoveryThreshold: 0.6,
				BasePoints:        -10, // Invalid
				RecoveryPoints:    -5,  // Invalid
			},
			expectValid: false,
			description: "Stage config with negative points should be invalid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isValid := suite.validateStageConfig(&tc.config)
			assert.Equal(t, tc.expectValid, isValid, tc.description)
		})
	}
}

func (suite *QuizTestSuite) TestQuiz_BusinessLogicScenarios() {
	t := suite.T()

	t.Run("QuizDurationCalculation", func(t *testing.T) {
		// Test that quiz duration makes sense relative to max questions
		quiz := suite.sampleQuiz
		avgTimePerQuestion := float64(quiz.TotalDurationSeconds) / float64(quiz.MaxQuestions)

		// Should allow reasonable time per question (at least 30 seconds)
		assert.GreaterOrEqual(t, avgTimePerQuestion, 30.0)

		// Should not be excessive (more than 30 minutes per question)
		assert.LessOrEqual(t, avgTimePerQuestion, 1800.0)
	})

	t.Run("StageConfigConsistency", func(t *testing.T) {
		for stageName, config := range suite.sampleQuiz.StageConfig {
			// Recovery threshold should be <= passing threshold
			assert.LessOrEqual(t, config.RecoveryThreshold, config.PassingThreshold,
				"Stage %s: recovery threshold should be <= passing threshold", stageName)

			// Recovery questions should be <= initial questions
			assert.LessOrEqual(t, config.RecoveryQuestions, config.InitialQuestions,
				"Stage %s: recovery questions should be <= initial questions", stageName)

			// Recovery points should be <= base points
			assert.LessOrEqual(t, config.RecoveryPoints, config.BasePoints,
				"Stage %s: recovery points should be <= base points", stageName)
		}
	})

	t.Run("TotalQuestionsEstimate", func(t *testing.T) {
		// Calculate estimated total questions based on stage configs
		totalEstimatedQuestions := 0
		for _, config := range suite.sampleQuiz.StageConfig {
			// Worst case: initial + recovery questions per stage
			totalEstimatedQuestions += config.InitialQuestions + config.RecoveryQuestions
		}

		// Max questions should be reasonable compared to estimated total
		assert.GreaterOrEqual(t, suite.sampleQuiz.MaxQuestions, len(suite.sampleQuiz.StageConfig),
			"Max questions should be at least the number of stages")

		// Should not be excessive compared to total possible questions
		assert.LessOrEqual(t, suite.sampleQuiz.MaxQuestions, totalEstimatedQuestions,
			"Max questions should not exceed total possible questions from all stages")
	})
}

// Helper methods for validation (these would normally be part of the model or a validator)
func (suite *QuizTestSuite) validateQuiz(quiz *Quiz) bool {
	if quiz.Title == "" {
		return false
	}
	if quiz.TotalDurationSeconds < 0 {
		return false
	}
	if quiz.MaxQuestions <= 0 {
		return false
	}
	return true
}

func (suite *QuizTestSuite) validateStageConfig(config *StageConfig) bool {
	if config.PassingThreshold < 0.0 || config.PassingThreshold > 1.0 {
		return false
	}
	if config.RecoveryThreshold < 0.0 || config.RecoveryThreshold > 1.0 {
		return false
	}
	if config.InitialQuestions < 0 || config.RecoveryQuestions < 0 {
		return false
	}
	if config.BasePoints < 0 || config.RecoveryPoints < 0 {
		return false
	}
	return true
}

// TestQuizSuite runs the test suite
func TestQuizSuite(t *testing.T) {
	suite.Run(t, new(QuizTestSuite))
}

// Additional standalone tests for edge cases
func TestQuizModel_EdgeCases(t *testing.T) {
	t.Run("EmptyStageConfig", func(t *testing.T) {
		quiz := &Quiz{
			ID:          "quiz_empty_config",
			Title:       "Quiz with Empty Config",
			StageConfig: make(map[string]StageConfig),
		}

		assert.NotNil(t, quiz.StageConfig)
		assert.Len(t, quiz.StageConfig, 0)
	})

	t.Run("SingleStageConfig", func(t *testing.T) {
		quiz := &Quiz{
			ID:    "quiz_single_stage",
			Title: "Single Stage Quiz",
			StageConfig: map[string]StageConfig{
				"remember": {
					InitialQuestions:  3,
					PassingThreshold:  0.7,
					RecoveryQuestions: 1,
					RecoveryThreshold: 0.5,
					BasePoints:        10,
					RecoveryPoints:    5,
				},
			},
		}

		assert.Len(t, quiz.StageConfig, 1)
		assert.Contains(t, quiz.StageConfig, "remember")
	})

	t.Run("MaximumDurationQuiz", func(t *testing.T) {
		quiz := &Quiz{
			ID:                   "quiz_max_duration",
			Title:                "Maximum Duration Quiz",
			TotalDurationSeconds: 86400, // 24 hours
			MaxQuestions:         1000,
		}

		avgTimePerQuestion := float64(quiz.TotalDurationSeconds) / float64(quiz.MaxQuestions)
		assert.Equal(t, 86.4, avgTimePerQuestion) // 86.4 seconds per question
	})
}

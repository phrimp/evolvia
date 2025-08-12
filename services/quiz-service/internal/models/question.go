package models

import (
	"fmt"
	"strings"
)

// QuestionType defines supported question types
type QuestionType string

const (
	// Primary supported question types
	QuestionTypeMultipleChoice QuestionType = "multiple_choice"
	QuestionTypeTrueFalse      QuestionType = "true_false"
	QuestionTypeSingleChoice   QuestionType = "single_choice"
)

// GetPrimaryQuestionTypes returns the list of primary supported question types
func GetPrimaryQuestionTypes() []QuestionType {
	return []QuestionType{
		QuestionTypeMultipleChoice,
		QuestionTypeTrueFalse,
		QuestionTypeSingleChoice,
	}
}

// IsValidQuestionType checks if a question type is valid
func IsValidQuestionType(qType string) bool {
	for _, validType := range GetPrimaryQuestionTypes() {
		if string(validType) == qType {
			return true
		}
	}
	return false
}

type Option struct {
	ID        string `bson:"id" json:"id"`
	Text      string `bson:"text" json:"text"`
	IsCorrect bool   `bson:"is_correct,omitempty" json:"is_correct,omitempty"`
}

type Question struct {
	ID            string   `bson:"_id,omitempty" json:"id"`
	Content       string   `bson:"content" json:"content"`
	Type          string   `bson:"type" json:"type"`
	Options       []Option `bson:"options" json:"options"`
	CorrectAnswer string   `bson:"correct_answer" json:"correct_answer"`
	// For true/false questions, stores "true" or "false"
	CorrectAnswerBoolean *bool    `bson:"correct_answer_boolean,omitempty" json:"correct_answer_boolean,omitempty"`
	Explanation          string   `bson:"explanation" json:"explanation"`
	SkillID              string   `bson:"skill_id" json:"skill_id"`
	DifficultyLevel      string   `bson:"difficulty_level" json:"difficulty_level"`
	BloomLevel           string   `bson:"bloom_level" json:"bloom_level"`
	Points               int      `bson:"points" json:"points"`
	EstimatedTimeSeconds int      `bson:"estimated_time_seconds" json:"estimated_time_seconds"`
	TopicTags            []string `bson:"topic_tags" json:"topic_tags"`
	QuestionPoolID       string   `bson:"question_pool_id" json:"question_pool_id"`
	// New Bloom scoring fields
	BloomScore         int            `bson:"bloom_score" json:"bloom_score"`
	BloomScoresByStage map[string]int `bson:"bloom_scores_by_stage" json:"bloom_scores_by_stage"`
}

// BloomBaseScores defines base scores for each Bloom taxonomy level
var BloomBaseScores = map[string]int{
	"remember":   10,
	"understand": 15,
	"apply":      20,
	"analyze":    25,
	"evaluate":   30,
	"create":     35,
}

// StageMultipliers defines score multipliers for difficulty stages
var StageMultipliers = map[string]float64{
	"easy":   1.0,
	"medium": 1.2,
	"hard":   1.5,
}

// CalculateBloomScore calculates and sets the Bloom score based on Bloom level
func (q *Question) CalculateBloomScore() {
	if baseScore, exists := BloomBaseScores[q.BloomLevel]; exists {
		q.BloomScore = baseScore
	} else {
		q.BloomScore = 10 // Default fallback
	}
}

// CalculateBloomScoresByStage calculates scores for all difficulty stages
func (q *Question) CalculateBloomScoresByStage() {
	q.CalculateBloomScore() // Ensure base score is calculated

	q.BloomScoresByStage = make(map[string]int)
	for stage, multiplier := range StageMultipliers {
		q.BloomScoresByStage[stage] = int(float64(q.BloomScore) * multiplier)
	}
}

// GetScoreForStage returns the appropriate score for a given stage
func (q *Question) GetScoreForStage(stage string) int {
	if q.BloomScoresByStage == nil {
		q.CalculateBloomScoresByStage()
	}

	if stageScore, exists := q.BloomScoresByStage[stage]; exists {
		return stageScore
	}

	// Fallback to base score
	return q.BloomScore
}

// EnsureBloomScores ensures both bloom score fields are populated
func (q *Question) EnsureBloomScores() {
	if q.BloomScore == 0 {
		q.CalculateBloomScore()
	}
	if len(q.BloomScoresByStage) == 0 {
		q.CalculateBloomScoresByStage()
	}
}

// Question Factory Methods

// NewMultipleChoiceQuestion creates a new multiple choice question
func NewMultipleChoiceQuestion(content, skillID, bloomLevel string, options []Option, correctAnswerID string) *Question {
	return &Question{
		Content:       content,
		Type:          string(QuestionTypeMultipleChoice),
		Options:       options,
		CorrectAnswer: correctAnswerID,
		SkillID:       skillID,
		BloomLevel:    bloomLevel,
	}
}

// NewTrueFalseQuestion creates a new true/false question
func NewTrueFalseQuestion(content, skillID, bloomLevel string, correctAnswer bool) *Question {
	trueFalseOptions := []Option{
		{ID: "true", Text: "True", IsCorrect: correctAnswer},
		{ID: "false", Text: "False", IsCorrect: !correctAnswer},
	}

	correctAnswerStr := "false"
	if correctAnswer {
		correctAnswerStr = "true"
	}

	return &Question{
		Content:              content,
		Type:                 string(QuestionTypeTrueFalse),
		Options:              trueFalseOptions,
		CorrectAnswer:        correctAnswerStr,
		CorrectAnswerBoolean: &correctAnswer,
		SkillID:              skillID,
		BloomLevel:           bloomLevel,
	}
}

// NewSingleChoiceQuestion creates a new single choice question (like multiple choice but with only one option marked correct)
func NewSingleChoiceQuestion(content, skillID, bloomLevel string, options []Option, correctAnswerID string) *Question {
	// Ensure only one option is marked as correct
	for i := range options {
		options[i].IsCorrect = (options[i].ID == correctAnswerID)
	}

	return &Question{
		Content:       content,
		Type:          string(QuestionTypeSingleChoice),
		Options:       options,
		CorrectAnswer: correctAnswerID,
		SkillID:       skillID,
		BloomLevel:    bloomLevel,
	}
}

// Validation Methods

// Validate validates the question based on its type
func (q *Question) Validate() error {
	if q.Content == "" {
		return fmt.Errorf("question content cannot be empty")
	}

	if !IsValidQuestionType(q.Type) {
		return fmt.Errorf("invalid question type: %s", q.Type)
	}

	switch QuestionType(q.Type) {
	case QuestionTypeMultipleChoice:
		return q.validateMultipleChoice()
	case QuestionTypeTrueFalse:
		return q.validateTrueFalse()
	case QuestionTypeSingleChoice:
		return q.validateSingleChoice()
	default:
		return fmt.Errorf("unsupported question type: %s", q.Type)
	}
}

func (q *Question) validateMultipleChoice() error {
	if len(q.Options) < 2 {
		return fmt.Errorf("multiple choice questions must have at least 2 options")
	}

	if q.CorrectAnswer == "" {
		return fmt.Errorf("multiple choice questions must have a correct answer")
	}

	// Check if correct answer exists in options
	correctOptionFound := false
	for _, option := range q.Options {
		if option.ID == q.CorrectAnswer {
			correctOptionFound = true
			break
		}
	}

	if !correctOptionFound {
		return fmt.Errorf("correct answer ID '%s' not found in options", q.CorrectAnswer)
	}

	return nil
}

func (q *Question) validateTrueFalse() error {
	if len(q.Options) != 2 {
		return fmt.Errorf("true/false questions must have exactly 2 options")
	}

	hasTrue := false
	hasFalse := false
	for _, option := range q.Options {
		if strings.ToLower(option.ID) == "true" {
			hasTrue = true
		}
		if strings.ToLower(option.ID) == "false" {
			hasFalse = true
		}
	}

	if !hasTrue || !hasFalse {
		return fmt.Errorf("true/false questions must have 'true' and 'false' options")
	}

	if q.CorrectAnswer != "true" && q.CorrectAnswer != "false" {
		return fmt.Errorf("true/false questions must have correct answer as 'true' or 'false'")
	}

	return nil
}

func (q *Question) validateSingleChoice() error {
	if len(q.Options) < 2 {
		return fmt.Errorf("single choice questions must have at least 2 options")
	}

	if q.CorrectAnswer == "" {
		return fmt.Errorf("single choice questions must have a correct answer")
	}

	// Check if correct answer exists in options and count correct options
	correctOptionFound := false
	correctCount := 0
	for _, option := range q.Options {
		if option.ID == q.CorrectAnswer {
			correctOptionFound = true
		}
		if option.IsCorrect {
			correctCount++
		}
	}

	if !correctOptionFound {
		return fmt.Errorf("correct answer ID '%s' not found in options", q.CorrectAnswer)
	}

	if correctCount > 1 {
		return fmt.Errorf("single choice questions can only have one correct option")
	}

	return nil
}

// Answer Validation Methods

// IsCorrectAnswer checks if the provided answer is correct
func (q *Question) IsCorrectAnswer(answer string) bool {
	switch QuestionType(q.Type) {
	case QuestionTypeMultipleChoice, QuestionTypeSingleChoice:
		return q.CorrectAnswer == answer
	case QuestionTypeTrueFalse:
		return q.CorrectAnswer == answer
	default:
		return false
	}
}

// IsCorrectBooleanAnswer checks if the provided boolean answer is correct (for true/false questions)
func (q *Question) IsCorrectBooleanAnswer(answer bool) bool {
	if QuestionType(q.Type) != QuestionTypeTrueFalse {
		return false
	}

	if q.CorrectAnswerBoolean != nil {
		return *q.CorrectAnswerBoolean == answer
	}

	// Fallback to string comparison
	return (answer && q.CorrectAnswer == "true") || (!answer && q.CorrectAnswer == "false")
}

// GetCorrectOptions returns the correct options for the question
func (q *Question) GetCorrectOptions() []Option {
	var correctOptions []Option

	for _, option := range q.Options {
		if option.ID == q.CorrectAnswer || option.IsCorrect {
			correctOptions = append(correctOptions, option)
		}
	}

	return correctOptions
}

// GetQuestionTypeInfo returns information about the question type
func (q *Question) GetQuestionTypeInfo() map[string]interface{} {
	info := map[string]interface{}{
		"type":           q.Type,
		"options_count":  len(q.Options),
		"correct_answer": q.CorrectAnswer,
	}

	switch QuestionType(q.Type) {
	case QuestionTypeTrueFalse:
		if q.CorrectAnswerBoolean != nil {
			info["correct_boolean"] = *q.CorrectAnswerBoolean
		}
		info["is_true_false"] = true
	case QuestionTypeMultipleChoice:
		info["is_multiple_choice"] = true
	case QuestionTypeSingleChoice:
		info["is_single_choice"] = true
		correctOptions := q.GetCorrectOptions()
		info["correct_options_count"] = len(correctOptions)
	}

	return info
}

// === Question Sanitization for API Security ===

// SafeOption represents an option without revealing correctness for GET /next-question
type SafeOption struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// ToSafeResponse returns a sanitized question for GET /next-question endpoint
// Hides: correct_answer, correct_answer_boolean, explanation, bloom_score, bloom_scores_by_stage, question_pool_id, options.is_correct
func (q *Question) ToSafeResponse() map[string]interface{} {
	// Sanitize options to remove is_correct field
	safeOptions := make([]SafeOption, len(q.Options))
	for i, option := range q.Options {
		safeOptions[i] = SafeOption{
			ID:   option.ID,
			Text: option.Text,
		}
	}

	return map[string]interface{}{
		"id":                     q.ID,
		"content":                q.Content,
		"type":                   q.Type,
		"options":                safeOptions,
		// Visible fields that don't reveal answers
		"skill_id":               q.SkillID,
		"difficulty_level":       q.DifficultyLevel,
		"bloom_level":            q.BloomLevel,
		"points":                 q.Points,
		"estimated_time_seconds": q.EstimatedTimeSeconds,
		"topic_tags":             q.TopicTags,
	}
}

// GetSensitiveFields returns the sensitive fields that should be shown after answer submission
func (q *Question) GetSensitiveFields() map[string]interface{} {
	return map[string]interface{}{
		"correct_answer":         q.CorrectAnswer,
		"correct_answer_boolean": q.CorrectAnswerBoolean,
		"explanation":            q.Explanation,
		"bloom_score":            q.BloomScore,
		"bloom_scores_by_stage":  q.BloomScoresByStage,
		"question_pool_id":       q.QuestionPoolID,
	}
}

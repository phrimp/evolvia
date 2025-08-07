package selection

import "quiz-service/internal/models"

// QuizPool represents a container of questions for a quiz
type QuizPool struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	SkillID     string            `json:"skill_id"`
	SkillTags   []string          `json:"skill_tags"` // Tags from the skill
	Questions   []models.Question `json:"questions"`
	TotalCount  int               `json:"total_count"`
}

// WeightedQuestion represents a question with its selection weight
type WeightedQuestion struct {
	Question    models.Question `json:"question"`
	Weight      float64         `json:"weight"`
	TagMatches  int             `json:"tag_matches"`
	MatchedTags []string        `json:"matched_tags"`
}

// SelectionCriteria defines criteria for selecting questions
type SelectionCriteria struct {
	SkillID        string   `json:"skill_id"`
	SkillTags      []string `json:"skill_tags"`
	Difficulty     string   `json:"difficulty"`
	ExcludeIDs     []string `json:"exclude_ids"`
	Count          int      `json:"count"`
	MinTagMatch    int      `json:"min_tag_match"`   // Minimum tags that must match
	WeightExponent float64  `json:"weight_exponent"` // Exponent for weight calculation (default 2.0)
}

// SelectionResult contains the selected questions and metadata
type SelectionResult struct {
	Questions       []models.Question  `json:"questions"`
	TotalCandidates int                `json:"total_candidates"`
	Weights         []WeightedQuestion `json:"weights,omitempty"`
	AverageMatch    float64            `json:"average_match"`
}

// SkillInfo represents skill information for selection
type SkillInfo struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Tags           []string `json:"tags"`
	TechnicalTerms []string `json:"technical_terms"`
	CommonNames    []string `json:"common_names"`
}

// Default selection configuration
func DefaultSelectionCriteria() *SelectionCriteria {
	return &SelectionCriteria{
		Count:          5,
		MinTagMatch:    0,   // Accept any question, but prefer higher matches
		WeightExponent: 2.0, // Square the match count for weighting
	}
}

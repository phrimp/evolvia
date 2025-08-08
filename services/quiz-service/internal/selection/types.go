// services/quiz-service/internal/selection/types.go
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
	// New fields for Bloom's level tracking
	BloomDistribution map[string]int            `json:"bloom_distribution"`
	DifficultyMatrix  map[string]map[string]int `json:"difficulty_matrix"` // difficulty -> bloom -> count
}

// WeightedQuestion represents a question with its selection weight
type WeightedQuestion struct {
	Question    models.Question `json:"question"`
	Weight      float64         `json:"weight"`
	TagMatches  int             `json:"tag_matches"`
	MatchedTags []string        `json:"matched_tags"`
	// New fields for Bloom's level
	BloomLevel  string  `json:"bloom_level"`
	TagWeight   float64 `json:"tag_weight"`   // Weight from tag matching
	BloomWeight float64 `json:"bloom_weight"` // Weight from Bloom's level
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
	// New fields for Bloom's level
	BloomLevels       []string           `json:"bloom_levels"`       // Preferred Bloom's levels
	BloomDistribution map[string]float64 `json:"bloom_distribution"` // Desired distribution
}

// SelectionResult contains the selected questions and metadata
type SelectionResult struct {
	Questions       []models.Question  `json:"questions"`
	TotalCandidates int                `json:"total_candidates"`
	Weights         []WeightedQuestion `json:"weights,omitempty"`
	AverageMatch    float64            `json:"average_match"`
	// New fields for analysis
	BloomCoverage  map[string]int `json:"bloom_coverage"` // Actual Bloom's distribution in result
	TagCoverage    map[string]int `json:"tag_coverage"`   // Tags covered in result
	SelectionStats SelectionStats `json:"selection_stats"`
}

// SelectionStats provides detailed statistics about the selection
type SelectionStats struct {
	TotalQuestionsScanned int            `json:"total_questions_scanned"`
	QuestionsFiltered     int            `json:"questions_filtered"`
	AverageTagMatch       float64        `json:"average_tag_match"`
	AverageWeight         float64        `json:"average_weight"`
	BloomLevelHits        map[string]int `json:"bloom_level_hits"`
	DifficultyHits        map[string]int `json:"difficulty_hits"`
}

// SkillInfo represents skill information for selection
type SkillInfo struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Tags           []string `json:"tags"`
	TechnicalTerms []string `json:"technical_terms"`
	CommonNames    []string `json:"common_names"`
	// New fields for enhanced matching
	PrimaryTags   []string `json:"primary_tags"`   // Most important tags
	SecondaryTags []string `json:"secondary_tags"` // Supporting tags
	RelatedSkills []string `json:"related_skills"` // Related skill IDs
}

// QuizPoolValidation represents validation results for a quiz pool
type QuizPoolValidation struct {
	IsValid               bool                      `json:"is_valid"`
	TotalQuestions        int                       `json:"total_questions"`
	DifficultyCount       map[string]int            `json:"difficulty_count"`
	BloomCount            map[string]int            `json:"bloom_count"`
	MissingLevels         []string                  `json:"missing_levels"`
	Warnings              []string                  `json:"warnings"`
	DifficultyBloomMatrix map[string]map[string]int `json:"difficulty_bloom_matrix"`
}

// Default selection configuration
func DefaultSelectionCriteria() *SelectionCriteria {
	return &SelectionCriteria{
		Count:          5,
		MinTagMatch:    0,   // Accept any question, but prefer higher matches
		WeightExponent: 2.0, // Square the match count for weighting
		BloomDistribution: map[string]float64{
			"remember":   0.2,
			"understand": 0.2,
			"apply":      0.2,
			"analyze":    0.2,
			"evaluate":   0.1,
			"create":     0.1,
		},
	}
}

// BloomTaxonomyLevels defines the standard Bloom's taxonomy levels in order
var BloomTaxonomyLevels = []string{
	"remember",
	"understand",
	"apply",
	"analyze",
	"evaluate",
	"create",
}

// BloomLevelWeights defines default importance weights for each Bloom level
var BloomLevelWeights = map[string]float64{
	"remember":   1.0,
	"understand": 1.2,
	"apply":      1.5,
	"analyze":    1.8,
	"evaluate":   2.0,
	"create":     2.5,
}

// DifficultyBloomMatrix defines typical Bloom's distribution per difficulty
var DifficultyBloomMatrix = map[string]map[string]float64{
	"easy": {
		"remember":   0.5,
		"understand": 0.3,
		"apply":      0.2,
	},
	"medium": {
		"understand": 0.2,
		"apply":      0.4,
		"analyze":    0.3,
		"evaluate":   0.1,
	},
	"hard": {
		"apply":    0.1,
		"analyze":  0.3,
		"evaluate": 0.4,
		"create":   0.2,
	},
}

// TagWeightConfig defines weight multipliers for different tag categories
type TagWeightConfig struct {
	PrimaryWeight   float64 `json:"primary_weight"`    // Weight for primary tags (default: 3.0)
	SecondaryWeight float64 `json:"secondary_weight"`  // Weight for secondary tags (default: 1.5)
	RelatedWeight   float64 `json:"related_weight"`    // Weight for related tags (default: 0.5)
	ExactMatchBonus float64 `json:"exact_match_bonus"` // Bonus for exact skill ID match (default: 2.0)
}

// EnhancedSkillInfo extends SkillInfo with categorized tags and weights
type EnhancedSkillInfo struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	PrimaryTags   []string        `json:"primary_tags"`   // Core concepts - highest weight
	SecondaryTags []string        `json:"secondary_tags"` // Supporting concepts - medium weight
	RelatedTags   []string        `json:"related_tags"`   // Peripheral concepts - low weight
	TagWeights    TagWeightConfig `json:"tag_weights"`    // Weight configuration
}

// EnhancedWeightedQuestion provides detailed weight breakdown
type EnhancedWeightedQuestion struct {
	Question    models.Question `json:"question"`
	TotalWeight float64         `json:"total_weight"`

	// Tag matching breakdown
	PrimaryMatches   int      `json:"primary_matches"`
	SecondaryMatches int      `json:"secondary_matches"`
	RelatedMatches   int      `json:"related_matches"`
	MatchedPrimary   []string `json:"matched_primary"`
	MatchedSecondary []string `json:"matched_secondary"`
	MatchedRelated   []string `json:"matched_related"`

	// Weight components
	TagWeight        float64            `json:"tag_weight"`
	BloomWeight      float64            `json:"bloom_weight"`
	SkillMatchBonus  float64            `json:"skill_match_bonus"`
	WeightComponents map[string]float64 `json:"weight_components"`
}

// EnhancedSelectionCriteria extends selection criteria with tag weights
type EnhancedSelectionCriteria struct {
	SkillInfo         *EnhancedSkillInfo `json:"skill_info"`
	Difficulty        string             `json:"difficulty"`
	ExcludeIDs        []string           `json:"exclude_ids"`
	Count             int                `json:"count"`
	MinPrimaryMatch   int                `json:"min_primary_match"`
	MinSecondaryMatch int                `json:"min_secondary_match"`
	PreferExactSkill  bool               `json:"prefer_exact_skill"`
	WeightExponent    float64            `json:"weight_exponent"`
	BloomDistribution map[string]float64 `json:"bloom_distribution"`
}

// Default tag weight configuration
func DefaultTagWeightConfig() TagWeightConfig {
	return TagWeightConfig{
		PrimaryWeight:   3.0, // Primary tags are 3x base weight
		SecondaryWeight: 1.5, // Secondary tags are 1.5x base weight
		RelatedWeight:   0.5, // Related tags are 0.5x base weight
		ExactMatchBonus: 2.0, // 2x multiplier for exact skill match
	}
}

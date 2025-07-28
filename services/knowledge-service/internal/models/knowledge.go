package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// SkillLevel represents proficiency levels
type SkillLevel string

const (
	SkillLevelBeginner     SkillLevel = "beginner"
	SkillLevelIntermediate SkillLevel = "intermediate"
	SkillLevelAdvanced     SkillLevel = "advanced"
	SkillLevelExpert       SkillLevel = "expert"
)

// RelationType defines how skills are related
type RelationType string

const (
	RelationPrerequisite   RelationType = "prerequisite"   // Required before learning this skill
	RelationBuildsOn       RelationType = "builds_on"      // This skill builds upon another
	RelationRelated        RelationType = "related"        // Related/similar skills
	RelationComplement     RelationType = "complement"     // Skills that work well together
	RelationAlternative    RelationType = "alternative"    // Alternative skills for same purpose
	RelationSpecialization RelationType = "specialization" // More specific version of a skill
)

// KeywordPattern represents different ways to identify a skill in text
type KeywordPattern struct {
	Text            string  `bson:"text" json:"text"`     // The actual keyword/phrase
	Weight          float64 `bson:"weight" json:"weight"` // 0.0 to 1.0, importance for identification
	Type            string  `bson:"type" json:"type"`     // "exact", "partial", "regex", "context"
	CaseSensitive   bool    `bson:"case_sensitive" json:"case_sensitive"`
	MinWordBoundary bool    `bson:"min_word_boundary" json:"min_word_boundary"` // Require word boundaries
}

// SkillRelation represents relationship between skills
type SkillRelation struct {
	SkillID      bson.ObjectID `bson:"skill_id" json:"skill_id"`
	RelationType RelationType  `bson:"relation_type" json:"relation_type"`
	Strength     float64       `bson:"strength" json:"strength"` // 0.0 to 1.0
	Description  string        `bson:"description,omitempty" json:"description,omitempty"`
}

// SkillCategory represents hierarchical categorization
type SkillCategory struct {
	ID       bson.ObjectID  `bson:"_id,omitempty" json:"id,omitempty"`
	Name     string         `bson:"name" json:"name"`
	ParentID *bson.ObjectID `bson:"parent_id,omitempty" json:"parent_id,omitempty"`
	Path     string         `bson:"path" json:"path"`   // e.g., "Technology/Programming/Web Development"
	Level    int            `bson:"level" json:"level"` // Depth in hierarchy
}

// SkillMetadata contains additional skill information
type SkillMetadata struct {
	Industry     []string `bson:"industry,omitempty" json:"industry,omitempty"`
	JobRoles     []string `bson:"job_roles,omitempty" json:"job_roles,omitempty"`
	Difficulty   int      `bson:"difficulty" json:"difficulty"`       // 1-10 scale
	TimeToLearn  int      `bson:"time_to_learn" json:"time_to_learn"` // Hours
	Trending     bool     `bson:"trending" json:"trending"`
	MarketDemand float64  `bson:"market_demand" json:"market_demand"` // 0.0 to 1.0
}

// SkillIdentificationRules defines how to identify this skill from text
type SkillIdentificationRules struct {
	// Primary patterns that strongly indicate this skill
	PrimaryPatterns []KeywordPattern `bson:"primary_patterns" json:"primary_patterns"`

	// Secondary patterns that provide additional evidence
	SecondaryPatterns []KeywordPattern `bson:"secondary_patterns" json:"secondary_patterns"`

	AcademicPatterns []KeywordPattern `bson:"academic_patterns" json:"academic_patterns"`

	// Negative patterns that should reduce confidence
	NegativePatterns []KeywordPattern `bson:"negative_patterns,omitempty" json:"negative_patterns,omitempty"`

	// Minimum requirements for positive identification
	MinPrimaryMatches   int     `bson:"min_primary_matches" json:"min_primary_matches"`
	MinSecondaryMatches int     `bson:"min_secondary_matches" json:"min_secondary_matches"`
	MinAcademicMatch    int     `bson:"min_academic_matches" json:"min_academic_matches"`
	MinTotalScore       float64 `bson:"min_total_score" json:"min_total_score"`

	// Context window for pattern matching (words before/after)
	ContextWindow int `bson:"context_window" json:"context_window"`
}

// Skill represents a knowledge/skill entity
type Skill struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name        string        `bson:"name" json:"name"`
	Description string        `bson:"description" json:"description"`

	// Text identification rules
	IdentificationRules SkillIdentificationRules `bson:"identification_rules" json:"identification_rules"`

	// Simple keyword lists for backward compatibility and quick matching
	CommonNames    []string `bson:"common_names" json:"common_names"` // Alternative names
	Abbreviations  []string `bson:"abbreviations,omitempty" json:"abbreviations,omitempty"`
	TechnicalTerms []string `bson:"technical_terms,omitempty" json:"technical_terms,omitempty"`

	// Categorization
	Category   *SkillCategory `bson:"category,omitempty" json:"category,omitempty"`
	CategoryID *bson.ObjectID `bson:"category_id,omitempty" json:"category_id,omitempty"`
	Tags       []string       `bson:"tags" json:"tags"`

	// Relationships with other skills
	Relations []SkillRelation `bson:"relations" json:"relations"`

	// Metadata
	Metadata SkillMetadata `bson:"metadata" json:"metadata"`

	// Versioning and tracking
	Version   int            `bson:"version" json:"version"`
	IsActive  bool           `bson:"is_active" json:"is_active"`
	CreatedAt time.Time      `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time      `bson:"updated_at" json:"updated_at"`
	CreatedBy *bson.ObjectID `bson:"created_by,omitempty" json:"created_by,omitempty"`
	UpdatedBy *bson.ObjectID `bson:"updated_by,omitempty" json:"updated_by,omitempty"`

	// Usage statistics
	UsageCount int        `bson:"usage_count" json:"usage_count"`
	LastUsed   *time.Time `bson:"last_used,omitempty" json:"last_used,omitempty"`
}

// SkillMatch represents a skill identified in text with detailed matching info
type SkillMatch struct {
	SkillID    bson.ObjectID `bson:"skill_id" json:"skill_id"`
	Skill      *Skill        `bson:"skill,omitempty" json:"skill,omitempty"`
	TotalScore float64       `bson:"total_score" json:"total_score"`
	Confidence float64       `bson:"confidence" json:"confidence"`

	// Detailed matching breakdown
	PrimaryMatches   []PatternMatch `bson:"primary_matches" json:"primary_matches"`
	SecondaryMatches []PatternMatch `bson:"secondary_matches" json:"secondary_matches"`
	NegativeMatches  []PatternMatch `bson:"negative_matches,omitempty" json:"negative_matches,omitempty"`
	ContextMatches   []PatternMatch `bson:"context_matches,omitempty" json:"context_matches,omitempty"`

	// Text location info
	TextSpans          []TextSpan `bson:"text_spans" json:"text_spans"`
	SurroundingContext string     `bson:"surrounding_context,omitempty" json:"surrounding_context,omitempty"`
}

// PatternMatch represents a single pattern match in text
type PatternMatch struct {
	Pattern     KeywordPattern `bson:"pattern" json:"pattern"`
	MatchedText string         `bson:"matched_text" json:"matched_text"`
	Score       float64        `bson:"score" json:"score"`
	Position    int            `bson:"position" json:"position"`
	Length      int            `bson:"length" json:"length"`
}

// TextSpan represents a span of text where skill was identified
type TextSpan struct {
	StartPos int    `bson:"start_pos" json:"start_pos"`
	EndPos   int    `bson:"end_pos" json:"end_pos"`
	Text     string `bson:"text" json:"text"`
}

// SkillGraph represents skill relationships for analysis
type SkillGraph struct {
	ID        bson.ObjectID   `bson:"_id,omitempty" json:"id,omitempty"`
	Name      string          `bson:"name" json:"name"`
	Skills    []bson.ObjectID `bson:"skills" json:"skills"`
	Relations []SkillRelation `bson:"relations" json:"relations"`
	CreatedAt time.Time       `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time       `bson:"updated_at" json:"updated_at"`
}

// UserSkill represents a user's skill proficiency
type UserSkill struct {
	ID               bson.ObjectID            `bson:"_id,omitempty" json:"id,omitempty"`
	UserID           bson.ObjectID            `bson:"user_id" json:"user_id"`
	SkillID          bson.ObjectID            `bson:"skill_id" json:"skill_id"`
	Level            SkillLevel               `bson:"level" json:"level"`
	Confidence       float64                  `bson:"confidence" json:"confidence"`
	YearsExperience  int                      `bson:"years_experience" json:"years_experience"`
	LastUsed         *time.Time               `bson:"last_used,omitempty" json:"last_used,omitempty"`
	Verified         bool                     `bson:"verified" json:"verified"`
	Endorsements     int                      `bson:"endorsements" json:"endorsements"`
	BloomsAssessment BloomsTaxonomyAssessment `bson:"blooms_assessment" json:"blooms_assessment"`
	CreatedAt        time.Time                `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time                `bson:"updated_at" json:"updated_at"`
}

type BloomsTaxonomyAssessment struct {
	Remember    float64   `bson:"remember" json:"remember"`     // Recalling facts and terminology
	Understand  float64   `bson:"understand" json:"understand"` // Explaining concepts
	Apply       float64   `bson:"apply" json:"apply"`           // Implementing and using knowledge
	Analyze     float64   `bson:"analyze" json:"analyze"`       // Breaking down complex problems
	Evaluate    float64   `bson:"evaluate" json:"evaluate"`     // Assessing and comparing solutions
	Create      float64   `bson:"create" json:"create"`         // Building original projects/solutions
	LastUpdated time.Time `bson:"last_updated" json:"last_updated"`
}

// GetOverallScore calculates weighted average across all Bloom's levels
func (b *BloomsTaxonomyAssessment) GetOverallScore() float64 {
	// Weight higher cognitive levels more heavily
	weights := map[string]float64{
		"remember":   0.10,
		"understand": 0.15,
		"apply":      0.20,
		"analyze":    0.20,
		"evaluate":   0.20,
		"create":     0.15,
	}

	total := b.Remember*weights["remember"] +
		b.Understand*weights["understand"] +
		b.Apply*weights["apply"] +
		b.Analyze*weights["analyze"] +
		b.Evaluate*weights["evaluate"] +
		b.Create*weights["create"]

	return total
}

// GetPrimaryStrength returns the Bloom's level with highest score
func (b *BloomsTaxonomyAssessment) GetPrimaryStrength() string {
	scores := map[string]float64{
		"remember":   b.Remember,
		"understand": b.Understand,
		"apply":      b.Apply,
		"analyze":    b.Analyze,
		"evaluate":   b.Evaluate,
		"create":     b.Create,
	}

	var maxLevel string
	var maxScore float64
	for level, score := range scores {
		if score > maxScore {
			maxScore = score
			maxLevel = level
		}
	}

	// If all scores are zero, return empty string
	if maxScore == 0 {
		return ""
	}

	return maxLevel
}

func (b *BloomsTaxonomyAssessment) GetWeakestArea() string {
	scores := map[string]float64{
		"remember":   b.Remember,
		"understand": b.Understand,
		"apply":      b.Apply,
		"analyze":    b.Analyze,
		"evaluate":   b.Evaluate,
		"create":     b.Create,
	}

	// Check if all scores are zero (not assessed yet)
	allZero := true
	for _, score := range scores {
		if score > 0 {
			allZero = false
			break
		}
	}

	// If never assessed, recommend starting with fundamentals
	if allZero {
		return "remember"
	}

	var minLevel string
	minScore := 100.0
	for level, score := range scores {
		if score >= 0 && score < minScore { // Include zeros for assessed skills
			minScore = score
			minLevel = level
		}
	}

	return minLevel
}

// MongoDB indexes for optimal performance
func GetSkillIndexes() []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "name", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "identification_rules.primary_patterns.text", Value: 1},
				{Key: "identification_rules.primary_patterns.weight", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "identification_rules.secondary_patterns.text", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "identification_rules.academic_patterns.text", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "common_names", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "abbreviations", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "technical_terms", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "category_id", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "tags", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "is_active", Value: 1},
				{Key: "usage_count", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "metadata.industry", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "metadata.job_roles", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "metadata.difficulty", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "metadata.trending", Value: 1},
				{Key: "metadata.market_demand", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "created_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "updated_at", Value: -1},
			},
		},
		// Text search index for general search functionality
		{
			Keys: bson.D{
				{Key: "name", Value: "text"},
				{Key: "description", Value: "text"},
				{Key: "common_names", Value: "text"},
				{Key: "technical_terms", Value: "text"},
				{Key: "tags", Value: "text"},
			},
			Options: options.Index().SetWeights(bson.M{
				"name":            10,
				"description":     5,
				"common_names":    8,
				"technical_terms": 6,
				"tags":            4,
			}),
		},
	}
}

type UserSkillWithDetails struct {
	*UserSkill
	SkillName        string   `json:"skill_name"`
	SkillDescription string   `json:"skill_description,omitempty"`
	SkillTags        []string `json:"skill_tags,omitempty"`
}

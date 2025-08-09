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
	RelationPrerequisite RelationType = "prerequisite" // Required before learning this skill
	RelationBuildsOn     RelationType = "builds_on"    // This skill builds upon another
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
	Name         string        `bson:"name" json:"name"`
	RelationType RelationType  `bson:"relation_type" json:"relation_type"`
	Strength     float64       `bson:"strength" json:"strength"` // 0.0 to 1.0
	Description  string        `bson:"description,omitempty" json:"description,omitempty"`
	TaggedSkill  TaggedSkill   `bson:"tagged_skill" json:"tagged_skill"`
}

// SkillCategory represents hierarchical categorization
type SkillCategory struct {
	ID        bson.ObjectID  `bson:"_id,omitempty" json:"id,omitempty"`
	Name      string         `bson:"name" json:"name"`
	ParentID  *bson.ObjectID `bson:"parent_id,omitempty" json:"parent_id,omitempty"`
	Path      string         `bson:"path" json:"path"`   // e.g., "Technology/Programming/Web Development"
	Level     int            `bson:"level" json:"level"` // Depth in hierarchy
	CreatedAt time.Time      `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time      `bson:"updated_at" json:"updated_at"`
}

type CategoryNode struct {
	Category *SkillCategory
	Children []*CategoryNode `json:"children"`
}

// CategoryStatistics contains statistical information about categories
type CategoryStatistics struct {
	TotalCategories int                       `json:"total_categories"`
	RootCategories  int                       `json:"root_categories"`
	MaxDepth        int                       `json:"max_depth"`
	ByLevel         map[int]int               `json:"by_level"`
	TopCategories   []*CategoryWithSkillCount `json:"top_categories"`
}

// CategoryWithSkillCount represents a category with associated skill count
type CategoryWithSkillCount struct {
	*SkillCategory
	SkillCount int `json:"skill_count"`
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

	// Legacy tags field (deprecated - kept for backward compatibility)
	Tags []string `bson:"tags,omitempty" json:"tags,omitempty"`

	// New categorized tags with weights
	TaggedSkill TaggedSkill `bson:"tagged_skill" json:"tagged_skill"`

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
	Addable    bool       `bson:"addable" json:"addable"`
}

// GetAllTags returns all tags for backward compatibility
func (s *Skill) GetAllTags() []string {
	// If legacy tags exist and new tags don't, return legacy
	if len(s.Tags) > 0 && len(s.TaggedSkill.PrimaryTags) == 0 &&
		len(s.TaggedSkill.SecondaryTags) == 0 && len(s.TaggedSkill.RelatedTags) == 0 {
		return s.Tags
	}
	// Otherwise return new categorized tags
	return s.TaggedSkill.GetAllTags()
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
	ID                  bson.ObjectID            `bson:"_id,omitempty" json:"id,omitempty"`
	UserID              bson.ObjectID            `bson:"user_id" json:"user_id"`
	SkillID             bson.ObjectID            `bson:"skill_id" json:"skill_id"`
	Level               SkillLevel               `bson:"level" json:"level"`
	Confidence          float64                  `bson:"confidence" json:"confidence"`
	YearsExperience     int                      `bson:"years_experience" json:"years_experience"`
	LastUsed            *time.Time               `bson:"last_used,omitempty" json:"last_used,omitempty"`
	Verified            bool                     `bson:"verified" json:"verified"`
	Endorsements        int                      `bson:"endorsements" json:"endorsements"`
	BloomsAssessment    BloomsTaxonomyAssessment `bson:"blooms_assessment" json:"blooms_assessment"`
	CreatedAt           time.Time                `bson:"created_at" json:"created_at"`
	UpdatedAt           time.Time                `bson:"updated_at" json:"updated_at"`
	VerificationHistory []SkillProgressHistory   `bson:"verification_history" json:"verification_history"`
}

type BloomsTaxonomyAssessment struct {
	Remember    float64   `bson:"remember" json:"remember"`     // Recalling facts and terminology
	Understand  float64   `bson:"understand" json:"understand"` // Explaining concepts
	Apply       float64   `bson:"apply" json:"apply"`           // Implementing and using knowledge
	Analyze     float64   `bson:"analyze" json:"analyze"`       // Breaking down complex problems
	Evaluate    float64   `bson:"evaluate" json:"evaluate"`     // Assessing and comparing solutions
	Create      float64   `bson:"create" json:"create"`         // Building original projects/solutions
	Verified    bool      `bson:"verified" json:"verified"`
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
				{Key: "tagged_skill.primary_tags", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "tagged_skill.secondary_tags", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "tagged_skill.related_tags", Value: 1},
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
				{Key: "tagged_skill.primary_tags", Value: "text"},
				{Key: "tagged_skill.secondary_tags", Value: "text"},
				{Key: "tagged_skill.related_tags", Value: "text"},
			},
			Options: options.Index().SetWeights(bson.M{
				"name":                        10,
				"description":                 5,
				"common_names":                8,
				"technical_terms":             6,
				"tags":                        4, // Legacy
				"tagged_skill.primary_tags":   9, // High weight for primary
				"tagged_skill.secondary_tags": 6, // Medium weight for secondary
				"tagged_skill.related_tags":   3, // Low weight for related
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

type SkillProgressHistory struct {
	ID                bson.ObjectID            `bson:"_id,omitempty"`
	UserID            bson.ObjectID            `bson:"user_id"`
	SkillID           bson.ObjectID            `bson:"skill_id"`
	BloomsSnapshot    BloomsTaxonomyAssessment `bson:"blooms_snapshot"`
	TotalHours        float64                  `bson:"total_hours"`
	VerificationCount int                      `bson:"verification_count"`
	Timestamp         time.Time                `bson:"timestamp"`
	TriggerEvent      string                   `bson:"trigger_event"` // "verification", "learning_session", "manual"
}

// Add this to your internal/models/knowledge.go file

// SkillWithCategory represents a skill with its category information for search results
type SkillWithCategory struct {
	*Skill
	CategoryName string `bson:"category_name,omitempty" json:"category_name,omitempty"`
	CategoryPath string `bson:"category_path,omitempty" json:"category_path,omitempty"`
}

// SkillSearchResult represents enhanced search results with match scoring
type SkillSearchResult struct {
	*SkillWithCategory
	MatchScore    float64  `json:"match_score,omitempty"`
	MatchedFields []string `json:"matched_fields,omitempty"`
}

type SkillWithStats struct {
	ID                bson.ObjectID  `bson:"_id,omitempty" json:"id,omitempty"`
	Name              string         `bson:"name" json:"name"`
	Description       string         `bson:"description" json:"description"`
	CategoryID        *bson.ObjectID `bson:"category_id,omitempty" json:"category_id,omitempty"`
	CategoryName      string         `bson:"category_name,omitempty" json:"category_name,omitempty"`
	Tags              []string       `bson:"tags" json:"tags"`
	Metadata          SkillMetadata  `bson:"metadata" json:"metadata"`
	UsageCount        int            `bson:"usage_count" json:"usage_count"`
	UserCount         int            `bson:"user_count" json:"user_count"`
	TotalEndorsements int            `bson:"total_endorsements" json:"total_endorsements"`
	LastUsed          *time.Time     `bson:"last_used,omitempty" json:"last_used,omitempty"`
	CreatedAt         time.Time      `bson:"created_at" json:"created_at"`
	UpdatedAt         time.Time      `bson:"updated_at" json:"updated_at"`
}

// TopSkillsResponse represents the response for top skills endpoint
type TopSkillsResponse struct {
	Skills    []*SkillWithStats `json:"skills"`
	Criteria  string            `json:"criteria"`
	Count     int               `json:"count"`
	Limit     int               `json:"limit"`
	Timestamp time.Time         `json:"timestamp"`
}

// TopSkillsCriteria defines available criteria for top skills selection
type TopSkillsCriteria string

const (
	TopSkillsByUsage        TopSkillsCriteria = "usage"        // Most frequently used
	TopSkillsByPopularity   TopSkillsCriteria = "popularity"   // Most users
	TopSkillsByEndorsements TopSkillsCriteria = "endorsements" // Most endorsed
	TopSkillsByTrending     TopSkillsCriteria = "trending"     // Trending skills
	TopSkillsByRecent       TopSkillsCriteria = "recent"       // Recently added
)

type TaggedSkill struct {
	PrimaryTags   []string `bson:"primary_tags" json:"primary_tags"`     // Core skill concepts (highest weight)
	SecondaryTags []string `bson:"secondary_tags" json:"secondary_tags"` // Supporting concepts (medium weight)
	RelatedTags   []string `bson:"related_tags" json:"related_tags"`     // Peripheral concepts (lowest weight)
}

// GetAllTags returns all tags combined (for backward compatibility)
func (ts *TaggedSkill) GetAllTags() []string {
	allTags := make([]string, 0, len(ts.PrimaryTags)+len(ts.SecondaryTags)+len(ts.RelatedTags))
	allTags = append(allTags, ts.PrimaryTags...)
	allTags = append(allTags, ts.SecondaryTags...)
	allTags = append(allTags, ts.RelatedTags...)
	return allTags
}

// GetWeightedTags returns tags with their weights for scoring algorithms
func (ts *TaggedSkill) GetWeightedTags() map[string]float64 {
	weighted := make(map[string]float64)

	// Primary tags have highest weight (1.0)
	for _, tag := range ts.PrimaryTags {
		weighted[tag] = 1.0
	}

	// Secondary tags have medium weight (0.6)
	for _, tag := range ts.SecondaryTags {
		if _, exists := weighted[tag]; !exists {
			weighted[tag] = 0.6
		}
	}

	// Related tags have lowest weight (0.3)
	for _, tag := range ts.RelatedTags {
		if _, exists := weighted[tag]; !exists {
			weighted[tag] = 0.3
		}
	}

	return weighted
}

// HasTag checks if a tag exists in any category
func (ts *TaggedSkill) HasTag(tag string) bool {
	for _, t := range ts.PrimaryTags {
		if t == tag {
			return true
		}
	}
	for _, t := range ts.SecondaryTags {
		if t == tag {
			return true
		}
	}
	for _, t := range ts.RelatedTags {
		if t == tag {
			return true
		}
	}
	return false
}

// GetTagWeight returns the weight of a specific tag
func (ts *TaggedSkill) GetTagWeight(tag string) float64 {
	for _, t := range ts.PrimaryTags {
		if t == tag {
			return 1.0
		}
	}
	for _, t := range ts.SecondaryTags {
		if t == tag {
			return 0.6
		}
	}
	for _, t := range ts.RelatedTags {
		if t == tag {
			return 0.3
		}
	}
	return 0.0
}

func (s *Skill) MigrateLegacyTags() {
	if len(s.Tags) > 0 && len(s.TaggedSkill.PrimaryTags) == 0 {
		// Simple migration strategy:
		// - First 1-2 tags become primary
		// - Next 2-3 tags become secondary
		// - Rest become related

		tagCount := len(s.Tags)

		if tagCount > 0 {
			// Determine primary tags (up to 2)
			primaryCount := 1
			if tagCount >= 4 {
				primaryCount = 2
			}
			s.TaggedSkill.PrimaryTags = s.Tags[:primaryCount]

			// Determine secondary tags
			if tagCount > primaryCount {
				secondaryEnd := primaryCount + 2
				if secondaryEnd > tagCount {
					secondaryEnd = tagCount
				}
				s.TaggedSkill.SecondaryTags = s.Tags[primaryCount:secondaryEnd]

				// Rest become related tags
				if secondaryEnd < tagCount {
					s.TaggedSkill.RelatedTags = s.Tags[secondaryEnd:]
				}
			}
		}
	}
}

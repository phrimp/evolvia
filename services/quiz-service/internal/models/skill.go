package models

import "time"

type Skill struct {
	ID                  string                 `bson:"_id,omitempty" json:"id"`
	Name                string                 `bson:"name" json:"name"`
	Description         string                 `bson:"description" json:"description"`
	IdentificationRules map[string]interface{} `bson:"identification_rules" json:"identification_rules"`
	CommonNames         []string               `bson:"common_names" json:"common_names"`
	TechnicalTerms      []string               `bson:"technical_terms" json:"technical_terms"`
	CategoryID          string                 `bson:"category_id" json:"category_id"`
	TaggedSkill         map[string]interface{} `bson:"tagged_skill" json:"tagged_skill"`
	Relations           []interface{}          `bson:"relations" json:"relations"`
	Metadata            SkillMetadata          `bson:"metadata" json:"metadata"`
	Version             int                    `bson:"version" json:"version"`
	IsActive            bool                   `bson:"is_active" json:"is_active"`
	CreatedAt           time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt           time.Time              `bson:"updated_at" json:"updated_at"`
	UsageCount          int                    `bson:"usage_count" json:"usage_count"`
	LastUsed            time.Time              `bson:"last_used" json:"last_used"`
}

type SkillMetadata struct {
	Industry     []string `bson:"industry" json:"industry"`
	JobRoles     []string `bson:"job_roles" json:"job_roles"`
	Difficulty   int      `bson:"difficulty" json:"difficulty"`
	TimeToLearn  int      `bson:"time_to_learn" json:"time_to_learn"`
	Trending     bool     `bson:"trending" json:"trending"`
	MarketDemand float64  `bson:"market_demand" json:"market_demand"`
}

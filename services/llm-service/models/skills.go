package models

type SkillExtractionRequest struct {
	Content string `json:"content" binding:"required"`
}

type SkillExtractionResponse struct {
	Skills []Skill `json:"skills"`
	ExtractionMetadata ExtractionMetadata `json:"extraction_metadata"`
}

type Skill struct {
	Name                string                `json:"name"`
	Description         string                `json:"description"`
	IdentificationRules IdentificationRules   `json:"identification_rules"`
	CommonNames         []string              `json:"common_names"`
	Abbreviations       []string              `json:"abbreviations"`
	TechnicalTerms      []string              `json:"technical_terms"`
	Category            SkillCategory         `json:"category"`
	Tags                []string              `json:"tags"`
	Relations           []SkillRelation       `json:"relations"`
	Metadata            SkillMetadata         `json:"metadata"`
	IsActive            bool                  `json:"is_active"`
	Version             int                   `json:"version"`
}

type IdentificationRules struct {
	PrimaryPatterns     []Pattern `json:"primary_patterns"`
	SecondaryPatterns   []Pattern `json:"secondary_patterns"`
	AcademicPatterns    []Pattern `json:"academic_patterns"`
	NegativePatterns    []Pattern `json:"negative_patterns"`
	MinPrimaryMatches   int       `json:"min_primary_matches"`
	MinSecondaryMatches int       `json:"min_secondary_matches"`
	MinAcademicMatches  int       `json:"min_academic_matches"`
	MinTotalScore       float64   `json:"min_total_score"`
	ContextWindow       int       `json:"context_window"`
}

type Pattern struct {
	Text           string  `json:"text"`
	Weight         float64 `json:"weight"`
	Type           string  `json:"type"`
	CaseSensitive  bool    `json:"case_sensitive"`
	MinWordBoundary bool   `json:"min_word_boundary"`
}

type SkillCategory struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Level int   `json:"level"`
}

type SkillRelation struct {
	SkillName    string  `json:"skill_name"`
	RelationType string  `json:"relation_type"`
	Strength     float64 `json:"strength"`
	Description  string  `json:"description"`
}

type SkillMetadata struct {
	Industry      []string `json:"industry"`
	JobRoles      []string `json:"job_roles"`
	Difficulty    int      `json:"difficulty"`
	TimeToLearn   int      `json:"time_to_learn"`
	Trending      bool     `json:"trending"`
	MarketDemand  float64  `json:"market_demand"`
}

type ExtractionMetadata struct {
	SourceType         string  `json:"source_type"`
	TotalSkillsFound   int     `json:"total_skills_found"`
	ConfidenceThreshold float64 `json:"confidence_threshold"`
	ExtractionDate     string  `json:"extraction_date"`
	ProcessingNotes    string  `json:"processing_notes"`
}

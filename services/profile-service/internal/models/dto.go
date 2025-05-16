package models

type ProfileDTO struct {
	PersonalInfo          *PersonalInfo           `json:"personalInfo,omitempty"`
	ContactInfo           *ContactInfo            `json:"contactInfo,omitempty"`
	EducationalBackground []EducationalBackground `json:"educationalBackground,omitempty"`
	PrivacySettings       *PrivacySettings        `json:"privacySettings,omitempty"`
}

type ProfileCompletenessResponse struct {
	Completeness       float64  `json:"completeness"`
	MissingFields      []string `json:"missingFields,omitempty"`
	RecommendedActions []string `json:"recommendedActions,omitempty"`
}

type CreateProfileRequest struct {
	UserID       string       `json:"userId" binding:"required"`
	PersonalInfo PersonalInfo `json:"personalInfo" binding:"required"`
	ContactInfo  ContactInfo  `json:"contactInfo" binding:"required"`
}

type UpdateProfileRequest struct {
	ProfileDTO ProfileDTO `json:"profile" binding:"required"`
}

type ProfileSearchQuery struct {
	Name        string `form:"name"`
	Institution string `form:"institution"`
	Field       string `form:"field"`
	Country     string `form:"country"`
	Page        int    `form:"page,default=1"`
	PageSize    int    `form:"pageSize,default=20"`
}

type ProfileSearchResult struct {
	Profiles    []Profile `json:"profiles"`
	TotalCount  int64     `json:"totalCount"`
	PageCount   int       `json:"pageCount"`
	CurrentPage int       `json:"currentPage"`
}

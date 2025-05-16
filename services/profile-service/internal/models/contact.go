package models

type Address struct {
	Street     string `json:"street,omitempty" bson:"street,omitempty"`
	City       string `json:"city,omitempty" bson:"city,omitempty"`
	State      string `json:"state,omitempty" bson:"state,omitempty"`
	Country    string `json:"country,omitempty" bson:"country,omitempty"`
	PostalCode string `json:"postalCode,omitempty" bson:"postalCode,omitempty"`
}

type ContactInfo struct {
	Email              string            `json:"email" bson:"email"`
	Phone              string            `json:"phone,omitempty" bson:"phone,omitempty"`
	AlternativeEmail   string            `json:"alternativeEmail,omitempty" bson:"alternativeEmail,omitempty"`
	Address            *Address          `json:"address,omitempty" bson:"address,omitempty"`
	SocialMediaHandles map[string]string `json:"socialMediaHandles,omitempty" bson:"socialMediaHandles,omitempty"`
}

type PrivacySettings struct {
	ProfileVisibility     VisibilityLevel `json:"profileVisibility" bson:"profileVisibility"`
	ContactInfoVisibility VisibilityLevel `json:"contactInfoVisibility" bson:"contactInfoVisibility"`
	EducationVisibility   VisibilityLevel `json:"educationVisibility" bson:"educationVisibility"`
	ActivityVisibility    VisibilityLevel `json:"activityVisibility,omitempty" bson:"activityVisibility,omitempty"`
}

package models

type EducationalBackground struct {
	Institution string `json:"institution" bson:"institution"`
	Degree      string `json:"degree,omitempty" bson:"degree,omitempty"`
	Field       string `json:"field,omitempty" bson:"field,omitempty"`
	StartDate   int    `json:"startDate" bson:"startDate"`
	EndDate     int    `json:"endDate,omitempty" bson:"endDate,omitempty"`
	InProgress  bool   `json:"inProgress" bson:"inProgress"`
	Description string `json:"description,omitempty" bson:"description,omitempty"`
}

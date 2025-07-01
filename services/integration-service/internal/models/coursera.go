package models

import (
	"time"
)

// Coursera OAuth2 Token
type CourseraTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// Coursera User Profile
type CourseraProfile struct {
	ID        string `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Timezone  string `json:"timezone"`
	Locale    string `json:"locale"`
}

// Coursera Enrollment from API
type CourseraEnrollment struct {
	ID             string  `json:"id"`
	UserID         string  `json:"userId"`
	CourseSlug     string  `json:"courseSlug"`
	CourseName     string  `json:"courseName"`
	EnrollmentTime int64   `json:"enrollmentTime"`
	Grade          float64 `json:"grade,omitempty"`
	CompletionTime *int64  `json:"completionTime,omitempty"`
	CertificateURL string  `json:"certificateUrl,omitempty"`
	Role           string  `json:"role"` // "LEARNER", "MENTOR", etc.
	PaymentStatus  string  `json:"paymentStatus"`
	CourseProgress float64 `json:"courseProgress,omitempty"`
}

// Parsed Course Data for Skill Extraction
type ParsedCourseData struct {
	CourseID       string           `json:"course_id"`
	CourseName     string           `json:"course_name"`
	CourseSlug     string           `json:"course_slug"`
	IsCompleted    bool             `json:"is_completed"`
	CompletionDate *time.Time       `json:"completion_date,omitempty"`
	EnrollmentDate time.Time        `json:"enrollment_date"`
	Grade          float64          `json:"grade,omitempty"`
	Skills         []ExtractedSkill `json:"skills"`
	Confidence     float64          `json:"confidence"`
	Source         string           `json:"source"` // "coursera"
}

// Extracted Skill from Course
type ExtractedSkill struct {
	Name            string  `json:"name"`
	Confidence      float64 `json:"confidence"`
	Level           string  `json:"level"` // beginner/intermediate/advanced based on completion
	YearsExperience int     `json:"years_experience"`
	Source          string  `json:"source"`         // "coursera_course"
	SourceDetails   string  `json:"source_details"` // course name
}

// Course Skill Mapping Event
type CourseSkillEvent struct {
	EventType string          `json:"event_type"`
	UserID    string          `json:"user_id"`
	Source    string          `json:"source"`
	Timestamp string          `json:"timestamp"`
	Data      CourseSkillData `json:"data"`
}

type CourseSkillData struct {
	Courses         []ParsedCourseData `json:"courses"`
	ExtractedSkills []ExtractedSkill   `json:"extracted_skills"`
}

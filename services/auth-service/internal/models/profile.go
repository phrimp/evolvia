package models

// UserProfile represents a user's profile information
type UserProfile struct {
	UserID      string            `json:"user_id"`
	DisplayName string            `json:"display_name"`
	Avatar      string            `json:"avatar"`
	Bio         string            `json:"bio"`
	Preferences map[string]string `json:"preferences"`
	CreatedAt   int64             `json:"created_at"`
	UpdatedAt   int64             `json:"updated_at"`
}

// UserWithProfile combines UserAuth and UserProfile information
type UserWithProfile struct {
	User    *UserAuth    `json:"user"`
	Profile *UserProfile `json:"profile"`
}

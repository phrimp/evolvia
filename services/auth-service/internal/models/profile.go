package models

type UserProfile struct {
	DisplayName string   `bson:"displayName" json:"displayName"`
	Role        []string `bson:"-" json:"roles"`
}

type UserWithProfile struct {
	User    *UserAuth    `json:"user"`
	Profile *UserProfile `json:"profile"`
}

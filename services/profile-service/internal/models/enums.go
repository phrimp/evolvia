package models

type Gender string

const (
	GenderMale           Gender = "male"
	GenderFemale         Gender = "female"
	GenderOther          Gender = "other"
	GenderPreferNotToSay Gender = "prefer-not-to-say"
)

type VisibilityLevel string

const (
	VisibilityPublic      VisibilityLevel = "public"
	VisibilityPrivate     VisibilityLevel = "private"
	VisibilityConnections VisibilityLevel = "connections"
)

package models

type Session struct {
	Token          string
	UserAgent      string
	IPAddress      string
	IsValid        bool
	CreatedAt      int
	LastActivityAt int
	Device         Device
	Location       Location
}

type Device struct {
	Type    string
	OS      string
	Browser string
}

type Location struct {
	Country string
	Region  string
	City    string
}

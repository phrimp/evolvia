package middleware

const (
	// Profile permissions
	ReadProfilePermission    = "read:profile"
	ReadAllProfilePermission = "read:profile:all"
	WriteProfilePermission   = "write:profile"
	UpdateProfilePermission  = "update:profile"
	DeleteProfilePermission  = "delete:profile"
	SearchProfilePermission  = "search:profile"

	// Profile analytics permissions
	ReadProfileAnalyticsPermission = "read:profile:analytics"

	// Admin permissions (for backward compatibility)
	AdminPermission   = "admin"
	ManagerPermission = "manager"
)

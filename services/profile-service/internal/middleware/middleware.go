package middleware

const (
	// Profile permissions
	ReadProfilePermission    = "read:profile"
	ReadAllProfilePermission = "read:profile:all"
	WriteProfilePermission   = "write:profile"
	UpdateProfilePermission  = "update:profile"
	DeleteProfilePermission  = "delete:profile"

	// Admin permissions (for backward compatibility)
	AdminPermission   = "admin"
	ManagerPermission = "manager"
)

package middleware

const (
	// object permissions
	ReadobjectPermission    = "read:object"
	ReadAllobjectPermission = "read:object:all"
	WriteobjectPermission   = "write:object"
	UpdateobjectPermission  = "update:object"
	DeleteobjectPermission  = "delete:object"
	SearchobjectPermission  = "search:object"

	// object analytics permissions
	ReadobjectAnalyticsPermission = "read:object:analytics"

	// Admin permissions (for backward compatibility)
	AdminPermission   = "admin"
	ManagerPermission = "manager"
)

package middleware

const (
	// Skill permissions
	ReadSkillPermission    = "read:skill"
	ReadAllSkillPermission = "read:skill:all"
	WriteSkillPermission   = "write:skill"
	UpdateSkillPermission  = "update:skill"
	DeleteSkillPermission  = "delete:skill"
	AdminSkillPermission   = "admin:skill"

	// User Skill permissions
	ReadUserSkillPermission    = "read:user-skill"
	ReadAllUserSkillPermission = "read:user-skill:all"
	WriteUserSkillPermission   = "write:user-skill"
	UpdateUserSkillPermission  = "update:user-skill"
	DeleteUserSkillPermission  = "delete:user-skill"
	EndorseUserSkillPermission = "endorse:user-skill"
	VerifyUserSkillPermission  = "verify:user-skill"
	AdminUserSkillPermission   = "admin:user-skill"

	// Knowledge analytics permissions
	ReadKnowledgeAnalyticsPermission = "read:knowledge:analytics"
	ReadKnowledgeDashboardPermission = "read:knowledge:dashboard"

	// Admin permissions (for backward compatibility)
	AdminPermission   = "admin"
	ManagerPermission = "manager"
)

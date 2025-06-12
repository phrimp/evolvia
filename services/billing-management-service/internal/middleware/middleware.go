package middleware

const (
	// Plan permissions
	ReadPlanPermission    = "read:plan"
	ReadAllPlanPermission = "read:plan:all"
	WritePlanPermission   = "write:plan"
	UpdatePlanPermission  = "update:plan"
	DeletePlanPermission  = "delete:plan"

	// Subscription permissions
	ReadSubscriptionPermission    = "read:subscription"
	ReadAllSubscriptionPermission = "read:subscription:all"
	WriteSubscriptionPermission   = "write:subscription"
	UpdateSubscriptionPermission  = "update:subscription"
	DeleteSubscriptionPermission  = "delete:subscription"
	ManageSubscriptionPermission  = "manage:subscription"

	// Billing dashboard and analytics permissions
	ReadBillingDashboardPermission     = "read:billing:dashboard"
	ReadBillingAnalyticsPermission     = "read:billing:analytics"
	ProcessBillingOperationsPermission = "process:billing:operations"

	// Admin permissions (for backward compatibility)
	AdminPermission   = "admin"
	ManagerPermission = "manager"
)

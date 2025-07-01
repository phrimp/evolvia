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

	// Advanced analytics permissions
	ReadAdvancedAnalyticsPermission  = "read:billing:analytics:advanced"
	ReadFinancialAnalyticsPermission = "read:billing:analytics:financial"
	ReadUserAnalyticsPermission      = "read:billing:analytics:users"
	ReadRevenueAnalyticsPermission   = "read:billing:analytics:revenue"

	// Export permissions
	ExportBillingDataPermission = "export:billing:data"
	ExportAnalyticsPermission   = "export:billing:analytics"

	// Admin permissions (for backward compatibility)
	AdminPermission   = "admin"
	ManagerPermission = "manager"

	// Super admin permissions
	SuperAdminPermission   = "super:admin"
	BillingAdminPermission = "billing:admin"
)

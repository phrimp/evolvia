package repository

import (
	"auth_service/internal/models"
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var available_permissions map[string]*models.Permission = make(map[string]*models.Permission)

var default_permissions []*models.Permission = []*models.Permission{
	// Basic CRUD permissions
	{Name: "read", Description: "Basic Read Permission for own resources", Category: "user", IsSystem: false},
	{Name: "write", Description: "Basic Write Permission for own resources", Category: "user", IsSystem: false},
	{Name: "update", Description: "Basic Update Permission for own resources", Category: "user", IsSystem: false},
	{Name: "delete", Description: "Basic Delete Permission for own resources", Category: "user", IsSystem: false},

	// Admin permissions (access to all resources)
	{Name: "read:admin", Description: "Read Permission for All resources", Category: "admin", IsSystem: false},
	{Name: "write:admin", Description: "Write Permission for All resources", Category: "admin", IsSystem: false},
	{Name: "update:admin", Description: "Update Permission for All resources", Category: "admin", IsSystem: false},
	{Name: "delete:admin", Description: "Delete Permission for All resources", Category: "admin", IsSystem: false},

	// Plan-specific permissions
	{Name: "read:plan", Description: "Read Permission for Plan resources", Category: "plan", IsSystem: false},
	{Name: "read:plan:all", Description: "Read All Permission for Plan resources", Category: "plan", IsSystem: false},
	{Name: "write:plan", Description: "Write Permission for Plan resources", Category: "plan", IsSystem: false},
	{Name: "update:plan", Description: "Update Permission for Plan resources", Category: "plan", IsSystem: false},
	{Name: "delete:plan", Description: "Delete Permission for Plan resources", Category: "plan", IsSystem: false},

	// Subscription-specific permissions
	{Name: "read:subscription", Description: "Read Permission for own Subscription resources", Category: "subscription", IsSystem: false},
	{Name: "read:subscription:all", Description: "Read All Permission for Subscription resources", Category: "subscription", IsSystem: false},
	{Name: "write:subscription", Description: "Write Permission for Subscription resources", Category: "subscription", IsSystem: false},
	{Name: "update:subscription", Description: "Update Permission for Subscription resources", Category: "subscription", IsSystem: false},
	{Name: "delete:subscription", Description: "Delete Permission for Subscription resources", Category: "subscription", IsSystem: false},
	{Name: "manage:subscription", Description: "Manage Permission for Subscription resources (suspend, reactivate, etc.)", Category: "subscription", IsSystem: false},

	// Billing dashboard and analytics permissions
	{Name: "read:billing:dashboard", Description: "Read Permission for Billing Dashboard", Category: "billing", IsSystem: false},
	{Name: "read:billing:analytics", Description: "Read Permission for Billing Analytics", Category: "billing", IsSystem: false},
	{Name: "process:billing:operations", Description: "Process Billing Operations (trial expirations, etc.)", Category: "billing", IsSystem: false},

	// Advanced analytics permissions
	{Name: "read:billing:analytics:advanced", Description: "Read Permission for Advanced Billing Analytics", Category: "billing", IsSystem: false},
	{Name: "read:billing:analytics:financial", Description: "Read Permission for Financial Analytics (MRR, ARR, LTV)", Category: "billing", IsSystem: false},
	{Name: "read:billing:analytics:users", Description: "Read Permission for User Analytics and Metrics", Category: "billing", IsSystem: false},
	{Name: "read:billing:analytics:revenue", Description: "Read Permission for Revenue Analytics and Forecasting", Category: "billing", IsSystem: false},
	{Name: "read:billing:analytics:trends", Description: "Read Permission for Subscription Trends and Historical Data", Category: "billing", IsSystem: false},
	{Name: "read:billing:analytics:cohort", Description: "Read Permission for Cohort Analysis and Customer Behavior", Category: "billing", IsSystem: false},
	{Name: "read:billing:analytics:geographic", Description: "Read Permission for Geographic Analytics", Category: "billing", IsSystem: false},
	{Name: "read:billing:analytics:performance", Description: "Read Permission for Performance Metrics and KPIs", Category: "billing", IsSystem: false},

	// Export and reporting permissions
	{Name: "export:billing:data", Description: "Export Billing Data to various formats", Category: "billing", IsSystem: false},
	{Name: "export:billing:analytics", Description: "Export Analytics Reports and Data", Category: "billing", IsSystem: false},
	{Name: "export:billing:financial", Description: "Export Financial Reports", Category: "billing", IsSystem: false},

	// Billing administration permissions
	{Name: "billing:admin", Description: "Full Administrative Access to Billing System", Category: "billing", IsSystem: false},
	{Name: "billing:manager", Description: "Manager-level Access to Billing Operations", Category: "billing", IsSystem: false},
	{Name: "billing:analyst", Description: "Analyst Access to Billing Data and Reports", Category: "billing", IsSystem: false},

	// User management permissions
	{Name: "read:user", Description: "Read Permission for User resources", Category: "user_management", IsSystem: false},
	{Name: "read:user:all", Description: "Read All Permission for User resources", Category: "user_management", IsSystem: false},
	{Name: "update:user", Description: "Update Permission for User resources", Category: "user_management", IsSystem: false},
	{Name: "delete:user", Description: "Delete Permission for User resources", Category: "user_management", IsSystem: false},
	{Name: "manage:user", Description: "Manage Permission for User resources (activate, deactivate, etc.)", Category: "user_management", IsSystem: false},

	// Profile-specific permissions
	{Name: "read:profile", Description: "Read Permission for Profile resources", Category: "profile", IsSystem: false},
	{Name: "read:profile:all", Description: "Read All Permission for Profile resources", Category: "profile", IsSystem: false},
	{Name: "write:profile", Description: "Write Permission for Profile resources", Category: "profile", IsSystem: false},
	{Name: "update:profile", Description: "Update Permission for Profile resources", Category: "profile", IsSystem: false},
	{Name: "delete:profile", Description: "Delete Permission for Profile resources", Category: "profile", IsSystem: false},
	{Name: "search:profile", Description: "Search Permission for Profile resources", Category: "profile", IsSystem: false},
	{Name: "read:profile:analytics", Description: "Read Permission for Profile Analytics", Category: "profile", IsSystem: false},

	// Object service specific permissions
	{Name: "read:object", Description: "Read Permission for Object resources", Category: "object", IsSystem: false},
	{Name: "read:object:all", Description: "Read All Permission for Object resources", Category: "object", IsSystem: false},
	{Name: "write:object", Description: "Write Permission for Object resources", Category: "object", IsSystem: false},
	{Name: "update:object", Description: "Update Permission for Object resources", Category: "object", IsSystem: false},
	{Name: "delete:object", Description: "Delete Permission for Object resources", Category: "object", IsSystem: false},
	{Name: "search:object", Description: "Search Permission for Object resources", Category: "object", IsSystem: false},

	// Skill management permissions
	{Name: "read:skill", Description: "Read Permission for Skills", Category: "skill", IsSystem: false},
	{Name: "read:skill:all", Description: "Read All Permission for Skills", Category: "skill", IsSystem: false},
	{Name: "write:skill", Description: "Write Permission for Skills", Category: "skill", IsSystem: false},
	{Name: "update:skill", Description: "Update Permission for Skills", Category: "skill", IsSystem: false},
	{Name: "delete:skill", Description: "Delete Permission for Skills", Category: "skill", IsSystem: false},
	{Name: "admin:skill", Description: "Admin Permission for Skills (full access)", Category: "skill", IsSystem: false},

	// User skill management permissions
	{Name: "read:user-skill", Description: "Read Permission for User Skills", Category: "user-skill", IsSystem: false},
	{Name: "read:user-skill:all", Description: "Read All Permission for User Skills", Category: "user-skill", IsSystem: false},
	{Name: "write:user-skill", Description: "Write Permission for User Skills", Category: "user-skill", IsSystem: false},
	{Name: "update:user-skill", Description: "Update Permission for User Skills", Category: "user-skill", IsSystem: false},
	{Name: "delete:user-skill", Description: "Delete Permission for User Skills", Category: "user-skill", IsSystem: false},
	{Name: "endorse:user-skill", Description: "Endorse User Skills", Category: "user-skill", IsSystem: false},
	{Name: "verify:user-skill", Description: "Verify User Skills", Category: "user-skill", IsSystem: false},
	{Name: "admin:user-skill", Description: "Admin Permission for User Skills (full access)", Category: "user-skill", IsSystem: false},

	// Knowledge analytics permissions
	{Name: "read:knowledge:analytics", Description: "Read Permission for Knowledge Analytics", Category: "knowledge", IsSystem: false},
	{Name: "read:knowledge:dashboard", Description: "Read Permission for Knowledge Dashboard", Category: "knowledge", IsSystem: false},

	// Input service permissions (for PowerPoint processing)
	{Name: "upload:powerpoint", Description: "Upload PowerPoint files for skill extraction", Category: "input", IsSystem: false},
	{Name: "process:input", Description: "Process input files for analysis", Category: "input", IsSystem: false},

	// Legacy admin/manager permissions for backward compatibility
	{Name: "admin", Description: "Global Admin Permission", Category: "admin", IsSystem: false},
	{Name: "manager", Description: "Global Manager Permission", Category: "manager", IsSystem: false},
	{Name: "super:admin", Description: "Super Administrator Permission - highest level access", Category: "admin", IsSystem: false},
}

type PermissionRepository struct {
	collection *mongo.Collection
	mu         *sync.Mutex
}

func NewPermissionRepository(db *mongo.Database) *PermissionRepository {
	return &PermissionRepository{
		collection: db.Collection("Permission"),
		mu:         &sync.Mutex{},
	}
}

func (pr *PermissionRepository) InitDefaultPermissions(ctx context.Context) error {
	count, err := pr.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("error checking permissions collection: %w", err)
	}

	if count > 0 {
		log.Printf("Permissions collection already contains %d documents, checking for new permissions...", count)

		// Check if we need to add any new permissions
		existingPermissions, err := pr.FindAll(ctx)
		if err != nil {
			return fmt.Errorf("error fetching existing permissions: %w", err)
		}

		existingPermissionNames := make(map[string]bool)
		for _, perm := range existingPermissions {
			existingPermissionNames[perm.Name] = true
		}

		// Add any missing permissions
		var addedCount int
		currentTime := int(time.Now().Unix())

		for _, permission := range default_permissions {
			if !existingPermissionNames[permission.Name] {
				// This is a new permission that doesn't exist yet
				newPermission := *permission // Create a copy
				newPermission.ID = bson.NewObjectID()
				newPermission.CreatedAt = currentTime
				newPermission.UpdatedAt = currentTime

				_, err := pr.collection.InsertOne(ctx, &newPermission)
				if err != nil {
					return fmt.Errorf("failed to insert new permission %s: %w", newPermission.Name, err)
				}

				pr.mu.Lock()
				available_permissions[newPermission.Name] = &newPermission
				pr.mu.Unlock()

				log.Printf("Added new permission: %s", newPermission.Name)
				addedCount++
			}
		}

		if addedCount > 0 {
			log.Printf("Successfully added %d new permissions", addedCount)
		} else {
			log.Printf("No new permissions to add")
		}

		return nil
	}

	log.Printf("Initializing default permissions...")
	currentTime := int(time.Now().Unix())

	for _, permission := range default_permissions {
		// Set creation time if not set
		if permission.CreatedAt == 0 {
			permission.CreatedAt = currentTime
		}

		if permission.UpdatedAt == 0 {
			permission.UpdatedAt = currentTime
		}

		if permission.ID.IsZero() {
			permission.ID = bson.NewObjectID()
		}

		_, err := pr.collection.InsertOne(ctx, permission)
		if err != nil {
			return fmt.Errorf("failed to insert default permission %s: %w", permission.Name, err)
		}

		pr.mu.Lock()
		available_permissions[permission.Name] = permission
		pr.mu.Unlock()

		log.Printf("Added default permission: %s", permission.Name)
	}

	log.Printf("Successfully initialized %d default permissions", len(default_permissions))
	return nil
}

func (pr *PermissionRepository) New(ctx context.Context, p *models.Permission) (*models.Permission, error) {
	if p.ID.IsZero() {
		p.ID = bson.NewObjectID()
	}

	currentTime := int(time.Now().Unix())
	if p.CreatedAt == 0 {
		p.CreatedAt = currentTime
	}

	_, err := pr.collection.InsertOne(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("failed to insert permission: %w", err)
	}
	return p, nil
}

func (pr *PermissionRepository) CollectPermissions(ctx context.Context) {
	available_permissions_slice, err := pr.FindAll(ctx)
	if err != nil {
		log.Printf("Error when collecting permissions: %s", err)
		return
	}
	for _, p := range available_permissions_slice {
		pr.mu.Lock()
		available_permissions[p.Name] = p
		pr.mu.Unlock()
	}
	log.Printf("Permission Collected: %d permissions loaded", len(available_permissions))
}

func (pr *PermissionRepository) FindAvailablePermission(ctx context.Context, name string) (*models.Permission, error) {
	pr.mu.Lock()
	found_permission, ok := available_permissions[name]
	pr.mu.Unlock()

	if ok {
		return found_permission, nil
	}

	var permission models.Permission
	err := pr.collection.FindOne(ctx, bson.M{"name": name}).Decode(&permission)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("permission '%s' not found", name)
		}
		return nil, fmt.Errorf("error finding permission in database: %w", err)
	}

	pr.mu.Lock()
	available_permissions[name] = &permission
	pr.mu.Unlock()

	return &permission, nil
}

func (pr *PermissionRepository) AddtoAvailablePermissions(p *models.Permission) {
	log.Printf("Adding New Available Permission: %v", p)
	pr.mu.Lock()
	available_permissions[p.Name] = p
	pr.mu.Unlock()
	log.Printf("New Available Permission Added")
}

func (pr *PermissionRepository) FindAll(ctx context.Context) ([]*models.Permission, error) {
	cursor, err := pr.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var permissions []*models.Permission
	if err = cursor.All(ctx, &permissions); err != nil {
		return nil, err
	}
	return permissions, nil
}

// Helper method to get permissions by category
func (pr *PermissionRepository) FindByCategory(ctx context.Context, category string) ([]*models.Permission, error) {
	cursor, err := pr.collection.Find(ctx, bson.M{"category": category})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var permissions []*models.Permission
	if err = cursor.All(ctx, &permissions); err != nil {
		return nil, err
	}
	return permissions, nil
}

// Helper method to get all billing-related permissions
func (pr *PermissionRepository) FindBillingPermissions(ctx context.Context) ([]*models.Permission, error) {
	return pr.FindByCategory(ctx, "billing")
}

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

	// Manager permissions (elevated but not full admin)
	{Name: "read:manager", Description: "Read Permission for managed resources", Category: "manager", IsSystem: false},
	{Name: "write:manager", Description: "Write Permission for managed resources", Category: "manager", IsSystem: false},
	{Name: "update:manager", Description: "Update Permission for managed resources", Category: "manager", IsSystem: false},
	{Name: "delete:manager", Description: "Delete Permission for managed resources", Category: "manager", IsSystem: false},

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
		log.Printf("Permissions collection already contains %d documents, skipping default initialization", count)
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
	log.Printf("Permission Collected: %v", available_permissions)
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
	pr.mu.Unlock() // Fixed: was double-locking instead of unlocking
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

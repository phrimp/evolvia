package repository

import (
	"auth_service/internal/models"
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var available_permissions map[string]*models.Permission = make(map[string]*models.Permission)

var default_permissions []*models.Permission = []*models.Permission{
	{Name: "read", Description: "TMP Read Permission for All resources", Category: "all", IsSystem: false},
	{Name: "write", Description: "TMP Write Permission for All resources", Category: "all", IsSystem: false},
	{Name: "update", Description: "TMP Update Permission for All resources", Category: "all", IsSystem: false},
	{Name: "delete", Description: "TMP Delete Permission for All resources", Category: "all", IsSystem: false},
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
			permission.ID = primitive.NewObjectID()
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
		p.ID = primitive.NewObjectID()
	}

	currentTime := int(time.Now().Unix())
	if p.CreatedAt == 0 {
		p.CreatedAt = currentTime
	}

	_, err := pr.collection.InsertOne(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("failed to insert session: %w", err)
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
	pr.mu.Lock()
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

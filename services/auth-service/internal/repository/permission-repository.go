package repository

import (
	"auth_service/internal/models"
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var available_permissions map[string]*models.Permission = make(map[string]*models.Permission)

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

func (pr *PermissionRepository) FindAvailablePermission(name string) (*models.Permission, error) {
	found_permission, ok := available_permissions[name]
	if !ok {
		return nil, fmt.Errorf("no available permission found")
	}
	return found_permission, nil
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

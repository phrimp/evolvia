package repository

import (
	"auth_service/internal/models"
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var availableRoles = make(map[string]*models.Role)

type RoleRepository struct {
	collection *mongo.Collection
	mu         *sync.Mutex
}

func NewRoleRepository(db *mongo.Database) *RoleRepository {
	return &RoleRepository{
		collection: db.Collection("Role"),
		mu:         &sync.Mutex{},
	}
}

func (r *RoleRepository) Create(ctx context.Context, role *models.Role) (*models.Role, error) {
	existing, err := r.FindByName(ctx, role.Name)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("error checking existing role: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("role with name '%s' already exists", role.Name)
	}

	if role.ID.IsZero() {
		role.ID = primitive.NewObjectID()
	}

	currentTime := int(time.Now().Unix())
	if role.CreatedAt == 0 {
		role.CreatedAt = currentTime
	}
	if role.UpdatedAt == 0 {
		role.UpdatedAt = currentTime
	}

	_, err = r.collection.InsertOne(ctx, role)
	if err != nil {
		return nil, fmt.Errorf("failed to insert role: %w", err)
	}

	r.mu.Lock()
	availableRoles[role.Name] = role
	r.mu.Unlock()

	return role, nil
}

func (r *RoleRepository) Update(ctx context.Context, role *models.Role) error {
	role.UpdatedAt = int(time.Now().Unix())

	filter := bson.M{"_id": role.ID}
	_, err := r.collection.UpdateOne(ctx, filter, bson.M{"$set": role})
	if err != nil {
		return fmt.Errorf("failed to update role: %w", err)
	}

	r.mu.Lock()
	availableRoles[role.Name] = role
	r.mu.Unlock()

	return nil
}

func (r *RoleRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	role, err := r.FindByID(ctx, id)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": id}
	_, err = r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete role: %w", err)
	}

	r.mu.Lock()
	delete(availableRoles, role.Name)
	r.mu.Unlock()

	return nil
}

func (r *RoleRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.Role, error) {
	var role models.Role
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&role)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("role with ID %s not found", id.Hex())
		}
		return nil, err
	}
	return &role, nil
}

func (r *RoleRepository) FindByName(ctx context.Context, name string) (*models.Role, error) {
	r.mu.Lock()
	if role, ok := availableRoles[name]; ok {
		r.mu.Unlock()
		return role, nil
	}
	r.mu.Unlock()

	var role models.Role
	err := r.collection.FindOne(ctx, bson.M{"name": name}).Decode(&role)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("role with name '%s' not found", name)
		}
		return nil, err
	}

	r.mu.Lock()
	availableRoles[name] = &role
	r.mu.Unlock()

	return &role, nil
}

func (r *RoleRepository) FindAll(ctx context.Context, page, limit int) ([]*models.Role, error) {
	opts := options.Find()
	opts.SetSort(bson.M{"name": 1})
	if page > 0 && limit > 0 {
		opts.SetSkip(int64((page - 1) * limit))
		opts.SetLimit(int64(limit))
	}

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var roles []*models.Role
	if err = cursor.All(ctx, &roles); err != nil {
		return nil, err
	}

	return roles, nil
}

func (r *RoleRepository) CollectRoles(ctx context.Context) error {
	roles, err := r.FindAll(ctx, 0, 0)
	if err != nil {
		return fmt.Errorf("error collecting roles: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for k := range availableRoles {
		delete(availableRoles, k)
	}

	for _, role := range roles {
		availableRoles[role.Name] = role
	}

	log.Printf("Loaded %d roles into cache", len(roles))
	return nil
}

func (r *RoleRepository) GetAvailableRoles() map[string]*models.Role {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make(map[string]*models.Role, len(availableRoles))
	maps.Copy(result, availableRoles)

	return result
}

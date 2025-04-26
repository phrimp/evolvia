package repository

import (
	"auth_service/internal/models"
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type UserRoleRepository struct {
	collection *mongo.Collection
}

func NewUserRoleRepository(db *mongo.Database) *UserRoleRepository {
	return &UserRoleRepository{
		collection: db.Collection("UserRole"),
	}
}

func (r *UserRoleRepository) Create(ctx context.Context, userRole *models.UserRole) (*models.UserRole, error) {
	filter := bson.M{
		"userId": userRole.UserID,
		"roleId": userRole.RoleID,
	}

	if !userRole.ScopeID.IsZero() {
		filter["scopeId"] = userRole.ScopeID
		filter["scopeType"] = userRole.ScopeType
	} else if userRole.ScopeType != "" {
		filter["scopeType"] = userRole.ScopeType
	}

	var existingUserRole models.UserRole
	err := r.collection.FindOne(ctx, filter).Decode(&existingUserRole)
	if err == nil {
		return nil, fmt.Errorf("user already has this role in this scope")
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("error checking existing user role: %w", err)
	}

	if userRole.ID.IsZero() {
		userRole.ID = primitive.NewObjectID()
	}

	currentTime := int(time.Now().Unix())
	if userRole.AssignedAt == 0 {
		userRole.AssignedAt = currentTime
	}

	if !userRole.IsActive {
		userRole.IsActive = true
	}

	_, err = r.collection.InsertOne(ctx, userRole)
	if err != nil {
		return nil, fmt.Errorf("failed to insert user role: %w", err)
	}

	return userRole, nil
}

func (r *UserRoleRepository) Update(ctx context.Context, userRole *models.UserRole) error {
	filter := bson.M{"_id": userRole.ID}
	_, err := r.collection.UpdateOne(ctx, filter, bson.M{"$set": userRole})
	if err != nil {
		return fmt.Errorf("failed to update user role: %w", err)
	}
	return nil
}

func (r *UserRoleRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	filter := bson.M{"_id": id}
	_, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete user role: %w", err)
	}
	return nil
}

func (r *UserRoleRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.UserRole, error) {
	var userRole models.UserRole
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&userRole)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("user role with ID %s not found", id.Hex())
		}
		return nil, err
	}
	return &userRole, nil
}

func (r *UserRoleRepository) FindByUserID(ctx context.Context, userID primitive.ObjectID) ([]*models.UserRole, error) {
	filter := bson.M{"userId": userID, "isActive": true}

	currentTime := int(time.Now().Unix())
	expiredFilter := bson.M{
		"userId":    userID,
		"isActive":  true,
		"expiresAt": bson.M{"$lt": currentTime, "$ne": 0},
	}

	update := bson.M{"$set": bson.M{"isActive": false}}
	_, err := r.collection.UpdateMany(ctx, expiredFilter, update)
	if err != nil {
		return nil, fmt.Errorf("error deactivating expired roles: %w", err)
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var userRoles []*models.UserRole
	if err = cursor.All(ctx, &userRoles); err != nil {
		return nil, err
	}

	return userRoles, nil
}

func (r *UserRoleRepository) FindByUserIDAndScope(ctx context.Context, userID primitive.ObjectID, scopeType string, scopeID primitive.ObjectID) ([]*models.UserRole, error) {
	filter := bson.M{
		"userId":   userID,
		"isActive": true,
	}

	if scopeID.IsZero() {
		if scopeType != "" {
			filter["scopeType"] = scopeType
		}
	} else {
		filter["scopeId"] = scopeID
		if scopeType != "" {
			filter["scopeType"] = scopeType
		}
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var userRoles []*models.UserRole
	if err = cursor.All(ctx, &userRoles); err != nil {
		return nil, err
	}

	return userRoles, nil
}

func (r *UserRoleRepository) FindUsersWithRole(ctx context.Context, roleID primitive.ObjectID, page, limit int) ([]primitive.ObjectID, error) {
	filter := bson.M{"roleId": roleID, "isActive": true}

	opts := options.Find()
	if page > 0 && limit > 0 {
		opts.SetSkip(int64((page - 1) * limit))
		opts.SetLimit(int64(limit))
	}

	opts.SetProjection(bson.M{"userId": 1, "_id": 0})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		UserID primitive.ObjectID `bson:"userId"`
	}
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	userIDs := make([]primitive.ObjectID, len(results))
	for i, result := range results {
		userIDs[i] = result.UserID
	}

	return userIDs, nil
}

func (r *UserRoleRepository) Deactivate(ctx context.Context, id primitive.ObjectID) error {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{"isActive": false}}
	_, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to deactivate user role: %w", err)
	}
	return nil
}

func (r *UserRoleRepository) DeactivateUserRoles(ctx context.Context, userID primitive.ObjectID) error {
	filter := bson.M{"userId": userID, "isActive": true}
	update := bson.M{"$set": bson.M{"isActive": false}}
	_, err := r.collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to deactivate user roles: %w", err)
	}
	return nil
}

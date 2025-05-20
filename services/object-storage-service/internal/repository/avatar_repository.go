package repository

import (
	"context"
	"log"
	"object-storage-service/internal/database/mongo"
	"object-storage-service/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	mongodb "go.mongodb.org/mongo-driver/v2/mongo"
)

type AvatarRepository struct {
	collection *mongodb.Collection
}

// NewAvatarRepository creates a new avatar repository
func NewAvatarRepository() *AvatarRepository {
	return &AvatarRepository{
		collection: mongo.GetCollection("avatars"),
	}
}

// Create saves a new avatar
func (r *AvatarRepository) Create(ctx context.Context, avatar *models.Avatar) (*models.Avatar, error) {
	avatar.CreatedAt = time.Now()
	avatar.UpdatedAt = time.Now()

	result, err := r.collection.InsertOne(ctx, avatar)
	if err != nil {
		log.Printf("Error creating avatar: %v", err)
		return nil, err
	}

	avatar.ID = result.InsertedID.(bson.ObjectID)
	return avatar, nil
}

// GetByID retrieves an avatar by ID
func (r *AvatarRepository) GetByID(ctx context.Context, id string) (*models.Avatar, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	var avatar models.Avatar
	err = r.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&avatar)
	if err != nil {
		if err == mongodb.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &avatar, nil
}

// GetByUserID retrieves all avatars for a user
func (r *AvatarRepository) GetByUserID(ctx context.Context, userID string) ([]*models.Avatar, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"userId": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var avatars []*models.Avatar
	if err = cursor.All(ctx, &avatars); err != nil {
		return nil, err
	}

	return avatars, nil
}

// GetDefaultAvatar retrieves the default avatar for a user
func (r *AvatarRepository) GetDefaultAvatar(ctx context.Context, userID string) (*models.Avatar, error) {
	var avatar models.Avatar
	err := r.collection.FindOne(ctx, bson.M{"userId": userID, "isDefault": true}).Decode(&avatar)
	if err != nil {
		if err == mongodb.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return &avatar, nil
}

// SetDefault sets an avatar as the default
func (r *AvatarRepository) SetDefault(ctx context.Context, id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	// Get the avatar to get the user ID
	var avatar models.Avatar
	err = r.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&avatar)
	if err != nil {
		return err
	}

	// Update all avatars for this user to not be default
	_, err = r.collection.UpdateMany(
		ctx,
		bson.M{"userId": avatar.UserID, "isDefault": true},
		bson.M{"$set": bson.M{"isDefault": false}},
	)
	if err != nil {
		return err
	}

	// Now set the selected avatar as default
	_, err = r.collection.UpdateOne(
		ctx,
		bson.M{"_id": objectID},
		bson.M{"$set": bson.M{"isDefault": true, "updatedAt": time.Now()}},
	)
	return err
}

// Delete deletes an avatar by ID
func (r *AvatarRepository) Delete(ctx context.Context, id string) error {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	// Check if this is the default avatar
	var avatar models.Avatar
	err = r.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&avatar)
	if err != nil {
		return err
	}

	// Delete the avatar
	_, err = r.collection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		return err
	}

	return nil
}

package reporsitory

import (
	"context"
	"fmt"
	"profile-service/internal/models"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type ProfileRepository struct {
	collection *mongo.Collection
	mu         *sync.Mutex
}

func NewProfileRepository(db *mongo.Database) *ProfileRepository {
	return &ProfileRepository{
		collection: db.Collection("Profile"),
		mu:         &sync.Mutex{},
	}
}

func (p *ProfileRepository) New(ctx context.Context, profile *models.Profile) (*models.Profile, error) {
	if profile.ID.IsZero() {
		profile.ID = bson.NewObjectID()
	}

	currentTime := int(time.Now().Unix())
	if profile.Metadata.CreatedAt == 0 {
		profile.Metadata.CreatedAt = currentTime
	}

	_, err := p.collection.InsertOne(ctx, profile)
	if err != nil {
		return nil, fmt.Errorf("failed to insert session: %w", err)
	}
	return profile, nil
}

func (r *ProfileRepository) FindAll(ctx context.Context, page, limit int) ([]*models.Profile, error) {
	opts := options.Find()
	opts.SetSort(bson.M{"createdAt": -1})
	opts.SetSkip(int64((page - 1) * limit))
	opts.SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []*models.Profile
	if err = cursor.All(ctx, &users); err != nil {
		return nil, err
	}

	return users, nil
}

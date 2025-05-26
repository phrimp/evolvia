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

func (r *ProfileRepository) New(ctx context.Context, profile *models.Profile) (*models.Profile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if profile.ID.IsZero() {
		profile.ID = bson.NewObjectID()
	}

	currentTime := int(time.Now().Unix())
	if profile.Metadata.CreatedAt == 0 {
		profile.Metadata.CreatedAt = currentTime
	}
	profile.Metadata.UpdatedAt = currentTime

	_, err := r.collection.InsertOne(ctx, profile)
	if err != nil {
		return nil, fmt.Errorf("failed to insert profile: %w", err)
	}
	return profile, nil
}

func (r *ProfileRepository) FindByID(ctx context.Context, id bson.ObjectID) (*models.Profile, error) {
	var profile models.Profile
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&profile)
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *ProfileRepository) FindByUserID(ctx context.Context, userID string) (*models.Profile, error) {
	var profile models.Profile
	err := r.collection.FindOne(ctx, bson.M{"userId": userID}).Decode(&profile)
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *ProfileRepository) Update(ctx context.Context, id bson.ObjectID, profile *models.Profile) (*models.Profile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	profile.Metadata.UpdatedAt = int(time.Now().Unix())

	filter := bson.M{"_id": id}
	update := bson.M{"$set": profile}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var updatedProfile models.Profile
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updatedProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	return &updatedProfile, nil
}

func (r *ProfileRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete profile: %w", err)
	}

	if result.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}

func (r *ProfileRepository) FindAll(ctx context.Context, page, limit int) ([]*models.Profile, error) {
	opts := options.Find()
	opts.SetSort(bson.M{"metadata.createdAt": -1})
	opts.SetSkip(int64((page - 1) * limit))
	opts.SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var profiles []*models.Profile
	if err = cursor.All(ctx, &profiles); err != nil {
		return nil, err
	}

	return profiles, nil
}

func (r *ProfileRepository) Search(ctx context.Context, query *models.ProfileSearchQuery) ([]*models.Profile, int64, error) {
	filter := bson.M{}

	if query.Name != "" {
		filter["$or"] = []bson.M{
			{"personalInfo.firstName": bson.M{"$regex": query.Name, "$options": "i"}},
			{"personalInfo.lastName": bson.M{"$regex": query.Name, "$options": "i"}},
			{"personalInfo.displayName": bson.M{"$regex": query.Name, "$options": "i"}},
		}
	}

	if query.Institution != "" {
		filter["educationalBackground.institution"] = bson.M{"$regex": query.Institution, "$options": "i"}
	}

	if query.Field != "" {
		filter["educationalBackground.field"] = bson.M{"$regex": query.Field, "$options": "i"}
	}

	if query.Country != "" {
		filter["contactInfo.address.country"] = bson.M{"$regex": query.Country, "$options": "i"}
	}

	totalCount, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count documents: %w", err)
	}

	opts := options.Find()
	opts.SetSort(bson.M{"metadata.createdAt": -1})
	opts.SetSkip(int64((query.Page - 1) * query.PageSize))
	opts.SetLimit(int64(query.PageSize))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search profiles: %w", err)
	}
	defer cursor.Close(ctx)

	var profiles []*models.Profile
	if err = cursor.All(ctx, &profiles); err != nil {
		return nil, 0, fmt.Errorf("failed to decode profiles: %w", err)
	}

	return profiles, totalCount, nil
}

func (r *ProfileRepository) GetByUserIDs(ctx context.Context, userIDs []string) ([]*models.Profile, error) {
	filter := bson.M{"userId": bson.M{"$in": userIDs}}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find profiles: %w", err)
	}
	defer cursor.Close(ctx)

	var profiles []*models.Profile
	if err = cursor.All(ctx, &profiles); err != nil {
		return nil, fmt.Errorf("failed to decode profiles: %w", err)
	}

	return profiles, nil
}

func (r *ProfileRepository) UpdateCompleteness(ctx context.Context, userID string, completeness float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	filter := bson.M{"userId": userID}
	update := bson.M{
		"$set": bson.M{
			"profileCompleteness": completeness,
			"metadata.updatedAt":  int(time.Now().Unix()),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update completeness: %w", err)
	}

	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}

func (r *ProfileRepository) CountProfiles(ctx context.Context) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("failed to count profiles: %w", err)
	}
	return count, nil
}

func (r *ProfileRepository) GetRecentProfiles(ctx context.Context, limit int) ([]*models.Profile, error) {
	opts := options.Find()
	opts.SetSort(bson.M{"metadata.createdAt": -1})
	opts.SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find recent profiles: %w", err)
	}
	defer cursor.Close(ctx)

	var profiles []*models.Profile
	if err = cursor.All(ctx, &profiles); err != nil {
		return nil, fmt.Errorf("failed to decode profiles: %w", err)
	}

	return profiles, nil
}

func (r *ProfileRepository) CreateIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "userId", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "personalInfo.firstName", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "personalInfo.lastName", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "contactInfo.email", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "educationalBackground.institution", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "metadata.createdAt", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "metadata.updatedAt", Value: -1}},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}

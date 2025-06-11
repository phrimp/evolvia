package repository

import (
	"billing-management-service/internal/models"
	"context"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type SubscriptionRepository struct {
	collection *mongo.Collection
	mu         *sync.Mutex
}

func NewSubscriptionRepository(db *mongo.Database) *SubscriptionRepository {
	return &SubscriptionRepository{
		collection: db.Collection("subscriptions"),
		mu:         &sync.Mutex{},
	}
}

func (r *SubscriptionRepository) New(ctx context.Context, subscription *models.Subscription) (*models.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if subscription.ID.IsZero() {
		subscription.ID = bson.NewObjectID()
	}

	currentTime := time.Now().Unix()
	if subscription.Metadata.CreatedAt == 0 {
		subscription.Metadata.CreatedAt = currentTime
	}
	subscription.Metadata.UpdatedAt = currentTime

	_, err := r.collection.InsertOne(ctx, subscription)
	if err != nil {
		return nil, fmt.Errorf("failed to insert subscription: %w", err)
	}
	return subscription, nil
}

func (r *SubscriptionRepository) FindByID(ctx context.Context, id bson.ObjectID) (*models.Subscription, error) {
	var subscription models.Subscription
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&subscription)
	if err != nil {
		return nil, err
	}
	return &subscription, nil
}

func (r *SubscriptionRepository) FindByUserID(ctx context.Context, userID string) (*models.Subscription, error) {
	var subscription models.Subscription
	err := r.collection.FindOne(ctx, bson.M{"userId": userID}).Decode(&subscription)
	if err != nil {
		return nil, err
	}
	return &subscription, nil
}

func (r *SubscriptionRepository) FindActiveByUserID(ctx context.Context, userID string) (*models.Subscription, error) {
	filter := bson.M{
		"userId": userID,
		"status": bson.M{"$in": []models.SubscriptionStatus{
			models.SubscriptionStatusActive,
			models.SubscriptionStatusTrial,
		}},
	}

	var subscription models.Subscription
	err := r.collection.FindOne(ctx, filter).Decode(&subscription)
	if err != nil {
		return nil, err
	}
	return &subscription, nil
}

func (r *SubscriptionRepository) Update(ctx context.Context, id bson.ObjectID, subscription *models.Subscription) (*models.Subscription, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	subscription.Metadata.UpdatedAt = time.Now().Unix()

	filter := bson.M{"_id": id}
	update := bson.M{"$set": subscription}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var updatedSubscription models.Subscription
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updatedSubscription)
	if err != nil {
		return nil, fmt.Errorf("failed to update subscription: %w", err)
	}

	return &updatedSubscription, nil
}

func (r *SubscriptionRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}

	if result.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}

func (r *SubscriptionRepository) Search(ctx context.Context, query *models.SubscriptionSearchQuery) ([]*models.Subscription, int64, error) {
	filter := bson.M{}

	if query.UserID != "" {
		filter["userId"] = query.UserID
	}

	if query.Status != "" {
		filter["status"] = query.Status
	}

	if query.PlanType != "" {
		// This requires joining with plans collection or storing planType in subscription
		// For now, we'll skip this filter or you can implement an aggregation pipeline
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
		return nil, 0, fmt.Errorf("failed to search subscriptions: %w", err)
	}
	defer cursor.Close(ctx)

	var subscriptions []*models.Subscription
	if err = cursor.All(ctx, &subscriptions); err != nil {
		return nil, 0, fmt.Errorf("failed to decode subscriptions: %w", err)
	}

	return subscriptions, totalCount, nil
}

func (r *SubscriptionRepository) FindByStatus(ctx context.Context, status models.SubscriptionStatus, page, limit int) ([]*models.Subscription, error) {
	filter := bson.M{"status": status}

	opts := options.Find()
	opts.SetSort(bson.M{"metadata.createdAt": -1})
	opts.SetSkip(int64((page - 1) * limit))
	opts.SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find subscriptions by status: %w", err)
	}
	defer cursor.Close(ctx)

	var subscriptions []*models.Subscription
	if err = cursor.All(ctx, &subscriptions); err != nil {
		return nil, fmt.Errorf("failed to decode subscriptions: %w", err)
	}

	return subscriptions, nil
}

func (r *SubscriptionRepository) FindByPlanID(ctx context.Context, planID bson.ObjectID, page, limit int) ([]*models.Subscription, error) {
	filter := bson.M{"planId": planID}

	opts := options.Find()
	opts.SetSort(bson.M{"metadata.createdAt": -1})
	opts.SetSkip(int64((page - 1) * limit))
	opts.SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find subscriptions by plan: %w", err)
	}
	defer cursor.Close(ctx)

	var subscriptions []*models.Subscription
	if err = cursor.All(ctx, &subscriptions); err != nil {
		return nil, fmt.Errorf("failed to decode subscriptions: %w", err)
	}

	return subscriptions, nil
}

func (r *SubscriptionRepository) FindExpiringSubscriptions(ctx context.Context, beforeTimestamp int64) ([]*models.Subscription, error) {
	filter := bson.M{
		"status":          models.SubscriptionStatusActive,
		"nextBillingDate": bson.M{"$lte": beforeTimestamp},
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find expiring subscriptions: %w", err)
	}
	defer cursor.Close(ctx)

	var subscriptions []*models.Subscription
	if err = cursor.All(ctx, &subscriptions); err != nil {
		return nil, fmt.Errorf("failed to decode subscriptions: %w", err)
	}

	return subscriptions, nil
}

func (r *SubscriptionRepository) FindTrialEndingSubscriptions(ctx context.Context, beforeTimestamp int64) ([]*models.Subscription, error) {
	filter := bson.M{
		"status":       models.SubscriptionStatusTrial,
		"trialEndDate": bson.M{"$lte": beforeTimestamp},
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find trial ending subscriptions: %w", err)
	}
	defer cursor.Close(ctx)

	var subscriptions []*models.Subscription
	if err = cursor.All(ctx, &subscriptions); err != nil {
		return nil, fmt.Errorf("failed to decode subscriptions: %w", err)
	}

	return subscriptions, nil
}

func (r *SubscriptionRepository) UpdateStatus(ctx context.Context, id bson.ObjectID, status models.SubscriptionStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"status":             status,
			"metadata.updatedAt": time.Now().Unix(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update subscription status: %w", err)
	}

	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}

func (r *SubscriptionRepository) CancelSubscription(ctx context.Context, id bson.ObjectID, reason string, immediate bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	currentTime := time.Now().Unix()

	update := bson.M{
		"$set": bson.M{
			"status":             models.SubscriptionStatusCanceled,
			"canceledAt":         currentTime,
			"cancelReason":       reason,
			"autoRenew":          false,
			"metadata.updatedAt": currentTime,
		},
	}

	if immediate {
		update["$set"].(bson.M)["endDate"] = currentTime
	}

	filter := bson.M{"_id": id}
	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to cancel subscription: %w", err)
	}

	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}

func (r *SubscriptionRepository) GetSubscriptionStats(ctx context.Context) (*models.BillingDashboard, error) {
	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id":   "$status",
				"count": bson.M{"$sum": 1},
			},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription stats: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode stats: %w", err)
	}

	dashboard := &models.BillingDashboard{}

	for _, result := range results {
		status := result["_id"].(string)
		count := result["count"].(int64)

		switch models.SubscriptionStatus(status) {
		case models.SubscriptionStatusActive:
			dashboard.ActiveSubscriptions = count
		case models.SubscriptionStatusTrial:
			dashboard.TrialSubscriptions = count
		}
	}

	return dashboard, nil
}

func (r *SubscriptionRepository) CountSubscriptions(ctx context.Context) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("failed to count subscriptions: %w", err)
	}
	return count, nil
}

func (r *SubscriptionRepository) CountActiveSubscriptions(ctx context.Context) (int64, error) {
	filter := bson.M{
		"status": bson.M{"$in": []models.SubscriptionStatus{
			models.SubscriptionStatusActive,
			models.SubscriptionStatusTrial,
		}},
	}

	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to count active subscriptions: %w", err)
	}
	return count, nil
}

func (r *SubscriptionRepository) GetRecentSubscriptions(ctx context.Context, limit int) ([]*models.Subscription, error) {
	opts := options.Find()
	opts.SetSort(bson.M{"metadata.createdAt": -1})
	opts.SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find recent subscriptions: %w", err)
	}
	defer cursor.Close(ctx)

	var subscriptions []*models.Subscription
	if err = cursor.All(ctx, &subscriptions); err != nil {
		return nil, fmt.Errorf("failed to decode subscriptions: %w", err)
	}

	return subscriptions, nil
}

func (r *SubscriptionRepository) CreateIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "userId", Value: 1}},
			Options: options.Index().SetUnique(false), // User can have multiple subscriptions (historical)
		},
		{
			Keys: bson.D{{Key: "planId", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "nextBillingDate", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "trialEndDate", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "endDate", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "metadata.createdAt", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "metadata.updatedAt", Value: -1}},
		},
		{
			Keys: bson.D{
				{Key: "userId", Value: 1},
				{Key: "status", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "nextBillingDate", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "trialEndDate", Value: 1},
			},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create subscription indexes: %w", err)
	}

	return nil
}

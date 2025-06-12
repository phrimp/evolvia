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

type PlanRepository struct {
	collection *mongo.Collection
	mu         *sync.Mutex
}

func NewPlanRepository(db *mongo.Database) *PlanRepository {
	return &PlanRepository{
		collection: db.Collection("plans"),
		mu:         &sync.Mutex{},
	}
}

func (r *PlanRepository) New(ctx context.Context, plan *models.Plan) (*models.Plan, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if plan.ID.IsZero() {
		plan.ID = bson.NewObjectID()
	}

	currentTime := time.Now().Unix()
	if plan.Metadata.CreatedAt == 0 {
		plan.Metadata.CreatedAt = currentTime
	}
	plan.Metadata.UpdatedAt = currentTime

	_, err := r.collection.InsertOne(ctx, plan)
	if err != nil {
		return nil, fmt.Errorf("failed to insert plan: %w", err)
	}
	return plan, nil
}

func (r *PlanRepository) FindByID(ctx context.Context, id bson.ObjectID) (*models.Plan, error) {
	var plan models.Plan
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&plan)
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

func (r *PlanRepository) FindByPlanType(ctx context.Context, planType models.PlanType) ([]*models.Plan, error) {
	filter := bson.M{"planType": planType, "isActive": true}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find plans by type: %w", err)
	}
	defer cursor.Close(ctx)

	var plans []*models.Plan
	if err = cursor.All(ctx, &plans); err != nil {
		return nil, fmt.Errorf("failed to decode plans: %w", err)
	}

	return plans, nil
}

func (r *PlanRepository) Update(ctx context.Context, id bson.ObjectID, plan *models.Plan) (*models.Plan, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	plan.Metadata.UpdatedAt = time.Now().Unix()

	filter := bson.M{"_id": id}
	update := bson.M{"$set": plan}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var updatedPlan models.Plan
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updatedPlan)
	if err != nil {
		return nil, fmt.Errorf("failed to update plan: %w", err)
	}

	return &updatedPlan, nil
}

func (r *PlanRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete plan: %w", err)
	}

	if result.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}

func (r *PlanRepository) FindAll(ctx context.Context, page, limit int) ([]*models.Plan, error) {
	opts := options.Find()
	opts.SetSort(bson.M{"metadata.createdAt": -1})
	opts.SetSkip(int64((page - 1) * limit))
	opts.SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find plans: %w", err)
	}
	defer cursor.Close(ctx)

	var plans []*models.Plan
	if err = cursor.All(ctx, &plans); err != nil {
		return nil, fmt.Errorf("failed to decode plans: %w", err)
	}

	return plans, nil
}

func (r *PlanRepository) FindActivePlans(ctx context.Context, page, limit int) ([]*models.Plan, error) {
	filter := bson.M{"isActive": true}

	opts := options.Find()
	opts.SetSort(bson.M{"price": 1}) // Sort by price ascending
	opts.SetSkip(int64((page - 1) * limit))
	opts.SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find active plans: %w", err)
	}
	defer cursor.Close(ctx)

	var plans []*models.Plan
	if err = cursor.All(ctx, &plans); err != nil {
		return nil, fmt.Errorf("failed to decode plans: %w", err)
	}

	return plans, nil
}

func (r *PlanRepository) GetByPlanTypes(ctx context.Context, planTypes []models.PlanType) ([]*models.Plan, error) {
	filter := bson.M{
		"planType": bson.M{"$in": planTypes},
		"isActive": true,
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find plans by types: %w", err)
	}
	defer cursor.Close(ctx)

	var plans []*models.Plan
	if err = cursor.All(ctx, &plans); err != nil {
		return nil, fmt.Errorf("failed to decode plans: %w", err)
	}

	return plans, nil
}

func (r *PlanRepository) UpdateStatus(ctx context.Context, id bson.ObjectID, isActive bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"isActive":           isActive,
			"metadata.updatedAt": time.Now().Unix(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update plan status: %w", err)
	}

	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}

func (r *PlanRepository) CountPlans(ctx context.Context) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("failed to count plans: %w", err)
	}
	return count, nil
}

func (r *PlanRepository) CountActivePlans(ctx context.Context) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"isActive": true})
	if err != nil {
		return 0, fmt.Errorf("failed to count active plans: %w", err)
	}
	return count, nil
}

func (r *PlanRepository) CreateIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "planType", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "isActive", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "price", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "billingCycle", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "currency", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "metadata.createdAt", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "metadata.updatedAt", Value: -1}},
		},
		{
			Keys: bson.D{
				{Key: "planType", Value: 1},
				{Key: "isActive", Value: 1},
			},
		},
	}

	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create plan indexes: %w", err)
	}

	return nil
}

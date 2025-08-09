package repository

import (
	"context"
	"quiz-service/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type SkillRepository struct {
	Col *mongo.Collection
}

func NewSkillRepository(db *mongo.Database) *SkillRepository {
	return &SkillRepository{Col: db.Collection("skills")}
}

func (r *SkillRepository) FindByID(ctx context.Context, id string) (*models.Skill, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var skill models.Skill
	err = r.Col.FindOne(ctx, bson.M{"_id": objID}).Decode(&skill)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &skill, nil
}

func (r *SkillRepository) FindActiveSkills(ctx context.Context) ([]models.Skill, error) {
	cur, err := r.Col.Find(ctx, bson.M{"is_active": true})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var skills []models.Skill
	for cur.Next(ctx) {
		var skill models.Skill
		if err := cur.Decode(&skill); err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}
	return skills, nil
}

func (r *SkillRepository) FindSkillsByCategory(ctx context.Context, categoryID string) ([]models.Skill, error) {
	objID, err := primitive.ObjectIDFromHex(categoryID)
	if err != nil {
		return nil, err
	}

	cur, err := r.Col.Find(ctx, bson.M{"category_id": objID, "is_active": true})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var skills []models.Skill
	for cur.Next(ctx) {
		var skill models.Skill
		if err := cur.Decode(&skill); err != nil {
			return nil, err
		}
		skills = append(skills, skill)
	}
	return skills, nil
}

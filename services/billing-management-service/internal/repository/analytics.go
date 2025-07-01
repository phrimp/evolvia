package repository

import (
	"billing-management-service/internal/models"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type AnalyticsRepository struct {
	subscriptionCollection *mongo.Collection
	planCollection         *mongo.Collection
}

func NewAnalyticsRepository(db *mongo.Database) *AnalyticsRepository {
	return &AnalyticsRepository{
		subscriptionCollection: db.Collection("subscriptions"),
		planCollection:         db.Collection("plans"),
	}
}

// GetUserMetrics returns user-related metrics
func (r *AnalyticsRepository) GetUserMetrics(ctx context.Context) (*models.UserMetrics, error) {
	pipeline := []bson.M{
		{
			"$facet": bson.M{
				"totalUsers": []bson.M{
					{"$group": bson.M{"_id": "$userId"}},
					{"$count": "count"},
				},
				"statusBreakdown": []bson.M{
					{"$group": bson.M{
						"_id":   "$status",
						"count": bson.M{"$sum": 1},
					}},
				},
			},
		},
	}

	cursor, err := r.subscriptionCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get user metrics: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode user metrics: %w", err)
	}

	metrics := &models.UserMetrics{}

	if len(results) > 0 {
		result := results[0]

		// Total users
		if totalUsers, ok := result["totalUsers"].([]interface{}); ok && len(totalUsers) > 0 {
			if userData, ok := totalUsers[0].(bson.M); ok {
				metrics.TotalUsers = userData["count"].(int64)
			}
		}

		// Status breakdown
		if statusData, ok := result["statusBreakdown"].([]interface{}); ok {
			for _, item := range statusData {
				if statusItem, ok := item.(bson.M); ok {
					status := statusItem["_id"].(string)
					count := statusItem["count"].(int64)

					switch models.SubscriptionStatus(status) {
					case models.SubscriptionStatusActive:
						metrics.ActiveSubscriptions = count
					case models.SubscriptionStatusTrial:
						metrics.TrialSubscriptions = count
					case models.SubscriptionStatusCanceled:
						metrics.CanceledSubscriptions = count
					case models.SubscriptionStatusSuspended:
						metrics.SuspendedSubscriptions = count
					case models.SubscriptionStatusPastDue:
						metrics.PastDueSubscriptions = count
					case models.SubscriptionStatusInactive:
						metrics.InactiveSubscriptions = count
					}
				}
			}
		}
	}

	// Calculate subscription percentage
	subscribedUsers := metrics.ActiveSubscriptions + metrics.TrialSubscriptions
	if metrics.TotalUsers > 0 {
		metrics.SubscriptionPercentage = float64(subscribedUsers) / float64(metrics.TotalUsers) * 100
	}

	return metrics, nil
}

// GetRevenueMetrics returns revenue-related metrics
func (r *AnalyticsRepository) GetRevenueMetrics(ctx context.Context) (*models.RevenueMetrics, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"status": bson.M{"$in": []string{"active", "trial"}},
			},
		},
		{
			"$lookup": bson.M{
				"from":         "plans",
				"localField":   "planId",
				"foreignField": "_id",
				"as":           "plan",
			},
		},
		{
			"$unwind": "$plan",
		},
		{
			"$group": bson.M{
				"_id":          nil,
				"totalRevenue": bson.M{"$sum": "$plan.price"},
				"monthlyRevenue": bson.M{
					"$sum": bson.M{
						"$cond": bson.M{
							"if":   bson.M{"$eq": []interface{}{"$plan.billingCycle", "monthly"}},
							"then": "$plan.price",
							"else": bson.M{"$divide": []interface{}{"$plan.price", 12}},
						},
					},
				},
				"yearlyRevenue": bson.M{
					"$sum": bson.M{
						"$cond": bson.M{
							"if":   bson.M{"$eq": []interface{}{"$plan.billingCycle", "yearly"}},
							"then": "$plan.price",
							"else": bson.M{"$multiply": []interface{}{"$plan.price", 12}},
						},
					},
				},
				"avgPrice":        bson.M{"$avg": "$plan.price"},
				"subscriberCount": bson.M{"$sum": 1},
			},
		},
	}

	cursor, err := r.subscriptionCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get revenue metrics: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode revenue metrics: %w", err)
	}

	metrics := &models.RevenueMetrics{}

	if len(results) > 0 {
		result := results[0]
		metrics.TotalRevenue = result["totalRevenue"].(float64)
		metrics.MonthlyRevenue = result["monthlyRevenue"].(float64)
		metrics.YearlyRevenue = result["yearlyRevenue"].(float64)
		metrics.AveragePrice = result["avgPrice"].(float64)
		subscriberCount := result["subscriberCount"].(int64)

		if subscriberCount > 0 {
			metrics.AverageRevenuePerUser = metrics.MonthlyRevenue / float64(subscriberCount)
		}
	}

	return metrics, nil
}

// GetSubscriptionTrends returns subscription trends over time
func (r *AnalyticsRepository) GetSubscriptionTrends(ctx context.Context, period string, limit int) (*models.SubscriptionTrends, error) {
	var groupBy bson.M

	switch period {
	case "hours":
		groupBy = bson.M{
			"year":  bson.M{"$year": bson.M{"$toDate": bson.M{"$multiply": []interface{}{"$metadata.createdAt", 1000}}}},
			"month": bson.M{"$month": bson.M{"$toDate": bson.M{"$multiply": []interface{}{"$metadata.createdAt", 1000}}}},
			"day":   bson.M{"$dayOfMonth": bson.M{"$toDate": bson.M{"$multiply": []interface{}{"$metadata.createdAt", 1000}}}},
			"hour":  bson.M{"$hour": bson.M{"$toDate": bson.M{"$multiply": []interface{}{"$metadata.createdAt", 1000}}}},
		}
	case "days":
		groupBy = bson.M{
			"year":  bson.M{"$year": bson.M{"$toDate": bson.M{"$multiply": []interface{}{"$metadata.createdAt", 1000}}}},
			"month": bson.M{"$month": bson.M{"$toDate": bson.M{"$multiply": []interface{}{"$metadata.createdAt", 1000}}}},
			"day":   bson.M{"$dayOfMonth": bson.M{"$toDate": bson.M{"$multiply": []interface{}{"$metadata.createdAt", 1000}}}},
		}
	case "weeks":
		groupBy = bson.M{
			"year": bson.M{"$year": bson.M{"$toDate": bson.M{"$multiply": []interface{}{"$metadata.createdAt", 1000}}}},
			"week": bson.M{"$week": bson.M{"$toDate": bson.M{"$multiply": []interface{}{"$metadata.createdAt", 1000}}}},
		}
	case "months":
		groupBy = bson.M{
			"year":  bson.M{"$year": bson.M{"$toDate": bson.M{"$multiply": []interface{}{"$metadata.createdAt", 1000}}}},
			"month": bson.M{"$month": bson.M{"$toDate": bson.M{"$multiply": []interface{}{"$metadata.createdAt", 1000}}}},
		}
	default:
		return nil, fmt.Errorf("invalid period: %s", period)
	}

	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id":   groupBy,
				"count": bson.M{"$sum": 1},
			},
		},
		{
			"$sort": bson.M{"_id": -1},
		},
		{
			"$limit": limit,
		},
	}

	cursor, err := r.subscriptionCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription trends: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode trends: %w", err)
	}

	trends := &models.SubscriptionTrends{
		Period: period,
		Data:   make([]models.TrendData, len(results)),
	}

	for i, result := range results {
		trends.Data[i] = models.TrendData{
			Period: result["_id"],
			Count:  result["count"].(int64),
		}
	}

	return trends, nil
}

// GetPlanPopularity returns plan popularity statistics
func (r *AnalyticsRepository) GetPlanPopularity(ctx context.Context) (*models.PlanPopularity, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"status": bson.M{"$in": []string{"active", "trial"}},
			},
		},
		{
			"$lookup": bson.M{
				"from":         "plans",
				"localField":   "planId",
				"foreignField": "_id",
				"as":           "plan",
			},
		},
		{
			"$unwind": "$plan",
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"planId":   "$planId",
					"planName": "$plan.name",
					"planType": "$plan.planType",
					"price":    "$plan.price",
				},
				"subscriberCount": bson.M{"$sum": 1},
				"revenue":         bson.M{"$sum": "$plan.price"},
			},
		},
		{
			"$sort": bson.M{"subscriberCount": -1},
		},
	}

	cursor, err := r.subscriptionCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get plan popularity: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode plan popularity: %w", err)
	}

	popularity := &models.PlanPopularity{
		Plans: make([]models.PlanStatistics, len(results)),
	}

	var totalSubscribers int64
	for _, result := range results {
		totalSubscribers += result["subscriberCount"].(int64)
	}

	for i, result := range results {
		planData := result["_id"].(bson.M)
		subscriberCount := result["subscriberCount"].(int64)

		percentage := float64(0)
		if totalSubscribers > 0 {
			percentage = float64(subscriberCount) / float64(totalSubscribers) * 100
		}

		popularity.Plans[i] = models.PlanStatistics{
			PlanID:          planData["planId"].(bson.ObjectID).Hex(),
			PlanName:        planData["planName"].(string),
			PlanType:        models.PlanType(planData["planType"].(string)),
			Price:           planData["price"].(float64),
			SubscriberCount: subscriberCount,
			Revenue:         result["revenue"].(float64),
			Percentage:      percentage,
		}
	}

	// Find most popular plan
	if len(popularity.Plans) > 0 {
		popularity.MostPopularPlan = popularity.Plans[0].PlanName
		popularity.TotalRevenue = popularity.Plans[0].Revenue
	}

	return popularity, nil
}

// GetRealTimeMetrics returns real-time metrics
func (r *AnalyticsRepository) GetRealTimeMetrics(ctx context.Context) (*models.RealTimeMetrics, error) {
	today := time.Now()
	startOfDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	startOfWeek := startOfDay.AddDate(0, 0, -int(today.Weekday()))

	pipeline := []bson.M{
		{
			"$facet": bson.M{
				"today": []bson.M{
					{
						"$match": bson.M{
							"metadata.createdAt": bson.M{
								"$gte": startOfDay.Unix(),
							},
						},
					},
					{"$count": "count"},
				},
				"thisWeek": []bson.M{
					{
						"$match": bson.M{
							"metadata.createdAt": bson.M{
								"$gte": startOfWeek.Unix(),
							},
						},
					},
					{"$count": "count"},
				},
				"activeNow": []bson.M{
					{
						"$match": bson.M{
							"status": bson.M{"$in": []string{"active", "trial"}},
						},
					},
					{"$count": "count"},
				},
			},
		},
	}

	cursor, err := r.subscriptionCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get real-time metrics: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode real-time metrics: %w", err)
	}

	metrics := &models.RealTimeMetrics{
		Timestamp: time.Now().Unix(),
	}

	if len(results) > 0 {
		result := results[0]

		if today, ok := result["today"].([]interface{}); ok && len(today) > 0 {
			if todayData, ok := today[0].(bson.M); ok {
				metrics.NewSubscriptionsToday = todayData["count"].(int64)
			}
		}

		if thisWeek, ok := result["thisWeek"].([]interface{}); ok && len(thisWeek) > 0 {
			if weekData, ok := thisWeek[0].(bson.M); ok {
				metrics.NewSubscriptionsThisWeek = weekData["count"].(int64)
			}
		}

		if activeNow, ok := result["activeNow"].([]interface{}); ok && len(activeNow) > 0 {
			if activeData, ok := activeNow[0].(bson.M); ok {
				metrics.ActiveSubscriptions = activeData["count"].(int64)
			}
		}
	}

	return metrics, nil
}

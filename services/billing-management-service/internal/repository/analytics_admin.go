package repository

import (
	"billing-management-service/internal/models"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// GetAdminSubscriptionStats returns comprehensive admin subscription statistics
func (r *AnalyticsRepository) GetAdminSubscriptionStats(ctx context.Context) (*models.AdminSubscriptionStats, error) {
	today := time.Now()
	startOfDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	startOfWeek := startOfDay.AddDate(0, 0, -int(today.Weekday()))
	startOfMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())

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
				"newToday": []bson.M{
					{
						"$match": bson.M{
							"metadata.createdAt": bson.M{
								"$gte": startOfDay.Unix(),
							},
						},
					},
					{"$count": "count"},
				},
				"newThisWeek": []bson.M{
					{
						"$match": bson.M{
							"metadata.createdAt": bson.M{
								"$gte": startOfWeek.Unix(),
							},
						},
					},
					{"$count": "count"},
				},
				"newThisMonth": []bson.M{
					{
						"$match": bson.M{
							"metadata.createdAt": bson.M{
								"$gte": startOfMonth.Unix(),
							},
						},
					},
					{"$count": "count"},
				},
				"canceledToday": []bson.M{
					{
						"$match": bson.M{
							"status": "canceled",
							"canceledAt": bson.M{
								"$gte": startOfDay.Unix(),
							},
						},
					},
					{"$count": "count"},
				},
				"canceledThisWeek": []bson.M{
					{
						"$match": bson.M{
							"status": "canceled",
							"canceledAt": bson.M{
								"$gte": startOfWeek.Unix(),
							},
						},
					},
					{"$count": "count"},
				},
				"canceledThisMonth": []bson.M{
					{
						"$match": bson.M{
							"status": "canceled",
							"canceledAt": bson.M{
								"$gte": startOfMonth.Unix(),
							},
						},
					},
					{"$count": "count"},
				},
			},
		},
	}

	cursor, err := r.subscriptionCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin subscription stats: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode admin subscription stats: %w", err)
	}

	stats := &models.AdminSubscriptionStats{
		GeneratedAt: time.Now().Unix(),
	}

	var totalUsers int64
	if len(results) > 0 {
		result := results[0]

		// Total users count
		if totalUsersData, ok := result["totalUsers"].([]interface{}); ok && len(totalUsersData) > 0 {
			if userData, ok := totalUsersData[0].(bson.M); ok {
				totalUsers = toInt64(userData["count"])
			}
		}

		// Status breakdown
		if statusData, ok := result["statusBreakdown"].([]interface{}); ok {
			for _, item := range statusData {
				if statusItem, ok := item.(bson.M); ok {
					status := statusItem["_id"].(string)
					count := toInt64(statusItem["count"])

					switch models.SubscriptionStatus(status) {
					case models.SubscriptionStatusActive:
						stats.ActiveSubscriptions = count
					case models.SubscriptionStatusTrial:
						stats.TrialSubscriptions = count
					case models.SubscriptionStatusCanceled:
						stats.CanceledSubscriptions = count
					case models.SubscriptionStatusSuspended:
						stats.SuspendedSubscriptions = count
					case models.SubscriptionStatusPastDue:
						stats.PastDueSubscriptions = count
					case models.SubscriptionStatusInactive:
						stats.InactiveSubscriptions = count
					}
				}
			}
		}

		// Calculate total subscriptions
		stats.TotalSubscriptions = stats.ActiveSubscriptions + stats.TrialSubscriptions + 
			stats.CanceledSubscriptions + stats.SuspendedSubscriptions + 
			stats.PastDueSubscriptions + stats.InactiveSubscriptions

		// Time-based metrics
		if newToday, ok := result["newToday"].([]interface{}); ok && len(newToday) > 0 {
			if todayData, ok := newToday[0].(bson.M); ok {
				stats.NewSubscriptionsToday = toInt64(todayData["count"])
			}
		}

		if newThisWeek, ok := result["newThisWeek"].([]interface{}); ok && len(newThisWeek) > 0 {
			if weekData, ok := newThisWeek[0].(bson.M); ok {
				stats.NewSubscriptionsThisWeek = toInt64(weekData["count"])
			}
		}

		if newThisMonth, ok := result["newThisMonth"].([]interface{}); ok && len(newThisMonth) > 0 {
			if monthData, ok := newThisMonth[0].(bson.M); ok {
				stats.NewSubscriptionsThisMonth = toInt64(monthData["count"])
			}
		}

		if canceledToday, ok := result["canceledToday"].([]interface{}); ok && len(canceledToday) > 0 {
			if todayData, ok := canceledToday[0].(bson.M); ok {
				stats.CancelationsToday = toInt64(todayData["count"])
			}
		}

		if canceledThisWeek, ok := result["canceledThisWeek"].([]interface{}); ok && len(canceledThisWeek) > 0 {
			if weekData, ok := canceledThisWeek[0].(bson.M); ok {
				stats.CancelationsThisWeek = toInt64(weekData["count"])
			}
		}

		if canceledThisMonth, ok := result["canceledThisMonth"].([]interface{}); ok && len(canceledThisMonth) > 0 {
			if monthData, ok := canceledThisMonth[0].(bson.M); ok {
				stats.CancelationsThisMonth = toInt64(monthData["count"])
			}
		}
	}

	// Calculate rates
	if totalUsers > 0 {
		subscribedUsers := stats.ActiveSubscriptions + stats.TrialSubscriptions
		stats.SubscriptionRate = float64(subscribedUsers) / float64(totalUsers) * 100
	}

	if stats.TotalSubscriptions > 0 {
		stats.CancelRate = float64(stats.CanceledSubscriptions) / float64(stats.TotalSubscriptions) * 100
	}

	if stats.TrialSubscriptions+stats.ActiveSubscriptions > 0 {
		stats.TrialConversionRate = float64(stats.ActiveSubscriptions) / float64(stats.TrialSubscriptions+stats.ActiveSubscriptions) * 100
	}

	// Get revenue metrics
	revenueMetrics, err := r.GetRevenueMetrics(ctx)
	if err == nil {
		stats.TotalRevenue = revenueMetrics.TotalRevenue
		stats.MonthlyRevenue = revenueMetrics.MonthlyRevenue
		stats.AverageRevenuePerUser = revenueMetrics.AverageRevenuePerUser
	}

	// Calculate churn and growth rates (simplified)
	stats.ChurnRate = r.calculateMonthlyChurnRate(ctx)
	stats.GrowthRate = r.calculateMonthlyGrowthRate(ctx)

	return stats, nil
}

// GetCancellationAnalytics returns detailed cancellation analytics
func (r *AnalyticsRepository) GetCancellationAnalytics(ctx context.Context) (*models.CancellationAnalytics, error) {
	pipeline := []bson.M{
		{
			"$facet": bson.M{
				"totalCancellations": []bson.M{
					{
						"$match": bson.M{
							"status": "canceled",
						},
					},
					{"$count": "count"},
				},
				"totalSubscriptions": []bson.M{
					{"$count": "count"},
				},
				"byReason": []bson.M{
					{
						"$match": bson.M{
							"status": "canceled",
							"cancelReason": bson.M{"$ne": ""},
						},
					},
					{
						"$group": bson.M{
							"_id":   "$cancelReason",
							"count": bson.M{"$sum": 1},
						},
					},
					{"$sort": bson.M{"count": -1}},
				},
				"byPlan": []bson.M{
					{
						"$match": bson.M{
							"status": "canceled",
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
								"planName": "$plan.name",
								"planType": "$plan.planType",
							},
							"count": bson.M{"$sum": 1},
						},
					},
					{"$sort": bson.M{"count": -1}},
				},
				"subscriptionLifetime": []bson.M{
					{
						"$match": bson.M{
							"status":     "canceled",
							"canceledAt": bson.M{"$ne": nil},
							"startDate":  bson.M{"$ne": nil},
						},
					},
					{
						"$project": bson.M{
							"lifetime": bson.M{
								"$divide": []interface{}{
									bson.M{"$subtract": []interface{}{"$canceledAt", "$startDate"}},
									86400, // Convert seconds to days
								},
							},
						},
					},
					{
						"$group": bson.M{
							"_id":             nil,
							"averageLifetime": bson.M{"$avg": "$lifetime"},
						},
					},
				},
			},
		},
	}

	cursor, err := r.subscriptionCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get cancellation analytics: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode cancellation analytics: %w", err)
	}

	analytics := &models.CancellationAnalytics{
		CancelationsByReason: make([]models.CancellationByReason, 0),
		CancelationsByPlan:   make([]models.CancellationByPlan, 0),
		CancelationTrends:    make([]models.CancellationTrendData, 0),
	}

	if len(results) > 0 {
		result := results[0]

		// Total cancellations
		if totalCancellations, ok := result["totalCancellations"].([]interface{}); ok && len(totalCancellations) > 0 {
			if cancelData, ok := totalCancellations[0].(bson.M); ok {
				analytics.TotalCancellations = toInt64(cancelData["count"])
			}
		}

		// Total subscriptions for rate calculation
		var totalSubscriptions int64
		if totalSubs, ok := result["totalSubscriptions"].([]interface{}); ok && len(totalSubs) > 0 {
			if subsData, ok := totalSubs[0].(bson.M); ok {
				totalSubscriptions = toInt64(subsData["count"])
			}
		}

		// Calculate cancel rate
		if totalSubscriptions > 0 {
			analytics.CancelRate = float64(analytics.TotalCancellations) / float64(totalSubscriptions) * 100
		}

		// By Reason
		if byReason, ok := result["byReason"].([]interface{}); ok {
			for _, item := range byReason {
				if reasonItem, ok := item.(bson.M); ok {
					reason := reasonItem["_id"].(string)
					count := toInt64(reasonItem["count"])
					rate := float64(0)
					if analytics.TotalCancellations > 0 {
						rate = float64(count) / float64(analytics.TotalCancellations) * 100
					}

					analytics.CancelationsByReason = append(analytics.CancelationsByReason, models.CancellationByReason{
						Reason: reason,
						Count:  count,
						Rate:   rate,
					})
				}
			}
		}

		// By Plan - need to get total users per plan for rate calculation
		if byPlan, ok := result["byPlan"].([]interface{}); ok {
			for _, item := range byPlan {
				if planItem, ok := item.(bson.M); ok {
					planData := toBsonM(planItem["_id"])
					planName := getBsonString(planData, "planName")
					planType := getBsonString(planData, "planType")
					count := toInt64(planItem["count"])

					// Get total users for this plan to calculate rate
					totalUsersForPlan, err := r.getTotalUsersForPlan(ctx, planName)
					if err != nil {
						totalUsersForPlan = 0
					}

					rate := float64(0)
					if totalUsersForPlan > 0 {
						rate = float64(count) / float64(totalUsersForPlan) * 100
					}

					analytics.CancelationsByPlan = append(analytics.CancelationsByPlan, models.CancellationByPlan{
						PlanName:   planName,
						PlanType:   planType,
						Count:      count,
						Rate:       rate,
						TotalUsers: totalUsersForPlan,
					})
				}
			}
		}

		// Average subscription lifetime
		if lifetimeData, ok := result["subscriptionLifetime"].([]interface{}); ok && len(lifetimeData) > 0 {
			if ltData, ok := lifetimeData[0].(bson.M); ok {
				analytics.AverageSubscriptionLife = toFloat64(ltData["averageLifetime"])
			}
		}
	}

	return analytics, nil
}

// Helper methods for calculations
func (r *AnalyticsRepository) calculateMonthlyChurnRate(ctx context.Context) float64 {
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	
	pipeline := []bson.M{
		{
			"$facet": bson.M{
				"canceledThisMonth": []bson.M{
					{
						"$match": bson.M{
							"status": "canceled",
							"canceledAt": bson.M{
								"$gte": startOfMonth.Unix(),
							},
						},
					},
					{"$count": "count"},
				},
				"activeAtMonthStart": []bson.M{
					{
						"$match": bson.M{
							"startDate": bson.M{"$lt": startOfMonth.Unix()},
							"$or": []bson.M{
								{"status": "active"},
								{"status": "trial"},
								{
									"status": "canceled",
									"canceledAt": bson.M{"$gte": startOfMonth.Unix()},
								},
							},
						},
					},
					{"$count": "count"},
				},
			},
		},
	}

	cursor, _ := r.subscriptionCollection.Aggregate(ctx, pipeline)
	if cursor == nil {
		return 0
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if cursor.All(ctx, &results) != nil || len(results) == 0 {
		return 0
	}

	result := results[0]
	var canceledThisMonth, activeAtMonthStart int64

	if canceled, ok := result["canceledThisMonth"].([]interface{}); ok && len(canceled) > 0 {
		if cancelData, ok := canceled[0].(bson.M); ok {
			canceledThisMonth = toInt64(cancelData["count"])
		}
	}

	if active, ok := result["activeAtMonthStart"].([]interface{}); ok && len(active) > 0 {
		if activeData, ok := active[0].(bson.M); ok {
			activeAtMonthStart = toInt64(activeData["count"])
		}
	}

	if activeAtMonthStart > 0 {
		return float64(canceledThisMonth) / float64(activeAtMonthStart) * 100
	}
	return 0
}

func (r *AnalyticsRepository) calculateMonthlyGrowthRate(ctx context.Context) float64 {
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	
	pipeline := []bson.M{
		{
			"$facet": bson.M{
				"newThisMonth": []bson.M{
					{
						"$match": bson.M{
							"metadata.createdAt": bson.M{
								"$gte": startOfMonth.Unix(),
							},
						},
					},
					{"$count": "count"},
				},
				"totalBeforeMonth": []bson.M{
					{
						"$match": bson.M{
							"metadata.createdAt": bson.M{
								"$lt": startOfMonth.Unix(),
							},
						},
					},
					{"$count": "count"},
				},
			},
		},
	}

	cursor, _ := r.subscriptionCollection.Aggregate(ctx, pipeline)
	if cursor == nil {
		return 0
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if cursor.All(ctx, &results) != nil || len(results) == 0 {
		return 0
	}

	result := results[0]
	var newThisMonth, totalBeforeMonth int64

	if newSubs, ok := result["newThisMonth"].([]interface{}); ok && len(newSubs) > 0 {
		if newData, ok := newSubs[0].(bson.M); ok {
			newThisMonth = toInt64(newData["count"])
		}
	}

	if totalBefore, ok := result["totalBeforeMonth"].([]interface{}); ok && len(totalBefore) > 0 {
		if beforeData, ok := totalBefore[0].(bson.M); ok {
			totalBeforeMonth = toInt64(beforeData["count"])
		}
	}

	if totalBeforeMonth > 0 {
		return float64(newThisMonth) / float64(totalBeforeMonth) * 100
	}
	return 0
}

func (r *AnalyticsRepository) getTotalUsersForPlan(ctx context.Context, planName string) (int64, error) {
	pipeline := []bson.M{
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
			"$match": bson.M{
				"plan.name": planName,
			},
		},
		{
			"$count": "count",
		},
	}

	cursor, err := r.subscriptionCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil || len(results) == 0 {
		return 0, err
	}

	return toInt64(results[0]["count"]), nil
}
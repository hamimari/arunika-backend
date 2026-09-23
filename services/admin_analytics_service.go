package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"time"
)

type AdminAnalyticsService struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewAdminAnalyticsService(db *gorm.DB, redis *redis.Client) *AdminAnalyticsService {
	return &AdminAnalyticsService{db: db, redis: redis}
}

type DayCount struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

type PaymentMetrics struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
	// Total is the gross value of the orders in this group, in whole rupiah.
	// int64 rather than float64 because amount_idr is a BIGINT and IDR has
	// no fractional unit — there is nothing to round, and summing large
	// totals through a float would lose precision for no benefit.
	Total int64 `json:"total"`
}

// cached fetches or caches a value in Redis with the given TTL.
func (s *AdminAnalyticsService) cached(key string, ttl time.Duration, fn func() (interface{}, error)) (interface{}, error) {
	ctx := context.Background()
	raw, err := s.redis.Get(ctx, key).Result()
	if err == nil {
		var result interface{}
		_ = json.Unmarshal([]byte(raw), &result)
		return result, nil
	}
	result, err := fn()
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(result)
	s.redis.Set(ctx, key, string(b), ttl)
	return result, nil
}

// GetDAU returns daily unique user login counts for the last N days.
func (s *AdminAnalyticsService) GetDAU(days int) ([]DayCount, error) {
	key := fmt.Sprintf("analytics:dau:%d", days)
	result, err := s.cached(key, 60*time.Second, func() (interface{}, error) {
		var rows []DayCount
		err := s.db.Raw(`
			SELECT DATE(created_at)::text AS date, COUNT(DISTINCT user_id) AS count
			FROM user_sessions
			WHERE created_at >= NOW() - INTERVAL '? days'
			GROUP BY DATE(created_at)
			ORDER BY date ASC
		`, days).Scan(&rows).Error
		return rows, err
	})
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(result)
	var rows []DayCount
	_ = json.Unmarshal(b, &rows)
	return rows, nil
}

// GetNewUsers returns daily new user counts for the last N days.
func (s *AdminAnalyticsService) GetNewUsers(days int) ([]DayCount, error) {
	key := fmt.Sprintf("analytics:new-users:%d", days)
	result, err := s.cached(key, 60*time.Second, func() (interface{}, error) {
		var rows []DayCount
		err := s.db.Raw(`
			SELECT DATE(created_at)::text AS date, COUNT(*) AS count
			FROM parents
			WHERE created_at >= NOW() - INTERVAL '? days' AND is_deleted = false
			GROUP BY DATE(created_at)
			ORDER BY date ASC
		`, days).Scan(&rows).Error
		return rows, err
	})
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(result)
	var rows []DayCount
	_ = json.Unmarshal(b, &rows)
	return rows, nil
}

type FeatureStat struct {
	Feature string `json:"feature"`
	Count   int64  `json:"count"`
}

// GetPopularFeatures returns top N features by usage count.
func (s *AdminAnalyticsService) GetPopularFeatures() ([]FeatureStat, error) {
	key := "analytics:popular-features"
	result, err := s.cached(key, 60*time.Second, func() (interface{}, error) {
		var rows []FeatureStat
		err := s.db.Raw(`
			SELECT feature, count FROM (
				SELECT 'tracing'  AS feature, COUNT(*) AS count FROM tracing_progress
				UNION ALL
				SELECT 'counting' AS feature, COUNT(*) AS count FROM counting_progress
				UNION ALL
				SELECT 'fairy_tales' AS feature, COUNT(*) AS count FROM dongengs WHERE is_deleted = false
			) t
			ORDER BY count DESC
			LIMIT 5
		`).Scan(&rows).Error
		return rows, err
	})
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(result)
	var rows []FeatureStat
	_ = json.Unmarshal(b, &rows)
	return rows, nil
}

type SubscriptionStats struct {
	Total   int64 `json:"total"`
	Premium int64 `json:"premium"`
	Free    int64 `json:"free"`
}

// GetSubscriptionStats returns total, premium, and free user counts.
func (s *AdminAnalyticsService) GetSubscriptionStats() (*SubscriptionStats, error) {
	key := "analytics:subscription-stats"
	result, err := s.cached(key, 60*time.Second, func() (interface{}, error) {
		var stats SubscriptionStats
		err := s.db.Raw(`
			SELECT
				COUNT(*)                                    AS total,
				COUNT(*) FILTER (WHERE status = 'premium') AS premium,
				COUNT(*) FILTER (WHERE status = 'free')    AS free
			FROM user_subscriptions
		`).Scan(&stats).Error
		return &stats, err
	})
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(result)
	var stats SubscriptionStats
	_ = json.Unmarshal(b, &stats)
	return &stats, nil
}

// GetPaymentMetrics returns order counts and gross value grouped by order
// status, optionally restricted to orders created within [from, to].
//
// `Status` is an order status — PENDING, PAID, FAILED, EXPIRED or REFUNDED
// (migrations V36 and V51) — and `Total` is the sum of `amount_idr` for the
// orders in that group. Because REFUNDED replaces PAID when Google Play
// voids a purchase, the PAID group already excludes refunded orders and
// needs no further adjustment to be read as settled revenue.
//
// One row per order, so a user who buys five AR cards contributes five.
//
// This previously counted rows in `user_subscriptions` — a pre-V36 stub that
// reported at most one row per user, with `Total` hardcoded to 0, which the
// backoffice then displayed as payment figures. See F5.
func (s *AdminAnalyticsService) GetPaymentMetrics(from, to string) ([]PaymentMetrics, error) {
	key := fmt.Sprintf("analytics:payments:%s:%s", from, to)
	result, err := s.cached(key, 60*time.Second, func() (interface{}, error) {
		var rows []PaymentMetrics
		var query *gorm.DB
		if from != "" {
			query = s.db.Raw(`
				SELECT status, COUNT(*) AS count, COALESCE(SUM(amount_idr), 0) AS total
				FROM orders
				WHERE created_at >= ? AND created_at <= ?
				GROUP BY status
				ORDER BY count DESC
			`, from, to)
		} else {
			query = s.db.Raw(`
				SELECT status, COUNT(*) AS count, COALESCE(SUM(amount_idr), 0) AS total
				FROM orders
				GROUP BY status
				ORDER BY count DESC
			`)
		}
		err := query.Scan(&rows).Error
		return rows, err
	})
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(result)
	var rows []PaymentMetrics
	_ = json.Unmarshal(b, &rows)
	return rows, nil
}

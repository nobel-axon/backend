// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/axon-arena/axon-server/internal/config"
)

// RateLimiter implements a simple token bucket rate limiter.
type RateLimiter struct {
	mu              sync.RWMutex
	buckets         map[string]*tokenBucket
	requestsPerMin  int
	burstSize       int
	cleanupInterval time.Duration
	stopCh          chan struct{}
}

type tokenBucket struct {
	tokens     float64
	lastUpdate time.Time
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(cfg *config.RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		buckets:         make(map[string]*tokenBucket),
		requestsPerMin:  cfg.RequestsPerMin,
		burstSize:       cfg.BurstSize,
		cleanupInterval: time.Duration(cfg.CleanupIntervalS) * time.Second,
		stopCh:          make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Stop stops the rate limiter cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// Allow checks if a request from the given IP is allowed.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, exists := rl.buckets[ip]

	if !exists {
		// Create new bucket with full tokens
		rl.buckets[ip] = &tokenBucket{
			tokens:     float64(rl.burstSize) - 1, // -1 for this request
			lastUpdate: now,
		}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(bucket.lastUpdate).Seconds()
	tokensPerSecond := float64(rl.requestsPerMin) / 60.0
	bucket.tokens += elapsed * tokensPerSecond

	// Cap at burst size
	if bucket.tokens > float64(rl.burstSize) {
		bucket.tokens = float64(rl.burstSize)
	}

	bucket.lastUpdate = now

	// Check if we have tokens
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}

	return false
}

// cleanup removes stale buckets periodically.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-5 * time.Minute)
			for ip, bucket := range rl.buckets {
				if bucket.lastUpdate.Before(cutoff) {
					delete(rl.buckets, ip)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// RateLimit returns a Gin middleware that applies rate limiting.
func RateLimit(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !rl.Allow(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
			})
			return
		}

		c.Next()
	}
}

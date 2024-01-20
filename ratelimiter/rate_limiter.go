package ratelimiter

import (
	"sync"
	"time"
)

type RateLimiter struct {
	tokens        int
	tokenCapacity int
	tokenRate     int
	mu            sync.Mutex
}

func NewRateLimiter(tokenCapacity int, tokenRate int) *RateLimiter {
	return &RateLimiter{
		tokens:        tokenCapacity,
		tokenCapacity: tokenCapacity,
		tokenRate:     tokenRate,
	}
}

func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(lastRequestedTime)
	lastRequestedTime = now

	refillAmount := int(float64(elapsed.Nanoseconds()) / float64(time.Minute.Nanoseconds()) * float64(r.tokenRate))
	r.tokens = min(r.tokens+refillAmount, r.tokenCapacity)

	if r.tokens > 0 {
		r.tokens--
		return true
	}

	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var lastRequestedTime = time.Now()

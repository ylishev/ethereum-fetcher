package network

import (
	"context"
	"time"

	"ethereum-fetcher/cmd"
)

// RateLimiter contains the credits channel and the rate at which the credits are supplied
type RateLimiter struct {
	ctx     context.Context
	credits chan struct{}
	wait    time.Duration
}

// NewRateLimiter creates a new rate limiter with a specified number of credits and refill interval
func NewRateLimiter(ctx context.Context, maxCredits int, refillInterval time.Duration) *RateLimiter {
	if maxCredits <= 0 {
		maxCredits = cmd.DefaultNodeCredit
	}
	rl := &RateLimiter{
		ctx:     ctx,
		credits: make(chan struct{}, maxCredits),
		wait:    time.Second / time.Duration(maxCredits),
	}

	// prefill the channel with maxCredits to allow for immediate consumption
	for i := 0; i < maxCredits; i++ {
		rl.credits <- struct{}{}
	}

	// refill the credits periodically
	go func() {
		ticker := time.NewTicker(refillInterval)
		defer ticker.Stop()
		for range ticker.C {
			select {
			case rl.credits <- struct{}{}: // adds credits to the channel if it's not full
			case <-ctx.Done():
				return
			default:
				// channel is still full, nothing to do
			}
		}
	}()

	return rl
}

// Allow checks credits availability and allow to run if any
func (rl *RateLimiter) Allow() bool {
	select {
	case <-rl.credits: // consume one credit
		return true
	case <-rl.ctx.Done():
		return false
	default:
		// no credits available, request should be limited
		return false
	}
}

func (rl *RateLimiter) WaitDuration() time.Duration {
	return rl.wait
}

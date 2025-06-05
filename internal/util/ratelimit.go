package util

import (
	"context"
	"sync"
	"time"
)

type TokenBucket struct {
	capacity int
	tokens   int
	reset    time.Duration
	mux      sync.Mutex
	ticker   *time.Ticker
	done     chan struct{}
}

func NewBucket(capacity int, per time.Duration) *TokenBucket {
	return &TokenBucket{
		capacity: capacity,
		tokens:   capacity,
		reset:    per,
		done:     make(chan struct{}),
	}
}

func (b *TokenBucket) Take(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		b.mux.Lock()
		if b.tokens > 0 {
			b.tokens--
			b.mux.Unlock()
			return nil
		}
		b.mux.Unlock()
		
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (b *TokenBucket) refill() {
	b.ticker = time.NewTicker(b.reset)
	defer b.ticker.Stop()
	
	for {
		select {
		case <-b.ticker.C:
			b.mux.Lock()
			b.tokens = b.capacity
			b.mux.Unlock()
		case <-b.done:
			return
		}
	}
}

func (b *TokenBucket) Start() {
	go b.refill()
}

func (b *TokenBucket) Stop() {
	close(b.done)
}

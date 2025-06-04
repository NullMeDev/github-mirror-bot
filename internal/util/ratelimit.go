package util

import (
	"sync"
	"time"
)

type TokenBucket struct {
	capacity int
	tokens   int
	reset    time.Duration
	mux      sync.Mutex
}

func NewBucket(capacity int, per time.Duration) *TokenBucket {
	return &TokenBucket{capacity: capacity, tokens: capacity, reset: per}
}

func (b *TokenBucket) Take() {
	for {
		b.mux.Lock()
		if b.tokens > 0 {
			b.tokens--
			b.mux.Unlock()
			return
		}
		b.mux.Unlock()
		time.Sleep(500 * time.Millisecond)
	}
}

func (b *TokenBucket) refill() {
	ticker := time.NewTicker(b.reset)
	for range ticker.C {
		b.mux.Lock()
		b.tokens = b.capacity
		b.mux.Unlock()
	}
}

func (b *TokenBucket) Run() { go b.refill() }

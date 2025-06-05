package search

import (
	"context"
	"crypto/sha1"
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"
)

type Queue struct {
	pool *redis.Pool
}

func NewQueue(address, password string, db int) *Queue {
	return &Queue{
		pool: &redis.Pool{
			MaxIdle:     3,
			MaxActive:   10,
			IdleTimeout: 240 * time.Second,
			Dial: func() (redis.Conn, error) {
				c, err := redis.Dial("tcp", address)
				if err != nil {
					return nil, err
				}
				if password != "" {
					if _, err := c.Do("AUTH", password); err != nil {
						c.Close()
						return nil, err
					}
				}
				if db != 0 {
					if _, err := c.Do("SELECT", db); err != nil {
						c.Close()
						return nil, err
					}
				}
				return c, nil
			},
			TestOnBorrow: func(c redis.Conn, t time.Time) error {
				_, err := c.Do("PING")
				return err
			},
		},
	}
}

func (q *Queue) Close() error {
	return q.pool.Close()
}

func (q *Queue) key() string { 
	return "known_repos" 
}

func (q *Queue) Seen(ctx context.Context, name string) (bool, error) {
	conn := q.pool.Get()
	defer conn.Close()
	
	sum := sha1.Sum([]byte(name))
	exists, err := redis.Bool(conn.Do("SISMEMBER", q.key(), sum[:]))
	if err != nil {
		return false, fmt.Errorf("failed to check if repo seen: %w", err)
	}
	return exists, nil
}

func (q *Queue) Mark(ctx context.Context, name string) error {
	conn := q.pool.Get()
	defer conn.Close()
	
	sum := sha1.Sum([]byte(name))
	_, err := conn.Do("SADD", q.key(), sum[:])
	if err != nil {
		return fmt.Errorf("failed to mark repo as seen: %w", err)
	}
	return nil
}

func (q *Queue) Enqueue(ctx context.Context, repoURL string) error {
	conn := q.pool.Get()
	defer conn.Close()
	
	_, err := conn.Do("LPUSH", "mirror_jobs", repoURL)
	if err != nil {
		return fmt.Errorf("failed to enqueue repo: %w", err)
	}
	return nil
}

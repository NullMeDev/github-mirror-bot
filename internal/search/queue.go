package search

import (
	"context"
	"crypto/sha1"

	"github.com/gomodule/redigo/redis"
)

type Queue struct {
	pool *redis.Pool
}

func NewQueue(addr string) *Queue {
	return &Queue{
		pool: &redis.Pool{
			MaxIdle: 3,
			Dial: func() (redis.Conn, error) {
				return redis.Dial("tcp", addr)
			},
		},
	}
}

func (q *Queue) key() string { return "known_repos" }

func (q *Queue) Seen(name string) bool {
	conn := q.pool.Get()
	defer conn.Close()
	sum := sha1.Sum([]byte(name))
	exists, _ := redis.Bool(conn.Do("SISMEMBER", q.key(), sum[:]))
	return exists
}

func (q *Queue) Mark(name string) {
	conn := q.pool.Get()
	defer conn.Close()
	sum := sha1.Sum([]byte(name))
	_, _ = conn.Do("SADD", q.key(), sum[:])
}

func (q *Queue) Enqueue(ctx context.Context, repoURL string) error {
	conn := q.pool.Get()
	defer conn.Close()
	_, err := conn.Do("LPUSH", "mirror_jobs", repoURL)
	return err
}

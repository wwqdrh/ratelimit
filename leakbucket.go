package ratelimit

// 漏桶实现

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/benbjohnson/clock"
)

type leakLimiter interface {
	Take() time.Time
}

type leakyBucket struct {
	rate int

	data sync.Map
}

func (m *leakyBucket) GetBucket(key string) leakLimiter {
	val, _ := m.data.LoadOrStore(key, NewAtomicInt64Based(m.rate))
	return val.(leakLimiter)
}

type config struct {
	clock Clock
	slack int           // 允许的突发流量大小
	per   time.Duration // 单位时间
}

type leakOption func(c *config)

func WithClock(cl Clock) leakOption {
	return func(c *config) {
		c.clock = cl
		if c.clock == nil {
			c.clock = realClock{}
		}
	}
}

func WithSlack(slack int) leakOption {
	return func(c *config) {
		c.slack = slack
	}
}

func WithPer(per time.Duration) leakOption {
	return func(c *config) {
		c.per = per
	}
}

func NewConfig(rate int, opts ...leakOption) config {
	c := config{
		clock: clock.New(),
		slack: 10,
		per:   time.Second,
	}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

// cpu cache 一般是以 cache line 为单位的，在 64 位的机器上一般是 64 字节
// 如果高频并发访问的数据小于 64 字节的时候就可能会和其他数据一起缓存，其他数据如果出现改变就会导致 cpu 认为缓存失效
// 为了尽可能提高性能，填充了 56 字节的无意义数据
type atomicInt64Limiter struct {
	//lint:ignore U1000 Padding is unused but it is crucial to maintain performance
	// of this rate limiter in case of collocation with other frequently accessed memory.
	prepadding [64]byte //nolint:structcheck
	state      int64    // unix nanoseconds of the next permissions issue.
	//lint:ignore U1000 like prepadding.
	postpadding [56]byte //nolint:structcheck

	perRequest time.Duration
	maxSlack   time.Duration
	clock      Clock
}

func NewAtomicInt64Based(rate int, opts ...leakOption) *atomicInt64Limiter {
	config := NewConfig(rate, opts...)
	perRequest := config.per / time.Duration(rate)
	l := &atomicInt64Limiter{
		perRequest: perRequest,
		maxSlack:   time.Duration(config.slack) * perRequest,
		clock:      config.clock,
	}
	atomic.StoreInt64(&l.state, 0)
	return l
}

func (t *atomicInt64Limiter) Take() time.Time {
	var (
		newTimeOfNextPermissionIssue int64
		now                          int64
	)
	for {
		now = t.clock.Now().UnixNano()
		timeOfNextPermissionIssue := atomic.LoadInt64(&t.state)

		switch {
		case timeOfNextPermissionIssue == 0 || (t.maxSlack == 0 && now-timeOfNextPermissionIssue > int64(t.perRequest)):
			// if this is our first call or t.maxSlack == 0 we need to shrink issue time to now
			newTimeOfNextPermissionIssue = now
		case t.maxSlack > 0 && now-timeOfNextPermissionIssue > int64(t.maxSlack):
			// a lot of nanoseconds passed since the last Take call
			// we will limit max accumulated time to maxSlack
			newTimeOfNextPermissionIssue = now - int64(t.maxSlack)
		default:
			// calculate the time at which our permission was issued
			newTimeOfNextPermissionIssue = timeOfNextPermissionIssue + int64(t.perRequest)
		}

		if atomic.CompareAndSwapInt64(&t.state, timeOfNextPermissionIssue, newTimeOfNextPermissionIssue) {
			break
		}
	}
	t.clock.Sleep(time.Duration(newTimeOfNextPermissionIssue - now))
	return time.Unix(0, newTimeOfNextPermissionIssue)
}

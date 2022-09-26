package ratelimit

// 漏桶实现

import (
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/benbjohnson/clock"
)

type leakOption func(c *config)

type config struct {
	clock Clock
	slack int
	per   time.Duration
}

func WithClock(cl Clock) leakOption {
	return func(c *config) {
		c.clock = cl
	}
}

var WithoutSlack leakOption = WithSlack(0)

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

// Option configures a Limiter.
type Option interface {
	apply(*config)
}

type unlimited struct{}

// NewUnlimited returns a RateLimiter that is not limited.
func NewUnlimited() Limiter {
	return unlimited{}
}

func (unlimited) Take() time.Time {
	return time.Now()
}

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

// newAtomicBased returns a new atomic based limiter.
func NewAtomicInt64Based(rate int, opts ...leakOption) *atomicInt64Limiter {
	// TODO consider moving config building to the implementation
	// independent code.
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

type state struct {
	last     time.Time
	sleepFor time.Duration
}

type atomicLimiter struct {
	state unsafe.Pointer
	//lint:ignore U1000 Padding is unused but it is crucial to maintain performance
	padding [56]byte //nolint:structcheck

	perRequest time.Duration
	maxSlack   time.Duration
	clock      Clock
}

// newAtomicBased returns a new atomic based limiter.
func NewAtomicBased(rate int, opts ...leakOption) *atomicLimiter {
	// TODO consider moving config building to the implementation
	// independent code.
	config := NewConfig(rate, opts...)
	perRequest := config.per / time.Duration(rate)
	l := &atomicLimiter{
		perRequest: perRequest,
		maxSlack:   -1 * time.Duration(config.slack) * perRequest,
		clock:      config.clock,
	}

	initialState := state{
		last:     time.Time{},
		sleepFor: 0,
	}
	atomic.StorePointer(&l.state, unsafe.Pointer(&initialState))
	return l
}

func (t *atomicLimiter) Take() time.Time {
	var (
		newState state
		taken    bool
		interval time.Duration
	)
	for !taken {
		now := t.clock.Now()

		previousStatePointer := atomic.LoadPointer(&t.state)
		oldState := (*state)(previousStatePointer)

		newState = state{
			last:     now,
			sleepFor: oldState.sleepFor,
		}

		if oldState.last.IsZero() {
			taken = atomic.CompareAndSwapPointer(&t.state, previousStatePointer, unsafe.Pointer(&newState))
			continue
		}

		newState.sleepFor += t.perRequest - now.Sub(oldState.last)

		if newState.sleepFor < t.maxSlack {
			newState.sleepFor = t.maxSlack
		}
		if newState.sleepFor > 0 {
			newState.last = newState.last.Add(newState.sleepFor)
			interval, newState.sleepFor = newState.sleepFor, 0
		}
		taken = atomic.CompareAndSwapPointer(&t.state, previousStatePointer, unsafe.Pointer(&newState))
	}
	t.clock.Sleep(interval)
	return newState.last
}

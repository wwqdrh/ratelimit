package ratelimit

import (
	"sync"
	"testing"
	"time"

	"go.uber.org/atomic"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/assert"
)

type testRunner interface {
	createLimiter(int, ...leakOption) leakLimiter
	startTaking(rls ...leakLimiter)
	assertCountAt(d time.Duration, count int)
	afterFunc(d time.Duration, fn func())
	getClock() *clock.Mock
}

func TestRateLimiter(t *testing.T) {
	t.Parallel()
	runTest(t, func(r testRunner) {
		rl := r.createLimiter(100, WithSlack(0))

		// Create copious counts concurrently.
		r.startTaking(rl)
		r.startTaking(rl)
		r.startTaking(rl)
		r.startTaking(rl)

		r.assertCountAt(1*time.Second, 100)
		r.assertCountAt(2*time.Second, 200)
		r.assertCountAt(3*time.Second, 300)
	})
}

func TestDelayedRateLimiter(t *testing.T) {
	t.Parallel()
	runTest(t, func(r testRunner) {
		slow := r.createLimiter(10, WithSlack(0))
		fast := r.createLimiter(100, WithSlack(0))

		r.startTaking(slow, fast)

		// slow -> fast, so 20*10 + 10*100
		r.afterFunc(20*time.Second, func() {
			r.startTaking(fast)
			r.startTaking(fast)
			r.startTaking(fast)
			r.startTaking(fast)
		})

		r.assertCountAt(30*time.Second, 1200)
	})
}

func TestPer(t *testing.T) {
	t.Parallel()
	runTest(t, func(r testRunner) {
		rl := r.createLimiter(7, WithSlack(0), WithPer(time.Minute))

		r.startTaking(rl)
		r.startTaking(rl)

		r.assertCountAt(1*time.Second, 1)
		r.assertCountAt(1*time.Minute, 8)
		r.assertCountAt(2*time.Minute, 15)
	})
}

// TestInitial verifies that the initial sequence is scheduled as expected.
func TestInitial(t *testing.T) {
	t.Parallel()
	tests := []struct {
		msg  string
		opts []leakOption
	}{
		{
			msg: "With Slack",
		},
		{
			msg:  "Without Slack",
			opts: []leakOption{WithSlack(0)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			runTest(t, func(r testRunner) {
				rl := r.createLimiter(10, tt.opts...)

				var (
					clk  = r.getClock()
					prev = clk.Now()

					results = make(chan time.Time)
					have    []time.Duration
					startWg sync.WaitGroup
				)
				startWg.Add(3)

				for i := 0; i < 3; i++ {
					go func() {
						startWg.Done()
						results <- rl.Take()
					}()
				}

				startWg.Wait()
				clk.Add(time.Second)

				for i := 0; i < 3; i++ {
					ts := <-results
					have = append(have, ts.Sub(prev))
					prev = ts
				}

				assert.Equal(t,
					[]time.Duration{
						0,
						time.Millisecond * 100,
						time.Millisecond * 100,
					},
					have,
					"bad timestamps for inital takes",
				)
			})
		})
	}
}

func TestSlack(t *testing.T) {
	t.Parallel()
	// To simulate slack, we combine two limiters.
	// - First, we start a single goroutine with both of them,
	//   during this time the slow limiter will dominate,
	//   and allow the fast limiter to accumulate slack.
	// - After 2 seconds, we start another goroutine with
	//   only the faster limiter. This will allow it to max out,
	//   and consume all the slack.
	// - After 3 seconds, we look at the final result, and we expect,
	//   a sum of:
	//   - slower limiter running for 3 seconds
	//   - faster limiter running for 1 second
	//   - slack accumulated by the faster limiter during the two seconds.
	//     it was blocked by slower limiter.
	tests := []struct {
		msg  string
		opt  []leakOption
		want int
	}{
		{
			msg: "no option, defaults to 10",
			// 2*10 + 1*100 + 1*10 (slack)
			want: 130,
		},
		{
			msg: "slack of 10, like default",
			opt: []leakOption{WithSlack(10)},
			// 2*10 + 1*100 + 1*10 (slack)
			want: 130,
		},
		{
			msg: "slack of 20",
			opt: []leakOption{WithSlack(20)},
			// 2*10 + 1*100 + 1*20 (slack)
			want: 140,
		},
		{
			// Note this is bigger then the rate of the limiter.
			msg: "slack of 150",
			opt: []leakOption{WithSlack(150)},
			// 2*10 + 1*100 + 1*150 (slack)
			want: 270,
		},
		{
			msg: "no option, defaults to 10, with per",
			// 2*(10*2) + 1*(100*2) + 1*10 (slack)
			opt:  []leakOption{WithPer(500 * time.Millisecond)},
			want: 230,
		},
		{
			msg: "slack of 10, like default, with per",
			opt: []leakOption{WithSlack(10), WithPer(500 * time.Millisecond)},
			// 2*(10*2) + 1*(100*2) + 1*10 (slack)
			want: 230,
		},
		{
			msg: "slack of 20, with per",
			opt: []leakOption{WithSlack(20), WithPer(500 * time.Millisecond)},
			// 2*(10*2) + 1*(100*2) + 1*20 (slack)
			want: 240,
		},
		{
			// Note this is bigger then the rate of the limiter.
			msg: "slack of 150, with per",
			opt: []leakOption{WithSlack(150), WithPer(500 * time.Millisecond)},
			// 2*(10*2) + 1*(100*2) + 1*150 (slack)
			want: 370,
		},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			runTest(t, func(r testRunner) {
				slow := r.createLimiter(10, WithSlack(0))
				fast := r.createLimiter(100, tt.opt...)

				r.startTaking(slow, fast)

				r.afterFunc(2*time.Second, func() {
					r.startTaking(fast)
					r.startTaking(fast)
				})

				// limiter with 10hz dominates here - we're always at 10.
				r.assertCountAt(1*time.Second, 10)
				r.assertCountAt(3*time.Second, tt.want)
			})
		})
	}
}

type runnerImpl struct {
	t *testing.T

	clock       *clock.Mock
	constructor func(int, ...leakOption) leakLimiter
	count       atomic.Int32
	maxDuration time.Duration
	doneCh      chan struct{}
	wg          sync.WaitGroup
}

func runTest(t *testing.T, fn func(testRunner)) {
	impls := []struct {
		name        string
		constructor func(int, ...leakOption) leakLimiter
	}{
		{
			name: "atomic_int64",
			constructor: func(rate int, opts ...leakOption) leakLimiter {
				return NewAtomicInt64Based(rate, opts...)
			},
		},
	}

	for _, tt := range impls {
		t.Run(tt.name, func(t *testing.T) {
			// Set a non-default time.Time since some limiters (int64 in particular) use
			// the default value as "non-initialized" state.
			clockMock := clock.NewMock()
			clockMock.Set(time.Now())
			r := runnerImpl{
				t:           t,
				clock:       clockMock,
				constructor: tt.constructor,
				doneCh:      make(chan struct{}),
			}
			defer close(r.doneCh)
			defer r.wg.Wait()

			fn(&r)
			r.clock.Add(r.maxDuration)
		})
	}
}

// createLimiter builds a limiter with given options.
func (r *runnerImpl) createLimiter(rate int, opts ...leakOption) leakLimiter {
	opts = append(opts, WithClock(r.clock))
	return r.constructor(rate, opts...)
}

func (r *runnerImpl) getClock() *clock.Mock {
	return r.clock
}

// startTaking tries to Take() on passed in limiters in a loop/goroutine.
func (r *runnerImpl) startTaking(rls ...leakLimiter) {
	r.goWait(func() {
		for {
			for _, rl := range rls {
				rl.Take()
			}
			r.count.Inc()
			select {
			case <-r.doneCh:
				return
			default:
			}
		}
	})
}

// assertCountAt asserts the limiters have Taken() a number of times at a given time.
func (r *runnerImpl) assertCountAt(d time.Duration, count int) {
	r.wg.Add(1)
	r.afterFunc(d, func() {
		assert.Equal(r.t, int32(count), r.count.Load(), "count not as expected")
		r.wg.Done()
	})
}

// afterFunc executes a func at a given time.
func (r *runnerImpl) afterFunc(d time.Duration, fn func()) {
	if d > r.maxDuration {
		r.maxDuration = d
	}

	r.goWait(func() {
		select {
		case <-r.doneCh:
			return
		case <-r.clock.After(d):
		}
		fn()
	})
}

// goWait runs a function in a goroutine and makes sure the goroutine was scheduled.
func (r *runnerImpl) goWait(fn func()) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		wg.Done()
		fn()
	}()
	wg.Wait()
}

package ratelimit

// 令牌桶实现
import (
	"math"
	"sync"
	"time"
)

// a base wrapper
type tokenBucket struct {
	fillInterval time.Duration
	cap          int64
	quantum      int64

	data sync.Map
}

// if not exist, create
func (m *tokenBucket) GetBucket(key string) *Bucket {
	val, _ := m.data.LoadOrStore(key, func() *Bucket {
		bucket := newBucket(m.fillInterval, m.cap, BucketWithQuantum(m.quantum))
		return bucket
	}())
	return val.(*Bucket)
}

type Bucket struct {
	clock Clock

	startTime time.Time // bucket start time

	capacity int64

	rate float64

	quantum int64 // added on each tick

	fillInterval time.Duration // the tick interval

	mu *sync.Mutex

	availableTokens int64

	latestTick int64
}

type bucketOpt func(b *Bucket)

func BucketWithClock(clock Clock) bucketOpt {
	return func(b *Bucket) {
		if clock == nil {
			clock = realClock{}
		}
		b.clock = clock
		b.startTime = clock.Now()
	}
}

func BucketWithRate(rate float64) bucketOpt {
	return func(b *Bucket) {
		b.rate = rate
	}
}

func BucketWithQuantum(quantum int64) bucketOpt {
	return func(b *Bucket) {
		if quantum <= 0 {
			panic("token bucket quantum is not > 0")
		}
		b.quantum = quantum
	}
}

const rateMargin = 0.01

func newBucket(fillInterval time.Duration, capacity int64, opts ...bucketOpt) *Bucket {
	if fillInterval <= 0 {
		panic("token bucket fill interval is not > 0")
	}
	if capacity <= 0 {
		panic("token bucket capacity is not > 0")
	}

	buck := &Bucket{
		clock:           realClock{},
		startTime:       realClock{}.Now(),
		capacity:        capacity,
		quantum:         1,
		fillInterval:    fillInterval,
		mu:              &sync.Mutex{},
		availableTokens: capacity,
		latestTick:      0,
	}
	for _, opt := range opts {
		opt(buck)
	}

	for quantum := int64(1); quantum < 1<<50; quantum = nextQuantum(quantum) {
		fillInterval := time.Duration(float64(time.Second) * float64(quantum) / buck.rate)
		if fillInterval <= 0 {
			continue
		}
		buck.fillInterval = fillInterval
		buck.quantum = quantum
		if diff := math.Abs(buck.Rate() - buck.rate); diff/buck.rate <= rateMargin {
			// mean the interval cocret
			return buck
		}
	}
	return buck
}

func nextQuantum(q int64) int64 {
	q1 := q * 11 / 10
	if q1 == q {
		q1++
	}
	return q1
}

func (tb *Bucket) Wait(count int64) {
	if d := tb.Take(count); d > 0 {
		tb.clock.Sleep(d)
	}
}

func (tb *Bucket) WaitMaxDuration(count int64, maxWait time.Duration) bool {
	d, ok := tb.TakeMaxDuration(count, maxWait)
	if d > 0 {
		tb.clock.Sleep(d)
	}
	return ok
}

const infinityDuration time.Duration = 0x7fffffffffffffff

func (tb *Bucket) Take(count int64) time.Duration {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	d, _ := tb.take(tb.clock.Now(), count, infinityDuration)
	return d
}

func (tb *Bucket) TakeMaxDuration(count int64, maxWait time.Duration) (time.Duration, bool) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.take(tb.clock.Now(), count, maxWait)
}

func (tb *Bucket) TakeAvailable(count int64) int64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.takeAvailable(tb.clock.Now(), count)
}

func (tb *Bucket) takeAvailable(now time.Time, count int64) int64 {
	if count <= 0 {
		return 0
	}
	tb.adjustavailableTokens(tb.currentTick(now))
	if tb.availableTokens <= 0 {
		return 0
	}
	if count > tb.availableTokens {
		count = tb.availableTokens
	}
	tb.availableTokens -= count
	return count
}

func (tb *Bucket) Available() int64 {
	return tb.available(tb.clock.Now())
}

func (tb *Bucket) available(now time.Time) int64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.adjustavailableTokens(tb.currentTick(now))
	return tb.availableTokens
}

func (tb *Bucket) Capacity() int64 {
	return tb.capacity
}

func (tb *Bucket) Rate() float64 {
	return 1e9 * float64(tb.quantum) / float64(tb.fillInterval)
}

func (tb *Bucket) take(now time.Time, count int64, maxWait time.Duration) (time.Duration, bool) {
	if count <= 0 {
		return 0, true
	}

	tick := tb.currentTick(now)
	tb.adjustavailableTokens(tick)
	avail := tb.availableTokens - count
	if avail >= 0 {
		tb.availableTokens = avail
		return 0, true
	}

	endTick := tick + (-avail+tb.quantum-1)/tb.quantum
	endTime := tb.startTime.Add(time.Duration(endTick) * tb.fillInterval)
	waitTime := endTime.Sub(now)
	if waitTime > maxWait {
		return 0, false
	}
	tb.availableTokens = avail
	return waitTime, true
}

func (tb *Bucket) currentTick(now time.Time) int64 {
	return int64(now.Sub(tb.startTime) / tb.fillInterval)
}

func (tb *Bucket) adjustavailableTokens(tick int64) {
	lastTick := tb.latestTick
	tb.latestTick = tick
	if tb.availableTokens >= tb.capacity {
		return
	}
	tb.availableTokens += (tick - lastTick) * tb.quantum
	if tb.availableTokens > tb.capacity {
		tb.availableTokens = tb.capacity
	}
}

package ratelimit

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Limiter interface {
	Take() time.Time
}

type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

var DefaultTokenBucketMap *tokenMap

type tokenMap struct {
	fillInterval time.Duration
	cap          int64
	quantum      int64

	data map[string]*Bucket
}

// if not exist, create
func (m *tokenMap) GetBucket(key string) *Bucket {
	if val, ok := m.data[key]; !ok {
		bucket := newBucket(m.fillInterval, m.cap, BucketWithQuantum(m.quantum))
		m.data[key] = bucket
		return bucket
	} else {
		return val
	}
}

// 令牌桶
// NewBucketWithQuantum和普通的 NewBucket() 的区别是，每次向桶中放令牌时，是放 quantum 个令牌，而不是一个令牌。
func TokenBucketMiddleware(fillInterval time.Duration, cap, quantum int64) gin.HandlerFunc {
	DefaultTokenBucketMap = &tokenMap{
		fillInterval: fillInterval,
		cap:          cap,
		quantum:      quantum,
		data:         map[string]*Bucket{},
	}

	return func(c *gin.Context) {
		if DefaultTokenBucketMap.GetBucket(fmt.Sprint(c.Request.URL)).TakeAvailable(1) < 1 {
			c.String(http.StatusForbidden, "rate limit...")
			c.Abort()
			return
		}
		c.Next()
	}
}

var DefaultLeakyBucketMap *leakyMap

type leakyMap struct {
	rate int

	data map[string]Limiter
}

// if not exist, create
func (m *leakyMap) GetBucket(key string) Limiter {
	if val, ok := m.data[key]; !ok {
		var lim Limiter = NewAtomicBased(m.rate)
		m.data[key] = lim
		return lim
	} else {
		return val
	}
}

// 漏桶 一秒能过多少请求，qps
func LeakyBucketMiddleware(rate int) gin.HandlerFunc {
	DefaultLeakyBucketMap = &leakyMap{
		rate: rate,
		data: map[string]Limiter{},
	}
	return func(ctx *gin.Context) {
		(DefaultLeakyBucketMap.GetBucket(fmt.Sprint(ctx.Request.URL))).Take()
		ctx.Next()
	}
}

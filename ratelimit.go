package ratelimit

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)


type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}

func (realClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

// 令牌桶
func TokenBucketMiddleware(fillInterval time.Duration, cap, quantum int64) gin.HandlerFunc {
	bucket := &tokenBucket{
		fillInterval: fillInterval,
		cap:          cap,
		quantum:      quantum,
		data:         sync.Map{},
	}

	return func(c *gin.Context) {
		if bucket.GetBucket(fmt.Sprint(c.Request.URL)).TakeAvailable(1) < 1 {
			c.String(http.StatusForbidden, "rate limit...")
			c.Abort()
			return
		}
		c.Next()
	}
}

// 漏桶 一秒能过多少请求，qps
func LeakyBucketMiddleware(rate int) gin.HandlerFunc {
	bucket := &leakyBucket{
		rate: rate,
		data: sync.Map{},
	}
	return func(ctx *gin.Context) {
		(bucket.GetBucket(fmt.Sprint(ctx.Request.URL))).Take()
		ctx.Next()
	}
}

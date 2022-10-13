package ratelimit

import (
	"testing"
	"time"
)

func TestAvailable(t *testing.T) {
	for i, tt := range []struct {
		about        string
		capacity     int64
		fillInterval time.Duration
		take         int64
		sleep        time.Duration

		expectCountAfterTake  int64
		expectCountAfterSleep int64
	}{{
		about:                 "should fill tokens after interval",
		capacity:              5,
		fillInterval:          time.Second,
		take:                  5,
		sleep:                 time.Second,
		expectCountAfterTake:  0,
		expectCountAfterSleep: 1,
	}, {
		about:                 "should fill tokens plus existing count",
		capacity:              2,
		fillInterval:          time.Second,
		take:                  1,
		sleep:                 time.Second,
		expectCountAfterTake:  1,
		expectCountAfterSleep: 2,
	}, {
		about:                 "shouldn't fill before interval",
		capacity:              2,
		fillInterval:          2 * time.Second,
		take:                  1,
		sleep:                 time.Second,
		expectCountAfterTake:  1,
		expectCountAfterSleep: 1,
	}, {
		about:                 "should fill only once after 1*interval before 2*interval",
		capacity:              2,
		fillInterval:          2 * time.Second,
		take:                  1,
		sleep:                 3 * time.Second,
		expectCountAfterTake:  1,
		expectCountAfterSleep: 2,
	}} {
		tb := newBucket(tt.fillInterval, tt.capacity)
		if c := tb.takeAvailable(tb.startTime, tt.take); c != tt.take {
			t.Fatalf("#%d: %s, take = %d, want = %d", i, tt.about, c, tt.take)
		}
		if c := tb.available(tb.startTime); c != tt.expectCountAfterTake {
			t.Fatalf("#%d: %s, after take, available = %d, want = %d", i, tt.about, c, tt.expectCountAfterTake)
		}
		if c := tb.available(tb.startTime.Add(tt.sleep)); c != tt.expectCountAfterSleep {
			t.Fatalf("#%d: %s, after some time it should fill in new tokens, available = %d, want = %d",
				i, tt.about, c, tt.expectCountAfterSleep)
		}
	}

}

func TestNoBonusTokenAfterBucketIsFull(t *testing.T) {
	tb := newBucket(time.Second*1, 100, BucketWithQuantum(20))
	curAvail := tb.Available()
	if curAvail != 100 {
		t.Fatalf("initially: actual available = %d, expected = %d", curAvail, 100)
	}

	time.Sleep(time.Second * 5)

	curAvail = tb.Available()
	if curAvail != 100 {
		t.Fatalf("after pause: actual available = %d, expected = %d", curAvail, 100)
	}

	cnt := tb.TakeAvailable(100)
	if cnt != 100 {
		t.Fatalf("taking: actual taken count = %d, expected = %d", cnt, 100)
	}

	curAvail = tb.Available()
	if curAvail != 0 {
		t.Fatalf("after taken: actual available = %d, expected = %d", curAvail, 0)
	}
}

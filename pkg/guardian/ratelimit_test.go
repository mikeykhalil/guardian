package guardian

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type FakeLimitStore struct {
	limit       Limit
	count       map[string]uint64
	injectedErr error
}

func (fl *FakeLimitStore) GetLimit() Limit {
	return fl.limit
}

func (fl *FakeLimitStore) Incr(context context.Context, key string, count uint, expireIn time.Duration) (uint64, error) {
	if fl.injectedErr != nil {
		return 0, fl.injectedErr
	}

	fl.count[key] += uint64(count)

	return fl.count[key], nil
}

func TestIPRateLimiterReturnsErrorWithInvalidStore(t *testing.T) {
	_, err := NewIPRateLimiter(nil)
	if err == nil {
		t.Errorf("error was nil when it shouldn't have been")
	}
}

func TestLimitRateLimits(t *testing.T) {

	// 3 rps
	limit := Limit{Count: 3, Duration: 1 * time.Second}

	fstore := &FakeLimitStore{limit: limit, count: make(map[string]uint64)}
	rl, err := NewIPRateLimiter(fstore)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	req := Request{RemoteAddress: "192.168.1.2"}
	sentCount := 10

	for i := 0; i < sentCount; i++ {
		blocked, remaining, err := rl.Limit(context.Background(), req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedBlocked := (limit.Count < uint64(i+1))
		if blocked != expectedBlocked {
			t.Fatalf("expected blocked: %v, received blocked: %v", expectedBlocked, blocked)
		}

		expectedRemaining := limit.Count - uint64(i+1)
		if limit.Count < uint64(i+1) {
			expectedRemaining = 0
		}
		if remaining != uint32(expectedRemaining) {
			t.Fatalf("remaining was %d when it should have been %d", remaining, expectedRemaining)
		}
	}
}

func TestLimitRateLimitsButThenAllowsAgain(t *testing.T) {

	// 3 rps
	limit := Limit{Count: 3, Duration: 1 * time.Second}

	fstore := &FakeLimitStore{limit: limit, count: make(map[string]uint64)}
	rl, err := NewIPRateLimiter(fstore)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	req := Request{RemoteAddress: "192.168.1.2"}
	sentCount := 10

	for i := 0; i < sentCount; i++ {
		blocked, remaining, err := rl.Limit(context.Background(), req)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedBlocked := (limit.Count < uint64(i+1))
		if blocked != expectedBlocked {
			t.Fatalf("expected blocked: %v, received blocked: %v", expectedBlocked, blocked)
		}

		expectedRemaining := limit.Count - uint64(i+1)
		if limit.Count < uint64(i+1) {
			expectedRemaining = 0
		}
		if remaining != uint32(expectedRemaining) {
			t.Fatalf("remaining was %d when it should have been %d", remaining, expectedRemaining)
		}
	}

	time.Sleep(limit.Duration)
	blocked, remaining, err := rl.Limit(context.Background(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if blocked != false {
		t.Fatalf("expected blocked: %v, received blocked: %v", false, blocked)
	}
	if remaining != uint32(limit.Count-1) {
		t.Fatalf("remaining was %d when it should have been %d", remaining, uint32(limit.Count-1))
	}
}

func TestLimitRemainingOfflowUsesMaxUInt32(t *testing.T) {

	// 3 rps
	limit := Limit{Count: ^uint64(0), Duration: 1 * time.Second}

	fstore := &FakeLimitStore{limit: limit, count: make(map[string]uint64)}
	rl, err := NewIPRateLimiter(fstore)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	req := Request{RemoteAddress: "192.168.1.2"}
	slot := rl.SlotKey(req, time.Now(), limit.Duration)
	fstore.count[slot] = uint64(^uint32(0)) << 5 // set slot count to some value > max uint32

	blocked, remaining, err := rl.Limit(context.Background(), req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if blocked != false {
		t.Fatalf("expected blocked: %v, received blocked: %v", false, blocked)
	}
	if remaining != ^uint32(0) {
		t.Fatalf("remaining was %d when it should have been %d", remaining, ^uint32(0))
	}
}

func TestLimitFailsOpen(t *testing.T) {

	// 3 rps
	limit := Limit{Count: 3, Duration: 1 * time.Second}

	fstore := &FakeLimitStore{limit: limit, count: make(map[string]uint64), injectedErr: fmt.Errorf("some error")}
	rl, err := NewIPRateLimiter(fstore)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	req := Request{RemoteAddress: "192.168.1.2"}

	blocked, _, err := rl.Limit(context.Background(), req)
	if err == nil {
		t.Error("expected error but received nothing")
	}

	if blocked != false {
		t.Error("failed closed when it should have failed open")
	}
}

func TestSlotKeyGeneration(t *testing.T) {
	rl, err := NewIPRateLimiter(&FakeLimitStore{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	referenceRequest := Request{RemoteAddress: "192.168.1.2"}
	referenceTime := time.Unix(1522969710, 0)

	tests := []struct {
		name          string
		request       Request
		requestTime   time.Time
		limitDuration time.Duration
		want          string
	}{
		{
			name:          "BucketSameSecond",
			request:       referenceRequest,
			requestTime:   referenceTime,
			limitDuration: 10 * time.Second,
			want:          "192.168.1.2:1522969710",
		},
		{
			name:          "BucketRoundsDown",
			request:       referenceRequest,
			requestTime:   referenceTime.Add(5 * time.Second),
			limitDuration: 10 * time.Second,
			want:          "192.168.1.2:1522969710",
		},
		{
			name:          "BucketNext",
			request:       referenceRequest,
			requestTime:   referenceTime.Add(10 * time.Second),
			limitDuration: 10 * time.Second,
			want:          "192.168.1.2:1522969720",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := rl.SlotKey(test.request, test.requestTime, test.limitDuration)
			if got != test.want {
				t.Errorf("got %v, wanted %v", got, test.want)
			}
		})
	}

}
package guardian

import (
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/go-redis/redis"
	"github.com/google/go-cmp/cmp"
)

func newTestConfStore(t *testing.T) (*RedisConfStore, *miniredis.Miniredis) {
	return newTestConfStoreWithDefaults(t, []net.IPNet{}, []net.IPNet{}, Limit{}, false)
}

func newTestConfStoreWithDefaults(t *testing.T, defaultWhitelist []net.IPNet, defaultBlacklist []net.IPNet, defaultLimit Limit, defaultReportOnly bool) (*RedisConfStore, *miniredis.Miniredis) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("error creating miniredis")
	}

	redis := redis.NewClient(&redis.Options{Addr: s.Addr()})
	return NewRedisConfStore(redis, defaultWhitelist, defaultBlacklist, defaultLimit, defaultReportOnly, TestingLogger), s
}

func TestConfStoreReturnsDefaults(t *testing.T) {
	expectedWhitelist := parseCIDRs([]string{"10.0.0.1/8"})
	expectedBlacklist := parseCIDRs([]string{"12.0.0.1/8"})
	expectedLimit := Limit{Count: 20, Duration: time.Second, Enabled: true}
	expectedReportOnly := true

	c, s := newTestConfStoreWithDefaults(t, expectedWhitelist, expectedBlacklist, expectedLimit, expectedReportOnly)
	defer s.Close()

	gotWhitelist := c.GetWhitelist()
	gotBlacklist := c.GetBlacklist()
	gotLimit := c.GetLimit()
	gotReportOnly := c.GetReportOnly()

	if !cmp.Equal(gotWhitelist, expectedWhitelist) {
		t.Errorf("expected: %v received: %v", expectedWhitelist, gotWhitelist)
	}

	if !cmp.Equal(gotBlacklist, expectedBlacklist) {
		t.Errorf("expected: %v received: %v", expectedWhitelist, gotWhitelist)
	}

	if gotLimit != expectedLimit {
		t.Errorf("expected: %v received: %v", expectedLimit, gotLimit)
	}

	if gotReportOnly != expectedReportOnly {
		t.Errorf("expected: %v received: %v", expectedReportOnly, gotReportOnly)
	}
}

func TestConfStoreReturnsEmptyWhitelistIfNil(t *testing.T) {
	expectedWhitelist := []net.IPNet{}
	expectedLimit := Limit{Count: 20, Duration: time.Second, Enabled: true}
	expectedReportOnly := true

	c, s := newTestConfStoreWithDefaults(t, nil, nil, expectedLimit, expectedReportOnly)
	defer s.Close()

	gotWhitelist := c.GetWhitelist()

	if !cmp.Equal(gotWhitelist, expectedWhitelist) {
		t.Errorf("expected: %v received: %v", expectedWhitelist, gotWhitelist)
	}
}

func TestConfStoreFetchesSets(t *testing.T) {
	c, s := newTestConfStore(t)
	defer s.Close()

	expectedWhitelist := parseCIDRs([]string{"10.0.0.1/8"})
	expectedBlacklist := parseCIDRs([]string{"12.0.0.1/8"})
	expectedLimit := Limit{Count: 20, Duration: time.Second, Enabled: true}
	expectedReportOnly := true

	if err := c.AddWhitelistCidrs(expectedWhitelist); err != nil {
		t.Fatalf("got error: %v", err)
	}

	if err := c.AddBlacklistCidrs(expectedBlacklist); err != nil {
		t.Fatalf("got error: %v", err)
	}

	if err := c.SetLimit(expectedLimit); err != nil {
		t.Fatalf("got error: %v", err)
	}

	if err := c.SetReportOnly(expectedReportOnly); err != nil {
		t.Fatalf("got error: %v", err)
	}

	gotWhitelist, err := c.FetchWhitelist()
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	gotBlacklist, err := c.FetchBlacklist()
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	gotLimit, err := c.FetchLimit()
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	gotReportOnly, err := c.FetchReportOnly()
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	if !cmp.Equal(gotWhitelist, expectedWhitelist) {
		t.Errorf("expected: %v received: %v", expectedWhitelist, gotWhitelist)
	}

	if !cmp.Equal(gotBlacklist, expectedBlacklist) {
		t.Errorf("expected: %v received: %v", expectedBlacklist, gotBlacklist)
	}

	if gotLimit != expectedLimit {
		t.Errorf("expected: %v received: %v", expectedLimit, gotLimit)
	}

	if gotReportOnly != expectedReportOnly {
		t.Errorf("expected: %v received: %v", expectedReportOnly, gotReportOnly)
	}
}

func TestConfStoreUpdateCacheConf(t *testing.T) {
	c, s := newTestConfStore(t)
	defer s.Close()

	expectedWhitelist := parseCIDRs([]string{"10.0.0.1/8"})
	expectedBlacklist := parseCIDRs([]string{"12.0.0.1/8"})
	expectedLimit := Limit{Count: 20, Duration: time.Second, Enabled: true}
	expectedReportOnly := true

	if err := c.AddWhitelistCidrs(expectedWhitelist); err != nil {
		t.Fatalf("got error: %v", err)
	}

	if err := c.AddBlacklistCidrs(expectedBlacklist); err != nil {
		t.Fatalf("got error: %v", err)
	}

	if err := c.SetLimit(expectedLimit); err != nil {
		t.Fatalf("got error: %v", err)
	}

	if err := c.SetReportOnly(expectedReportOnly); err != nil {
		t.Fatalf("got error: %v", err)
	}

	c.UpdateCachedConf()

	gotWhitelist := c.GetWhitelist()
	gotBlacklist := c.GetBlacklist()
	gotLimit := c.GetLimit()
	gotReportOnly := c.GetReportOnly()

	if !cmp.Equal(gotWhitelist, expectedWhitelist) {
		t.Errorf("expected: %v received: %v", expectedWhitelist, gotWhitelist)
	}

	if !cmp.Equal(gotBlacklist, expectedBlacklist) {
		t.Errorf("expected: %v received: %v", expectedBlacklist, gotBlacklist)
	}

	if gotLimit != expectedLimit {
		t.Errorf("expected: %v received: %v", expectedLimit, gotLimit)
	}

	if gotReportOnly != expectedReportOnly {
		t.Errorf("expected: %v received: %v", expectedReportOnly, gotReportOnly)
	}
}

func TestConfStoreRunUpdatesCache(t *testing.T) {
	c, s := newTestConfStore(t)
	defer s.Close()

	expectedWhitelist := parseCIDRs([]string{"10.1.1.1/8"})
	expectedBlacklist := parseCIDRs([]string{"11.1.1.1/8"})
	expectedLimit := Limit{Count: 40, Duration: time.Minute, Enabled: true}
	expectedReportOnly := true

	if err := c.AddWhitelistCidrs(expectedWhitelist); err != nil {
		t.Fatalf("got error: %v", err)
	}

	if err := c.AddBlacklistCidrs(expectedBlacklist); err != nil {
		t.Fatalf("got error: %v", err)
	}

	if err := c.SetLimit(expectedLimit); err != nil {
		t.Fatalf("got error: %v", err)
	}

	if err := c.SetReportOnly(expectedReportOnly); err != nil {
		t.Fatalf("got error: %v", err)
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		c.RunSync(1*time.Second, stop)
		close(done)
	}()
	time.Sleep(2 * time.Second)
	close(stop)
	<-done

	gotWhitelist := c.GetWhitelist()
	gotBlacklist := c.GetBlacklist()
	gotLimit := c.GetLimit()
	gotReportOnly := c.GetReportOnly()

	if !cmp.Equal(gotWhitelist, expectedWhitelist) {
		t.Errorf("expected: %v received: %v", expectedWhitelist, gotWhitelist)
	}

	if !cmp.Equal(gotBlacklist, expectedBlacklist) {
		t.Errorf("expected: %v received: %v", expectedWhitelist, gotWhitelist)
	}

	if gotLimit != expectedLimit {
		t.Errorf("expected: %v received: %v", expectedLimit, gotLimit)
	}

	if gotReportOnly != expectedReportOnly {
		t.Errorf("expected: %v received: %v", expectedReportOnly, gotReportOnly)
	}
}

func TestConfStoreRemoveWhitelistCidr(t *testing.T) {
	c, s := newTestConfStore(t)
	defer s.Close()

	addWhitelist := parseCIDRs([]string{"10.1.1.1/8", "192.168.1.1/24"})
	if err := c.AddWhitelistCidrs(addWhitelist); err != nil {
		t.Fatalf("got error: %v", err)
	}

	if err := c.RemoveWhitelistCidrs(parseCIDRs([]string{"10.1.1.1/8"})); err != nil {
		t.Fatalf("got error: %v", err)
	}

	gotWhitelist, err := c.FetchWhitelist()
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	expectedWhitelist := parseCIDRs([]string{"192.168.1.1/24"})
	if !cmp.Equal(gotWhitelist, expectedWhitelist) {
		t.Errorf("expected: %v received: %v", expectedWhitelist, gotWhitelist)
	}
}

func TestConfStoreRemoveBlacklistCidr(t *testing.T) {
	c, s := newTestConfStore(t)
	defer s.Close()

	addBlacklist := parseCIDRs([]string{"10.1.1.1/8", "192.168.1.1/24"})
	if err := c.AddBlacklistCidrs(addBlacklist); err != nil {
		t.Fatalf("got error: %v", err)
	}

	if err := c.RemoveBlacklistCidrs(parseCIDRs([]string{"10.1.1.1/8"})); err != nil {
		t.Fatalf("got error: %v", err)
	}

	got, err := c.FetchBlacklist()
	if err != nil {
		t.Fatalf("got error: %v", err)
	}

	expected := parseCIDRs([]string{"192.168.1.1/24"})
	if !cmp.Equal(got, expected) {
		t.Errorf("expected: %v received: %v", expected, got)
	}
}

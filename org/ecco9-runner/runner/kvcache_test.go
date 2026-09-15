package runner

import (
	"math"
	"testing"
)

func TestKVCacheAcquireAndRelease(t *testing.T) {
	c := NewKVCache(128, 2)
	s, err := c.Acquire(-1)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	// Acquire the same seq again while in use should grab the other slot.
	s2, err := c.Acquire(s.SeqID)
	if err != nil {
		t.Fatalf("Acquire second: %v", err)
	}
	if s2.SeqID == s.SeqID {
		t.Fatal("expected a distinct slot while first is in use")
	}
	c.Release(s.SeqID)
	// Now the original seq is free again.
	s3, err := c.Acquire(s.SeqID)
	if err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
	if s3.SeqID != s.SeqID {
		t.Fatalf("expected to reacquire seq %d, got %d", s.SeqID, s3.SeqID)
	}
}

func TestKVCacheFullReturnsError(t *testing.T) {
	c := NewKVCache(64, 1)
	if _, err := c.Acquire(-1); err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	if _, err := c.Acquire(-1); err != ErrKVCacheFull {
		t.Fatalf("expected ErrKVCacheFull, got %v", err)
	}
}

func TestKVCachePutAndUtilization(t *testing.T) {
	c := NewKVCache(100, 1)
	if _, err := c.Acquire(0); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := c.Put(0, make([]int32, 40)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if got := c.Utilization(); got != 0.4 {
		t.Fatalf("Utilization = %v, want 0.4", got)
	}
	// Overfilling the context window must fail.
	if err := c.Put(0, make([]int32, 61)); err != ErrKVCacheFull {
		t.Fatalf("expected ErrKVCacheFull on overflow, got %v", err)
	}
}

func TestKVCacheCopyPrefix(t *testing.T) {
	c := NewKVCache(64, 2)
	_, _ = c.Acquire(0)
	_, _ = c.Acquire(1)
	if err := c.Put(0, []int32{1, 2, 3, 4, 5}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	c.CopyPrefix(0, 1, 3)
	if got := c.slots[1].Len(); got != 3 {
		t.Fatalf("dst prefix len = %d, want 3", got)
	}
	// Copying more than available clamps to source length.
	c.CopyPrefix(0, 1, 100)
	if got := c.slots[1].Len(); got != 5 {
		t.Fatalf("clamped prefix len = %d, want 5", got)
	}
}

func TestKVCacheCanResume(t *testing.T) {
	c := NewKVCache(64, 1)
	_, _ = c.Acquire(0)
	if err := c.Put(0, []int32{7, 8, 9}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if !c.CanResume(0, 3) {
		t.Fatal("CanResume(0,3) should be true when cache holds exactly 3 positions")
	}
	if c.CanResume(0, 2) {
		t.Fatal("CanResume(0,2) should be false for a mid-prefix position")
	}
	if c.CanResume(9, 3) {
		t.Fatal("CanResume on invalid seq should be false")
	}
}

func TestKVCacheRemoveRange(t *testing.T) {
	c := NewKVCache(64, 1)
	_, _ = c.Acquire(0)
	if err := c.Put(0, []int32{1, 2, 3, 4, 5}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	// Remove everything from index 2 onward (math.MaxInt32 sentinel).
	if err := c.Remove(0, 2, math.MaxInt32); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if got := c.slots[0].Len(); got != 2 {
		t.Fatalf("len after remove = %d, want 2", got)
	}
	// Removing from an out-of-range seq errors.
	if err := c.Remove(9, 0, 1); err != ErrNotSupported {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}

func TestKVCacheCloseFreesSlots(t *testing.T) {
	c := NewKVCache(32, 2)
	_, _ = c.Acquire(0)
	_ = c.Put(0, []int32{1, 2, 3})
	c.Close()
	if got := c.Utilization(); got != 0 {
		t.Fatalf("Utilization after Close = %v, want 0", got)
	}
	// Slots usable again after close.
	if _, err := c.Acquire(0); err != nil {
		t.Fatalf("Acquire after Close: %v", err)
	}
}

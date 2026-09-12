package runner

import (
	"errors"
	"math"
	"sync"
)

// KV-cache errors, adapted from kvcache/cache.go.
var (
	ErrKVCacheFull  = errors.New("could not find a kv cache slot")
	ErrNotSupported = errors.New("model does not support operation")
)

// Slot is one sequence's contiguous cache window (the InputCacheSlot of the
// monorepo reduced to its control-plane essentials).
type Slot struct {
	SeqID int
	// tokens holds the cached positions for this sequence.
	tokens []int32
	inUse  bool
}

// Len returns the number of cached positions.
func (s *Slot) Len() int { return len(s.tokens) }

// KVCache manages per-sequence key/value cache slots, a control-plane
// adaptation of the kvcache.Cache interface: sequence slot allocation,
// prefix copy, resume checks, and ranged removal.
type KVCache struct {
	mu sync.Mutex
	// capacity: total cache entries per sequence (context window).
	capacity int
	// maxSequences: slots across all batches.
	maxSequences int
	slots        []*Slot
}

// NewKVCache initializes the cache (the Init() management half of the
// kvcache.Cache interface).
func NewKVCache(capacity, maxSequences int) *KVCache {
	if capacity <= 0 {
		capacity = 4096
	}
	if maxSequences <= 0 {
		maxSequences = 1
	}
	c := &KVCache{capacity: capacity, maxSequences: maxSequences}
	for i := 0; i < maxSequences; i++ {
		c.slots = append(c.slots, &Slot{SeqID: i})
	}
	return c
}

// Acquire finds a free slot for a sequence, mirroring LoadCacheSlot.
func (c *KVCache) Acquire(seq int) (*Slot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if seq >= 0 && seq < len(c.slots) && !c.slots[seq].inUse {
		c.slots[seq].inUse = true
		return c.slots[seq], nil
	}
	for _, s := range c.slots {
		if !s.inUse {
			s.inUse = true
			return s, nil
		}
	}
	return nil, ErrKVCacheFull
}

// Release frees a slot.
func (c *KVCache) Release(seq int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if seq >= 0 && seq < len(c.slots) {
		c.slots[seq].inUse = false
	}
}

// Put appends token positions to a sequence's cache (the management half of
// Put/StartForward).
func (c *KVCache) Put(seq int, positions []int32) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if seq < 0 || seq >= len(c.slots) {
		return ErrNotSupported
	}
	s := c.slots[seq]
	if len(s.tokens)+len(positions) > c.capacity {
		return ErrKVCacheFull
	}
	s.tokens = append(s.tokens, positions...)
	return nil
}

// CopyPrefix copies tokens in [0, n) from srcSeq to dstSeq.
func (c *KVCache) CopyPrefix(srcSeq, dstSeq int, n int32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if srcSeq < 0 || srcSeq >= len(c.slots) || dstSeq < 0 || dstSeq >= len(c.slots) {
		return
	}
	src, dst := c.slots[srcSeq], c.slots[dstSeq]
	if int(n) > len(src.tokens) {
		n = int32(len(src.tokens))
	}
	dst.tokens = append([]int32(nil), src.tokens[:n]...)
}

// CanResume reports whether seq can continue at position pos, i.e. the cache
// holds exactly the prefix [0, pos).
func (c *KVCache) CanResume(seq, pos int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if seq < 0 || seq >= len(c.slots) {
		return false
	}
	return int32(c.slots[seq].Len()) == int32(pos)
}

// Remove deletes tokens in [begin, end) from seq; end == math.MaxInt32
// removes everything from begin onward.
func (c *KVCache) Remove(seq, begin, end int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if seq < 0 || seq >= len(c.slots) {
		return ErrNotSupported
	}
	s := c.slots[seq]
	if begin < 0 || begin > len(s.tokens) {
		return nil
	}
	if end == math.MaxInt32 || end > len(s.tokens) {
		end = len(s.tokens)
	}
	s.tokens = append(s.tokens[:begin], s.tokens[end:]...)
	return nil
}

// Utilization reports filled fraction across all slots.
func (c *KVCache) Utilization() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	used := 0
	for _, s := range c.slots {
		used += len(s.tokens)
	}
	return float64(used) / float64(c.capacity*c.maxSequences)
}

// Close frees all slots.
func (c *KVCache) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range c.slots {
		s.tokens = nil
		s.inUse = false
	}
}

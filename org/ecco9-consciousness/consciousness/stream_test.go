package consciousness

import (
	"sync"
	"testing"
)

func TestConcurrentBroadcastAndUnsubscribe(t *testing.T) {
	s := NewStream()
	for i := 0; i < 1000; i++ {
		ch := s.Subscribe()
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.BroadcastEvent(StreamEvent{Thought: &Thought{Content: "race regression"}})
		}()
		go func() {
			defer wg.Done()
			s.Unsubscribe(ch)
		}()
		wg.Wait()
	}
}

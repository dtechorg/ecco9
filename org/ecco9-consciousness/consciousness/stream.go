// Package consciousness implements the stream of consciousness and layered
// communication extracted from core/consciousness
// (stream_of_consciousness.go, layer_communication.go).
package consciousness

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// LayerType mirrors ecco9.consciousness.v1.LayerType.
type LayerType int32

const (
	LayerUnspecified LayerType = iota
	LayerSensory                  // basic perception layer
	LayerIntegrative              // reflective integration layer
	LayerMeta                     // meta-cognitive layer
)

func (l LayerType) String() string {
	switch l {
	case LayerSensory:
		return "SENSORY"
	case LayerIntegrative:
		return "INTEGRATIVE"
	case LayerMeta:
		return "META"
	}
	return "UNSPECIFIED"
}

// Thought is a single thought in the stream (mirrors Thought proto).
type Thought struct {
	ID              string  `json:"id"`
	Content         string  `json:"content"`
	Intensity       float64 `json:"intensity"`
	TimestampUnixMs int64   `json:"timestamp_unix_ms"`
}

// LayerMessage is passed between consciousness layers (mirrors LayerMessage).
type LayerMessage struct {
	ID              string    `json:"id"`
	SourceLayer     LayerType `json:"source_layer"`
	TargetLayer     LayerType `json:"target_layer"`
	Content         string    `json:"content"`
	Salience        float64   `json:"salience"`
	TimestampUnixMs int64     `json:"timestamp_unix_ms"`
}

// StreamEvent is the oneof payload of StreamConsciousnessResponse.
type StreamEvent struct {
	Thought      *Thought      `json:"thought,omitempty"`
	LayerMessage *LayerMessage `json:"layer_message,omitempty"`
}

// ThoughtType categorizes thoughts (from stream_of_consciousness.go).
type ThoughtType string

const (
	ThoughtPerception    ThoughtType = "perception"
	ThoughtReflection    ThoughtType = "reflection"
	ThoughtQuestion      ThoughtType = "question"
	ThoughtInsight       ThoughtType = "insight"
	ThoughtPlanning      ThoughtType = "planning"
	ThoughtMetaCognition ThoughtType = "metacognition"
	ThoughtWonder        ThoughtType = "wonder"
)

// Stream is the StreamOfConsciousness engine: persistent internal awareness
// with continuous thought generation, insight synthesis, and self-inquiry.
type Stream struct {
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	currentThought *Thought
	history        []*Thought
	maxHistory     int

	awarenessLevel float64
	focusAreas     []string

	thoughtsGenerated uint64
	insightsGenerated uint64
	questionsAsked    uint64

	subscribers []chan StreamEvent
	running     bool
}

// NewStream creates a stream-of-consciousness engine.
func NewStream() *Stream {
	ctx, cancel := context.WithCancel(context.Background())
	return &Stream{
		ctx:            ctx,
		cancel:         cancel,
		history:        make([]*Thought, 0),
		maxHistory:     1000,
		awarenessLevel: 0.5,
	}
}

// Start begins continuous thought generation.
func (s *Stream) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()
	go s.thoughtLoop(3 * time.Second)
	go s.insightLoop(30 * time.Second)
}

// Stop halts the stream.
func (s *Stream) Stop() {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
	s.cancel()
}

// BroadcastEvent emits a raw stream event (thought or layer_message payload).
func (s *Stream) BroadcastEvent(ev StreamEvent) { s.broadcast(ev) }

// Publish accepts an externally-published thought into the stream.
func (s *Stream) Publish(t Thought) bool {
	if t.ID == "" {
		t.ID = fmt.Sprintf("thought_%d", time.Now().UnixNano())
	}
	if t.TimestampUnixMs == 0 {
		t.TimestampUnixMs = time.Now().UnixMilli()
	}
	if t.Intensity == 0 {
		t.Intensity = 0.7
	}
	thought := t
	s.mu.Lock()
	s.currentThought = &thought
	s.history = append(s.history, &thought)
	if len(s.history) > s.maxHistory {
		s.history = s.history[len(s.history)-s.maxHistory:]
	}
	s.thoughtsGenerated++
	s.mu.Unlock()
	s.broadcast(StreamEvent{Thought: &thought})
	return true
}

// Subscribe registers a channel for stream events.
func (s *Stream) Subscribe() chan StreamEvent {
	ch := make(chan StreamEvent, 64)
	s.mu.Lock()
	s.subscribers = append(s.subscribers, ch)
	s.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber.
func (s *Stream) Unsubscribe(ch chan StreamEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sub := range s.subscribers {
		if sub == ch {
			s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

func (s *Stream) broadcast(ev StreamEvent) {
	s.mu.RLock()
	subs := append([]chan StreamEvent(nil), s.subscribers...)
	s.mu.RUnlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// RecentThoughts returns the n most recent thoughts.
func (s *Stream) RecentThoughts(n int) []*Thought {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if n > len(s.history) {
		n = len(s.history)
	}
	out := make([]*Thought, n)
	copy(out, s.history[len(s.history)-n:])
	return out
}

// Metrics returns stream counters.
func (s *Stream) Metrics() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]any{
		"thoughts_generated": s.thoughtsGenerated,
		"insights_generated": s.insightsGenerated,
		"questions_asked":    s.questionsAsked,
		"history_size":       len(s.history),
		"awareness_level":    s.awarenessLevel,
	}
}

// thoughtLoop generates a thought every interval.
func (s *Stream) thoughtLoop(d time.Duration) {
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.generateThought()
		}
	}
}

// insightLoop periodically synthesizes insights from recent thoughts.
func (s *Stream) insightLoop(d time.Duration) {
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.generateInsight()
		}
	}
}

// generateThought creates a new thought from recent context.
func (s *Stream) generateThought() {
	content := fallbackThought()
	s.Publish(Thought{Content: content, Intensity: 0.7})
}

// generateInsight synthesizes an insight when enough thoughts exist.
func (s *Stream) generateInsight() {
	recent := s.RecentThoughts(10)
	if len(recent) < 3 {
		return
	}
	insight := "I'm noticing patterns in how these thoughts connect..."
	s.mu.Lock()
	s.insightsGenerated++
	s.mu.Unlock()
	s.Publish(Thought{Content: insight, Intensity: 0.8})
}

// fallbackThought generates template thoughts without an LLM
// (from stream_of_consciousness.go generateFallbackThought).
func fallbackThought() string {
	templates := []string{
		"I notice patterns emerging in my recent experiences...",
		"What connections exist between these concepts?",
		"How can I deepen my understanding of this domain?",
		"I sense a shift in my cognitive state...",
		"What questions should I be asking?",
		"I'm becoming aware of new possibilities...",
		"How does this relate to my core identity?",
		"I wonder about the implications of this pattern...",
		"There's something interesting about this relationship...",
		"I'm noticing a resonance between these ideas...",
	}
	return templates[int(time.Now().Unix())%len(templates)]
}

// ClassifyThought applies the heuristic thought-type classification from
// stream_of_consciousness.go determineThoughtType.
func ClassifyThought(content string) ThoughtType {
	switch {
	case strings.Contains(content, "?"):
		return ThoughtQuestion
	case strings.Contains(content, "I notice"), strings.Contains(content, "I see"):
		return ThoughtPerception
	case strings.Contains(content, "insight"), strings.Contains(content, "realize"):
		return ThoughtInsight
	case strings.Contains(content, "wonder"), strings.Contains(content, "curious"):
		return ThoughtWonder
	case strings.Contains(content, "plan"), strings.Contains(content, "will"):
		return ThoughtPlanning
	}
	return ThoughtReflection
}

// ---- Layer communication hub (from layer_communication.go) ----

// Hub routes messages between the sensory, integrative, and meta layers.
type Hub struct {
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	channels map[LayerType]chan *LayerMessage
	history  []*LayerMessage
	maxHist  int

	messagesProcessed uint64
	emergenceDetected uint64

	onMessage func(*LayerMessage)
	running   bool
}

// NewHub creates a three-layer communication hub.
func NewHub() *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		ctx:    ctx,
		cancel: cancel,
		channels: map[LayerType]chan *LayerMessage{
			LayerSensory:     make(chan *LayerMessage, 100),
			LayerIntegrative: make(chan *LayerMessage, 100),
			LayerMeta:        make(chan *LayerMessage, 100),
		},
		maxHist: 1000,
	}
}

// SetMessageCallback registers a listener for delivered layer messages.
func (h *Hub) SetMessageCallback(fn func(*LayerMessage)) {
	h.mu.Lock()
	h.onMessage = fn
	h.mu.Unlock()
}

// Start begins per-layer message processors and emergence detection.
func (h *Hub) Start() {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return
	}
	h.running = true
	h.mu.Unlock()
	for layer, ch := range h.channels {
		go h.processLayer(layer, ch)
	}
	go h.detectEmergence()
}

// Stop halts the hub.
func (h *Hub) Stop() {
	h.mu.Lock()
	h.running = false
	h.mu.Unlock()
	h.cancel()
}

// Send routes a message to its target layer channel.
func (h *Hub) Send(msg *LayerMessage) error {
	h.mu.RLock()
	if !h.running {
		h.mu.RUnlock()
		return fmt.Errorf("communication hub not running")
	}
	ch, ok := h.channels[msg.TargetLayer]
	h.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown layer: %d", msg.TargetLayer)
	}
	if msg.ID == "" {
		msg.ID = fmt.Sprintf("msg-%d", time.Now().UnixNano())
	}
	if msg.TimestampUnixMs == 0 {
		msg.TimestampUnixMs = time.Now().UnixMilli()
	}

	h.mu.Lock()
	h.history = append(h.history, msg)
	if len(h.history) > h.maxHist {
		h.history = h.history[1:]
	}
	h.mu.Unlock()

	select {
	case ch <- msg:
	default:
		return fmt.Errorf("layer channel full")
	}
	return nil
}

// processLayer delivers queued messages to the layer callback.
func (h *Hub) processLayer(_ LayerType, ch chan *LayerMessage) {
	for {
		select {
		case <-h.ctx.Done():
			return
		case msg := <-ch:
			h.mu.RLock()
			fn := h.onMessage
			h.mu.RUnlock()
			if fn != nil {
				fn(msg)
			}
			h.mu.Lock()
			h.messagesProcessed++
			h.mu.Unlock()
		}
	}
}

// detectEmergence watches for emergent patterns in message history.
func (h *Hub) detectEmergence() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.analyzeEmergence()
		}
	}
}

// analyzeEmergence detects upward cascades (sensory -> integrative -> meta).
func (h *Hub) analyzeEmergence() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.history) < 10 {
		return
	}
	recent := h.history[len(h.history)-20:]
	layerCount := make(map[LayerType]int)
	for _, m := range recent {
		layerCount[m.SourceLayer]++
	}
	// Bottom-up cascade: sensory activity triggering meta-cognitive attention.
	if layerCount[LayerSensory] > 5 && layerCount[LayerMeta] > 2 {
		h.emergenceDetected++
	}
}

// RecentMessages returns the n most recent inter-layer messages.
func (h *Hub) RecentMessages(n int) []*LayerMessage {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if n > len(h.history) {
		n = len(h.history)
	}
	out := make([]*LayerMessage, n)
	copy(out, h.history[len(h.history)-n:])
	return out
}

// Metrics returns hub counters.
func (h *Hub) Metrics() map[string]any {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return map[string]any{
		"messages_processed":   h.messagesProcessed,
		"emergence_detected":   h.emergenceDetected,
		"message_history_size": len(h.history),
		"sensory_queue":        len(h.channels[LayerSensory]),
		"integrative_queue":    len(h.channels[LayerIntegrative]),
		"meta_queue":           len(h.channels[LayerMeta]),
	}
}

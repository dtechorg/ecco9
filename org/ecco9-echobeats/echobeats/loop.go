// Package echobeats implements the 12-step 3-phase cognitive loop extracted
// from core/echobeats (three_phase_echobeats.go, cognitive_loop.go,
// scheduler.go, twelvestep.go). The loop runs 7 expressive steps and 5
// reflective steps orchestrated by three concurrent SimpleInferenceEngines,
// with a priority-queue event scheduler for goal-directed wake/rest cycles.
package echobeats

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"
)

// CognitivePhase enumerates the loop phases (mirrors ecco9.echobeats.v1).
type CognitivePhase int32

const (
	PhaseUnspecified CognitivePhase = iota
	PhaseExpressive                 // steps 1-7
	PhaseReflective                 // steps 8-12
	PhaseIntegrative                // cycle integration / sync
)

func (p CognitivePhase) String() string {
	switch p {
	case PhaseExpressive:
		return "EXPRESSIVE"
	case PhaseReflective:
		return "REFLECTIVE"
	case PhaseIntegrative:
		return "INTEGRATIVE"
	}
	return "UNSPECIFIED"
}

// StepType categorizes the 12 steps (from shared_types.go).
type StepType int

const (
	StepRelevanceRealization  StepType = iota // steps 1, 7 (pivotal)
	StepAffordanceInteraction                 // steps 2-6
	StepSalienceSimulation                    // steps 8-12
)

func (s StepType) String() string {
	return [...]string{"RelevanceRealization", "AffordanceInteraction", "SalienceSimulation"}[s]
}

// StepExecution records one emitted step execution.
type StepExecution struct {
	Step             int32          `json:"step"`
	Phase            CognitivePhase `json:"phase"`
	Thought          string         `json:"thought"`
	ExecutedAtUnixMs int64          `json:"executed_at_unix_ms"`
	RelevanceScore   float64        `json:"relevance_score"`
}

// EngineState is the state of a SimpleInferenceEngine.
type EngineState int

const (
	EngineIdle EngineState = iota
	EngineProcessing
	EngineWaiting
)

// SimpleInferenceTask is a unit of work for a SimpleInferenceEngine.
type SimpleInferenceTask struct {
	ID        string
	Type      StepType
	StartTime time.Time
	EndTime   time.Time
}

// SimpleInferenceEngine is one of the three concurrent inference engines
// (from three_phase_echobeats.go).
type SimpleInferenceEngine struct {
	mu             sync.RWMutex
	ID             int
	Name           string
	State          EngineState
	CurrentTask    *SimpleInferenceTask
	CompletedTasks uint64
	ProcessingTime time.Duration
}

// ThreePhase is the EchoBeatsThreePhase 12-step cognitive loop.
type ThreePhase struct {
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	engines [3]*SimpleInferenceEngine

	currentStep  int // 1..12
	currentPhase CognitivePhase
	stepHistory  []StepExecution
	maxHistory   int

	presentCommitment string
	cyclesCompleted   uint64
	stepsExecuted     uint64

	subscribers []chan StepExecution
	running     bool
	freqHz      float64
}

// NewThreePhase creates the 12-step 3-phase loop with its three engines.
func NewThreePhase() *ThreePhase {
	ctx, cancel := context.WithCancel(context.Background())
	names := []string{
		"Expressive-Reflective Engine",
		"Perception-Action Engine",
		"Learning-Integration Engine",
	}
	tp := &ThreePhase{
		ctx:          ctx,
		cancel:       cancel,
		currentStep:  1,
		currentPhase: PhaseExpressive,
		maxHistory:   1000,
		freqHz:       1.0,
	}
	for i := range tp.engines {
		tp.engines[i] = &SimpleInferenceEngine{ID: i + 1, Name: names[i]}
	}
	return tp
}

// Start begins the cognitive loop at the given cycle frequency in Hz.
func (tp *ThreePhase) Start(freqHz float64) bool {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	if tp.running {
		return false
	}
	if freqHz > 0 {
		tp.freqHz = freqHz
	}
	// Stop cancels a context permanently. Recreate it for every new lifecycle
	// so a valid start-stop-start sequence continues producing steps.
	tp.ctx, tp.cancel = context.WithCancel(context.Background())
	ctx := tp.ctx
	tp.running = true
	go tp.cognitiveLoop(ctx)
	for _, e := range tp.engines {
		go tp.runEngine(ctx, e)
	}
	return true
}

// Stop halts the cognitive loop.
func (tp *ThreePhase) Stop() bool {
	tp.mu.Lock()
	if !tp.running {
		tp.mu.Unlock()
		return false
	}
	tp.running = false
	tp.mu.Unlock()
	tp.cancel()
	return true
}

// Running reports whether the loop is active.
func (tp *ThreePhase) Running() bool {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	return tp.running
}

// stepInterval maps the cycle frequency to a per-step interval (12 steps/cycle).
func (tp *ThreePhase) stepInterval() time.Duration {
	tp.mu.RLock()
	f := tp.freqHz
	tp.mu.RUnlock()
	if f <= 0 {
		f = 1.0
	}
	return time.Duration(float64(time.Second) / (f * 12.0))
}

// cognitiveLoop executes one step per tick (from three_phase_echobeats.go).
func (tp *ThreePhase) cognitiveLoop(ctx context.Context) {
	ticker := time.NewTicker(tp.stepInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tp.executeNextStep()
		}
	}
}

// runEngine processes tasks for one SimpleInferenceEngine.
func (tp *ThreePhase) runEngine(ctx context.Context, e *SimpleInferenceEngine) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tp.processEngineTask(e)
		}
	}
}

// processEngineTask alternates engine states idle -> processing -> idle.
func (tp *ThreePhase) processEngineTask(e *SimpleInferenceEngine) {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch e.State {
	case EngineIdle:
		tp.mu.RLock()
		step := tp.currentStep
		tp.mu.RUnlock()
		e.CurrentTask = &SimpleInferenceTask{
			ID:        fmt.Sprintf("task_e%d_%d", e.ID, time.Now().UnixNano()),
			Type:      stepType(step),
			StartTime: time.Now(),
		}
		e.State = EngineProcessing
	case EngineProcessing:
		if e.CurrentTask != nil {
			e.CurrentTask.EndTime = time.Now()
			e.ProcessingTime += e.CurrentTask.EndTime.Sub(e.CurrentTask.StartTime)
			e.CompletedTasks++
		}
		e.CurrentTask = nil
		e.State = EngineIdle
	}
}

// executeNextStep runs the current step, records and emits the execution.
func (tp *ThreePhase) executeNextStep() {
	tp.mu.Lock()
	step := tp.currentStep
	tp.mu.Unlock()

	var thought string
	var score float64
	switch {
	case step == 1 || step == 7:
		// Pivotal relevance realization: orient present commitment.
		thought, score = tp.relevanceRealization(step)
	case step >= 2 && step <= 6:
		// Actual affordance interaction: condition past performance.
		thought, score = tp.affordanceInteraction(step)
	default:
		// Virtual salience simulation: anticipate future potential.
		thought, score = tp.salienceSimulation(step)
	}

	exec := StepExecution{
		Step:             int32(step),
		Phase:            phaseForStep(step),
		Thought:          thought,
		ExecutedAtUnixMs: time.Now().UnixMilli(),
		RelevanceScore:   score,
	}

	tp.mu.Lock()
	tp.stepHistory = append(tp.stepHistory, exec)
	if len(tp.stepHistory) > tp.maxHistory {
		tp.stepHistory = tp.stepHistory[len(tp.stepHistory)-tp.maxHistory:]
	}
	tp.stepsExecuted++
	tp.currentStep++
	if tp.currentStep > 12 {
		tp.currentStep = 1
		tp.cyclesCompleted++
	}
	tp.currentPhase = phaseForStep(tp.currentStep)
	subs := append([]chan StepExecution(nil), tp.subscribers...)
	tp.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- exec:
		default: // drop for slow subscribers
		}
	}
}

// relevanceRealization implements steps 1 and 7 (pivotal commitment).
func (tp *ThreePhase) relevanceRealization(step int) (string, float64) {
	focus := fmt.Sprintf("commitment_step%d_%d", step, time.Now().Unix())
	tp.mu.Lock()
	tp.presentCommitment = focus
	tp.mu.Unlock()
	return fmt.Sprintf("Relevance realization: orienting present commitment to %s", focus), 0.8
}

// affordanceInteraction implements steps 2-6.
func (tp *ThreePhase) affordanceInteraction(step int) (string, float64) {
	performance := 0.7 + float64(step)*0.05
	return fmt.Sprintf("Affordance interaction step %d: action completed (quality %.2f)", step, performance), performance
}

// salienceSimulation implements steps 8-12.
func (tp *ThreePhase) salienceSimulation(step int) (string, float64) {
	salience := (0.6 + float64(step-8)*0.08) * (0.7 + float64(step-8)*0.05)
	return fmt.Sprintf("Salience simulation step %d: future scenario explored", step), salience
}

// Subscribe registers a channel receiving every emitted StepExecution.
func (tp *ThreePhase) Subscribe() chan StepExecution {
	ch := make(chan StepExecution, 64)
	tp.mu.Lock()
	tp.subscribers = append(tp.subscribers, ch)
	tp.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel.
func (tp *ThreePhase) Unsubscribe(ch chan StepExecution) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	for i, s := range tp.subscribers {
		if s == ch {
			tp.subscribers = append(tp.subscribers[:i], tp.subscribers[i+1:]...)
			// Do not close here: executeNextStep broadcasts from a snapshot taken
			// outside the lock, which may still contain this channel.
			return
		}
	}
}

// State mirrors GetLoopStateResponse.
func (tp *ThreePhase) State() (step int32, phase CognitivePhase, cycles, steps uint64, running bool) {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	return int32(tp.currentStep), tp.currentPhase, tp.cyclesCompleted, tp.stepsExecuted, tp.running
}

// EngineStats returns completed-task counts for the three engines.
func (tp *ThreePhase) EngineStats() [3]uint64 {
	var out [3]uint64
	for i, e := range tp.engines {
		e.mu.RLock()
		out[i] = e.CompletedTasks
		e.mu.RUnlock()
	}
	return out
}

func stepType(step int) StepType {
	switch {
	case step == 1 || step == 7:
		return StepRelevanceRealization
	case step >= 2 && step <= 6:
		return StepAffordanceInteraction
	default:
		return StepSalienceSimulation
	}
}

func phaseForStep(step int) CognitivePhase {
	switch {
	case step >= 1 && step <= 4:
		return PhaseExpressive
	case step >= 5 && step <= 8:
		return PhaseReflective
	case step >= 9 && step <= 12:
		return PhaseIntegrative
	default:
		return PhaseExpressive
	}
}

// ---- Priority-queue event scheduler (from scheduler.go) ----

// EventType categorizes cognitive events.
type EventType int

const (
	EventThought EventType = iota
	EventPerception
	EventAction
	EventLearning
	EventMemoryConsolidation
	EventGoalPursuit
	EventIntrospection
	EventWake
	EventRest
)

// CognitiveEvent is a schedulable unit in the event queue.
type CognitiveEvent struct {
	ID          string
	Type        EventType
	Priority    int
	ScheduledAt time.Time
	Payload     string
}

// eventHeap implements heap.Interface ordered by priority then schedule time.
type eventHeap []*CognitiveEvent

func (h eventHeap) Len() int { return len(h) }
func (h eventHeap) Less(i, j int) bool {
	if h[i].Priority != h[j].Priority {
		return h[i].Priority > h[j].Priority
	}
	return h[i].ScheduledAt.Before(h[j].ScheduledAt)
}
func (h eventHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *eventHeap) Push(x any)   { *h = append(*h, x.(*CognitiveEvent)) }
func (h *eventHeap) Pop() any {
	old := *h
	n := len(old)
	e := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return e
}

// Scheduler is the goal-directed EchoBeats event scheduler.
type Scheduler struct {
	mu        sync.Mutex
	queue     eventHeap
	processed uint64
	scheduled uint64
}

// NewScheduler creates an empty priority-queue scheduler.
func NewScheduler() *Scheduler {
	s := &Scheduler{}
	heap.Init(&s.queue)
	return s
}

// Schedule pushes an event onto the queue.
func (s *Scheduler) Schedule(e *CognitiveEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e.ID == "" {
		e.ID = fmt.Sprintf("evt_%d", time.Now().UnixNano())
	}
	if e.ScheduledAt.IsZero() {
		e.ScheduledAt = time.Now()
	}
	heap.Push(&s.queue, e)
	s.scheduled++
}

// PopDue removes and returns the next due event, or nil.
func (s *Scheduler) PopDue() *CognitiveEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.queue) == 0 || time.Now().Before(s.queue[0].ScheduledAt) {
		return nil
	}
	s.processed++
	return heap.Pop(&s.queue).(*CognitiveEvent)
}

// Stats returns (scheduled, processed, pending).
func (s *Scheduler) Stats() (uint64, uint64, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scheduled, s.processed, len(s.queue)
}

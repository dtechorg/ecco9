package echobeats

import (
	"testing"
	"time"
)

func TestThreePhaseRestartsAfterStop(t *testing.T) {
	tp := NewThreePhase()
	if !tp.Start(40) {
		t.Fatal("first start was rejected")
	}
	time.Sleep(40 * time.Millisecond)
	if !tp.Stop() {
		t.Fatal("first stop was rejected")
	}
	_, _, _, firstSteps, _ := tp.State()
	if firstSteps == 0 {
		t.Fatal("first run emitted no steps")
	}

	if !tp.Start(40) {
		t.Fatal("restart was rejected")
	}
	time.Sleep(40 * time.Millisecond)
	if !tp.Stop() {
		t.Fatal("second stop was rejected")
	}
	_, _, _, secondSteps, _ := tp.State()
	if secondSteps <= firstSteps {
		t.Fatalf("restart did not advance the loop: before=%d after=%d", firstSteps, secondSteps)
	}
}

func TestPhaseForStepCoversThreeFourStepWindows(t *testing.T) {
	for step := 1; step <= 12; step++ {
		want := PhaseExpressive
		if step >= 5 && step <= 8 {
			want = PhaseReflective
		} else if step >= 9 {
			want = PhaseIntegrative
		}
		if got := phaseForStep(step); got != want {
			t.Fatalf("step %d: got %s, want %s", step, got, want)
		}
	}
}

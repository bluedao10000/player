package scheduler

import (
	"errors"
	"testing"
	"time"
)

func TestHeartbeatGateCountsFailuresAndThrottlesUnchangedState(t *testing.T) {
	start := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	var gate heartbeatGate

	state, consecutive, write := gate.observe(errors.New("first"), start)
	if state != "failed" || consecutive != 1 || !write {
		t.Fatalf("first failure = (%q, %d, %t)", state, consecutive, write)
	}
	gate.committed(state, start)

	state, consecutive, write = gate.observe(errors.New("second"), start.Add(10*time.Second))
	if state != "failed" || consecutive != 2 || write {
		t.Fatalf("throttled failure = (%q, %d, %t)", state, consecutive, write)
	}

	state, consecutive, write = gate.observe(errors.New("third"), start.Add(30*time.Second))
	if state != "failed" || consecutive != 3 || !write {
		t.Fatalf("failure at throttle boundary = (%q, %d, %t)", state, consecutive, write)
	}
	gate.committed(state, start.Add(30*time.Second))

	state, consecutive, write = gate.observe(nil, start.Add(31*time.Second))
	if state != "success" || consecutive != 0 || !write {
		t.Fatalf("recovery = (%q, %d, %t)", state, consecutive, write)
	}
	gate.committed(state, start.Add(31*time.Second))

	state, consecutive, write = gate.observe(nil, start.Add(61*time.Second))
	if state != "success" || consecutive != 0 || !write {
		t.Fatalf("success at throttle boundary = (%q, %d, %t)", state, consecutive, write)
	}
}

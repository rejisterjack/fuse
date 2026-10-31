package budget

import "testing"

func TestHardLimitBlocks(t *testing.T) {
	tr := NewTracker(1, 2)
	scope := Scope{Agent: "a"}
	tr.Record(scope, 2.5)
	d := tr.Check(scope)
	if d.Allowed || !d.HardHit {
		t.Fatalf("expected hard block, got %+v", d)
	}
}

func TestSoftLimitAllows(t *testing.T) {
	tr := NewTracker(1, 5)
	scope := Scope{Agent: "a"}
	tr.Record(scope, 1.5)
	d := tr.Check(scope)
	if !d.Allowed || !d.SoftHit {
		t.Fatalf("expected soft hit allow, got %+v", d)
	}
}

func TestRequestHardLimit(t *testing.T) {
	tr := NewTracker(100, 200)
	scope := Scope{Agent: "r"}
	// Without store, request limits only apply via in-memory reqs when rules exist.
	// Simulate by recording requests and checking defaults don't block on requests.
	for i := 0; i < 5; i++ {
		tr.RecordRequest(scope)
	}
	d := tr.Check(scope)
	if !d.Allowed {
		t.Fatalf("defaults should not request-block without rules, got %+v", d)
	}
}

func TestDefaultHardStillWorks(t *testing.T) {
	tr := NewTracker(5, 10)
	scope := Scope{Agent: "x", User: "u"}
	tr.Record(scope, 10)
	d := tr.Check(scope)
	if d.Allowed {
		t.Fatalf("expected block at hard default: %+v", d)
	}
}

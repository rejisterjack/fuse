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

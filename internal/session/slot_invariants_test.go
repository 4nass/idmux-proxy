package session

import (
	"testing"
	"time"
)

func TestStateUpsertKeepsOtherIndexesStable(t *testing.T) {
	state := NewState()
	first, err := state.Add("first", 4)
	if err != nil {
		t.Fatalf("add first session: %v", err)
	}
	second, err := state.Add("second", 4)
	if err != nil {
		t.Fatalf("add second session: %v", err)
	}
	if first != 0 || second != 1 {
		t.Fatalf("unexpected initial indexes: first=%d second=%d", first, second)
	}

	if err := state.Upsert(first, "first-updated", 4); err != nil {
		t.Fatalf("update first session: %v", err)
	}
	if state.Sessions[0].IDPSessionID != "first-updated" ||
		state.Sessions[1].IDPSessionID != "second" ||
		state.Sessions[1].Index != 1 ||
		state.ActiveIndex != first {
		t.Fatalf("upsert changed another session or active index: %+v", state)
	}
	if err := state.Validate(4, time.Now()); err != nil {
		t.Fatalf("validate updated state: %v", err)
	}
}

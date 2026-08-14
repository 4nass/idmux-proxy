package session

import (
	"testing"
	"time"
)

func TestStateAddAndRemoveKeepsIndexesStable(t *testing.T) {
	state := NewState()
	first, err := state.Add("first", 3)
	if err != nil || first != 0 {
		t.Fatalf("add first session: index=%d err=%v", first, err)
	}
	second, err := state.Add("second", 3)
	if err != nil || second != 1 {
		t.Fatalf("add second session: index=%d err=%v", second, err)
	}
	if !state.Remove(0) {
		t.Fatal("remove first session: expected success")
	}
	if state.ActiveIndex != 1 || len(state.Sessions) != 2 || state.Sessions[0].IDPSessionID != "" || state.Sessions[1].Index != 1 || state.Sessions[1].IDPSessionID != "second" {
		t.Fatalf("state did not keep stable indexes: %+v", state)
	}
	third, err := state.Add("third", 3)
	if err != nil || third != 0 {
		t.Fatalf("reuse empty slot: index=%d err=%v", third, err)
	}
	if state.Sessions[1].IDPSessionID != "second" {
		t.Fatalf("reusing an empty slot changed another session: %+v", state.Sessions)
	}
	if err := state.Validate(3, time.Now()); err != nil {
		t.Fatalf("validate state: %v", err)
	}
}

func TestStateRejectsExpiredState(t *testing.T) {
	state := NewState()
	state.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	if err := state.Validate(3, time.Now()); err == nil {
		t.Fatal("expected expired state to be rejected")
	}
}

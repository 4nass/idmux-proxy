package session

import "testing"

func TestSealerRoundTripAndTamperRejection(t *testing.T) {
	sealer, err := NewSealer([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatalf("create sealer: %v", err)
	}
	state := NewState()
	if _, err := state.Add("opaque-idp-session", 4); err != nil {
		t.Fatalf("add state: %v", err)
	}
	value, err := sealer.Seal(state)
	if err != nil {
		t.Fatalf("seal state: %v", err)
	}
	opened, err := sealer.Open(value, 4)
	if err != nil {
		t.Fatalf("open state: %v", err)
	}
	if opened.Sessions[0].IDPSessionID != "opaque-idp-session" {
		t.Fatalf("unexpected opened session: %+v", opened.Sessions)
	}
	tampered := value[:len(value)-1] + "A"
	if _, err := sealer.Open(tampered, 4); err == nil {
		t.Fatal("expected tampered cookie to be rejected")
	}
}

func TestSealerReadsPreviousKeyDuringRotation(t *testing.T) {
	oldKey := []byte("01234567890123456789012345678901")
	newKey := []byte("abcdefghijklmnopqrstuvwxyz123456")
	oldSealer, err := NewSealer(oldKey)
	if err != nil {
		t.Fatalf("create old sealer: %v", err)
	}
	state := NewState()
	if _, err := state.Add("old-session", 4); err != nil {
		t.Fatalf("add state: %v", err)
	}
	value, err := oldSealer.Seal(state)
	if err != nil {
		t.Fatalf("seal old state: %v", err)
	}
	rotatedSealer, err := NewSealer(newKey, oldKey)
	if err != nil {
		t.Fatalf("create rotated sealer: %v", err)
	}
	opened, err := rotatedSealer.Open(value, 4)
	if err != nil || opened.Sessions[0].IDPSessionID != "old-session" {
		t.Fatalf("previous key was not accepted: state=%+v err=%v", opened, err)
	}
	newValue, err := rotatedSealer.Seal(state)
	if err != nil {
		t.Fatalf("seal with active key: %v", err)
	}
	if _, err := oldSealer.Open(newValue, 4); err == nil {
		t.Fatal("old key unexpectedly opened a cookie sealed with the new key")
	}
}

func FuzzSealerOpenDoesNotPanic(f *testing.F) {
	f.Add("invalid")
	f.Fuzz(func(t *testing.T, value string) {
		sealer, err := NewSealer([]byte("01234567890123456789012345678901"))
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("sealer panicked: %v", recovered)
			}
		}()
		_, _ = sealer.Open(value, 8)
	})
}

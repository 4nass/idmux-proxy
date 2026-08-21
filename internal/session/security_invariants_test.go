package session

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestStateValidateRejectsBrokenInvariants(t *testing.T) {
	tests := []struct {
		name  string
		state State
		max   int
	}{
		{
			name:  "unsupported version",
			state: State{Version: CurrentVersion + 1, ActiveIndex: -1},
			max:   4,
		},
		{
			name: "too many sessions",
			state: func() State {
				state := NewState()
				_, _ = state.Add("first", 4)
				_, _ = state.Add("second", 4)
				return state
			}(),
			max: 1,
		},
		{
			name: "unstable index",
			state: func() State {
				state := NewState()
				_, _ = state.Add("session", 4)
				state.Sessions[0].Index = 1
				return state
			}(),
			max: 4,
		},
		{
			name: "metadata in empty slot",
			state: State{
				Version:     CurrentVersion,
				ActiveIndex: -1,
				Sessions:    []Entry{{Index: 0, UserID: "must-not-leak"}},
			},
			max: 4,
		},
		{
			name: "oversized IdP session ID",
			state: State{
				Version:     CurrentVersion,
				ActiveIndex: 0,
				Sessions: []Entry{{
					Index:        0,
					IDPSessionID: strings.Repeat("x", 4097),
				}},
			},
			max: 4,
		},
		{
			name: "missing active session",
			state: func() State {
				state := NewState()
				_, _ = state.Add("session", 4)
				state.ActiveIndex = 1
				return state
			}(),
			max: 4,
		},
		{
			name: "active index in empty state",
			state: State{
				Version:     CurrentVersion,
				ActiveIndex: 0,
			},
			max: 4,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.state.Validate(test.max, time.Now()); err == nil {
				t.Fatal("expected invalid state to be rejected")
			}
		})
	}
}

func TestSealerRejectsExpiredCookie(t *testing.T) {
	sealer := newInvariantTestSealer(t)
	state := NewState()
	state.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	value := sealInvariantPayload(t, sealer, state)

	if _, err := sealer.Open(value, 4); err == nil {
		t.Fatal("expected expired cookie to be rejected")
	}
}

func TestSealerRejectsOversizedCookie(t *testing.T) {
	sealer := newInvariantTestSealer(t)
	if _, err := sealer.Open(strings.Repeat("A", defaultCookieBytes+1), 4); err == nil {
		t.Fatal("expected oversized cookie to be rejected")
	}
}

func TestSealerRejectsTrailingPayload(t *testing.T) {
	sealer := newInvariantTestSealer(t)
	state := NewState()
	payload, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	payload = append(payload, []byte(" trailing-data")...)
	value := sealInvariantPayloadBytes(t, sealer, payload)

	if _, err := sealer.Open(value, 4); err == nil {
		t.Fatal("expected trailing payload to be rejected")
	}
}

func FuzzStateValidateDoesNotPanic(f *testing.F) {
	f.Add(CurrentVersion, -1, "session")
	f.Add(CurrentVersion+1, 0, strings.Repeat("x", 64))
	f.Fuzz(func(t *testing.T, version, active int, id string) {
		state := NewState()
		state.Version = version
		state.ActiveIndex = active
		if id != "" {
			state.Sessions = []Entry{{Index: 0, IDPSessionID: id}}
		}
		_ = state.Validate(8, time.Unix(1, 0))
	})
}

func newInvariantTestSealer(t *testing.T) *Sealer {
	t.Helper()
	sealer, err := NewSealer([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatalf("create sealer: %v", err)
	}
	return sealer
}

func sealInvariantPayload(t *testing.T, sealer *Sealer, state State) string {
	t.Helper()
	payload, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	return sealInvariantPayloadBytes(t, sealer, payload)
}

func sealInvariantPayloadBytes(t *testing.T, sealer *Sealer, payload []byte) string {
	t.Helper()
	nonce := make([]byte, sealer.active.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		t.Fatalf("create nonce: %v", err)
	}
	sealed := sealer.active.Seal(nonce, nonce, payload, sealer.associated)
	return base64.RawURLEncoding.EncodeToString(sealed)
}

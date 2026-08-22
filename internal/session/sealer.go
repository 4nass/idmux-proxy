package session

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

const defaultCookieBytes = 3800

type Sealer struct {
	active       cipher.AEAD
	readers      []cipher.AEAD
	maxCookieLen int
	associated   []byte
}

func NewSealer(keys ...[]byte) (*Sealer, error) {
	if len(keys) == 0 {
		return nil, errors.New("at least one cookie key is required")
	}
	readers := make([]cipher.AEAD, 0, len(keys))
	for index, key := range keys {
		if len(key) != 32 {
			return nil, fmt.Errorf("cookie key %d must be exactly 32 bytes", index)
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, fmt.Errorf("create AES cipher %d: %w", index, err)
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("create GCM cipher %d: %w", index, err)
		}
		readers = append(readers, aead)
	}
	return &Sealer{
		active:       readers[0],
		readers:      readers,
		maxCookieLen: defaultCookieBytes,
		associated:   []byte("idmux-proxy:session:v1"),
	}, nil
}

func (s *Sealer) Seal(state State) (string, error) {
	if err := state.Validate(0, time.Now()); err != nil {
		return "", fmt.Errorf("validate state before sealing: %w", err)
	}
	plaintext, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("encode session state: %w", err)
	}
	nonce := make([]byte, s.active.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("create session nonce: %w", err)
	}
	sealed := s.active.Seal(nonce, nonce, plaintext, s.associated)
	value := base64.RawURLEncoding.EncodeToString(sealed)
	if len(value) > s.maxCookieLen {
		return "", errors.New("encrypted session state is too large for one cookie")
	}
	return value, nil
}

func (s *Sealer) Open(value string, maxSessions int) (State, error) {
	if value == "" || len(value) > s.maxCookieLen {
		return State{}, errors.New("session cookie has an invalid size")
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return State{}, errors.New("session cookie is not valid base64")
	}
	if len(ciphertext) < s.active.NonceSize() {
		return State{}, errors.New("session cookie is too short")
	}
	nonce := ciphertext[:s.active.NonceSize()]
	encrypted := ciphertext[s.active.NonceSize():]
	for _, reader := range s.readers {
		plaintext, openErr := reader.Open(nil, nonce, encrypted, s.associated)
		if openErr != nil {
			continue
		}
		var state State
		decoder := json.NewDecoder(bytes.NewReader(plaintext))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&state); err != nil {
			return State{}, errors.New("session cookie payload is invalid")
		}
		var trailing any
		if err := decoder.Decode(&trailing); err != io.EOF {
			return State{}, errors.New("session cookie payload has trailing data")
		}
		if err := state.Validate(maxSessions, time.Now()); err != nil {
			return State{}, fmt.Errorf("validate session cookie: %w", err)
		}
		return state, nil
	}
	return State{}, errors.New("session cookie authentication failed")
}

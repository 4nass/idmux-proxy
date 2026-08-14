package session

import (
	"errors"
	"fmt"
	"time"
)

const CurrentVersion = 1

var (
	ErrEmptyIDPSession = errors.New("idp session ID is empty")
	ErrInvalidIndex    = errors.New("session index is invalid")
	ErrTooManySessions = errors.New("maximum session count reached")
)

// Entry is the small amount of state needed to select one IdP session.
// The IDP session ID is stored only inside the encrypted cookie.
type Entry struct {
	Index        int    `json:"index"`
	UserID       string `json:"user_id,omitempty"`
	DisplayName  string `json:"display_name,omitempty"`
	IDPSessionID string `json:"idp_session_id"`
	CreatedAt    int64  `json:"created_at"`
}

// State is the encrypted payload stored in the browser cookie.
type State struct {
	Version     int     `json:"version"`
	ActiveIndex int     `json:"active_index"`
	Sessions    []Entry `json:"sessions"`
	IssuedAt    int64   `json:"issued_at"`
	ExpiresAt   int64   `json:"expires_at"`
}

func NewState() State {
	return State{Version: CurrentVersion, ActiveIndex: -1, Sessions: []Entry{}}
}

func (s State) Clone() State {
	s.Sessions = append([]Entry(nil), s.Sessions...)
	return s
}

func (s State) Entry(index int) (Entry, bool) {
	if index < 0 || index >= len(s.Sessions) {
		return Entry{}, false
	}
	entry := s.Sessions[index]
	if entry.Index != index || entry.IDPSessionID == "" {
		return Entry{}, false
	}
	return entry, true
}

func (s State) ActiveCount() int {
	count := 0
	for _, entry := range s.Sessions {
		if entry.IDPSessionID != "" {
			count++
		}
	}
	return count
}

func (s State) Validate(maxSessions int, now time.Time) error {
	if s.Version != CurrentVersion {
		return fmt.Errorf("unsupported state version %d", s.Version)
	}
	if maxSessions > 0 && len(s.Sessions) > maxSessions {
		return ErrTooManySessions
	}
	if s.ExpiresAt > 0 && now.Unix() >= s.ExpiresAt {
		return errors.New("session state has expired")
	}

	present := 0
	for index, entry := range s.Sessions {
		if entry.Index != index {
			return fmt.Errorf("session indexes are not stable")
		}
		if entry.IDPSessionID == "" {
			if entry.UserID != "" || entry.DisplayName != "" || entry.CreatedAt != 0 {
				return errors.New("empty session slot contains metadata")
			}
			continue
		}
		present++
		if len(entry.IDPSessionID) > 4096 {
			return errors.New("idp session ID is too long")
		}
	}
	if present == 0 {
		if s.ActiveIndex != -1 {
			return fmt.Errorf("empty state has active index %d", s.ActiveIndex)
		}
		return nil
	}
	if _, ok := s.Entry(s.ActiveIndex); !ok {
		return fmt.Errorf("active index %d is not present", s.ActiveIndex)
	}
	return nil
}

func (s *State) Add(idpSessionID string, maxSessions int) (int, error) {
	if idpSessionID == "" {
		return -1, ErrEmptyIDPSession
	}
	for index, entry := range s.Sessions {
		if entry.IDPSessionID == "" {
			s.Sessions[index] = Entry{
				Index:        index,
				IDPSessionID: idpSessionID,
				CreatedAt:    time.Now().Unix(),
			}
			s.ActiveIndex = index
			return index, nil
		}
	}
	if maxSessions > 0 && len(s.Sessions) >= maxSessions {
		return -1, ErrTooManySessions
	}
	index := len(s.Sessions)
	s.Sessions = append(s.Sessions, Entry{
		Index:        index,
		IDPSessionID: idpSessionID,
		CreatedAt:    time.Now().Unix(),
	})
	s.ActiveIndex = index
	return index, nil
}

func (s *State) Upsert(index int, idpSessionID string, maxSessions int) error {
	if index < 0 {
		return ErrInvalidIndex
	}
	if idpSessionID == "" {
		return ErrEmptyIDPSession
	}
	if index < len(s.Sessions) {
		s.Sessions[index].IDPSessionID = idpSessionID
		if s.Sessions[index].CreatedAt == 0 {
			s.Sessions[index].CreatedAt = time.Now().Unix()
		}
		s.ActiveIndex = index
		return nil
	}
	if index != len(s.Sessions) {
		return ErrInvalidIndex
	}
	_, err := s.Add(idpSessionID, maxSessions)
	return err
}

func (s *State) Remove(index int) bool {
	if _, ok := s.Entry(index); !ok {
		return false
	}
	s.Sessions[index] = Entry{Index: index}
	for len(s.Sessions) > 0 && s.Sessions[len(s.Sessions)-1].IDPSessionID == "" {
		s.Sessions = s.Sessions[:len(s.Sessions)-1]
	}
	if len(s.Sessions) == 0 {
		s.ActiveIndex = -1
		return true
	}
	if s.ActiveIndex == index {
		s.ActiveIndex = s.firstPresent()
	}
	return true
}

func (s State) firstPresent() int {
	for index, entry := range s.Sessions {
		if entry.IDPSessionID != "" {
			return index
		}
	}
	return -1
}

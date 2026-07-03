package update

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type StateStore struct {
	path string
}

func NewStateStore(path string) *StateStore {
	return &StateStore{path: path}
}

func (s *StateStore) Load() (State, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{Phase: PhaseIdle}, nil
		}
		return State{}, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, fmt.Errorf("parse state: %w", err)
	}
	if st.Phase == "" {
		st.Phase = PhaseIdle
	}
	return st, nil
}

func (s *StateStore) Save(st State) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if st.StartedAt == "" {
		st.StartedAt = now
	}
	st.UpdatedAt = now
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *StateStore) SetPhase(st State, phase Phase) error {
	st.Phase = phase
	st.Error = ""
	return s.Save(st)
}

func (s *StateStore) Fail(st State, phase Phase, err error) error {
	st.Phase = phase
	if err != nil {
		st.Error = err.Error()
	}
	return s.Save(st)
}

func (s *StateStore) Reset() error {
	return s.Save(State{Phase: PhaseIdle})
}

package web

import (
	"strings"
	"sync"
	"time"

	"github.com/belaytzev/tdmeter/checker"
)

// StatusStore holds the latest proxy check results in a thread-safe manner.
type StatusStore struct {
	mu        sync.RWMutex
	results   []checker.Result
	lastCheck time.Time
}

// NewStatusStore creates an empty StatusStore.
func NewStatusStore() *StatusStore {
	return &StatusStore{}
}

// Update replaces stored results with a copy of the provided slice.
func (s *StatusStore) Update(results []checker.Result) {
	cp := make([]checker.Result, len(results))
	copy(cp, results)

	s.mu.Lock()
	s.results = cp
	s.lastCheck = time.Now()
	s.mu.Unlock()
}

// Results returns a copy of the latest results and the time of the last check.
func (s *StatusStore) Results() ([]checker.Result, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cp := make([]checker.Result, len(s.results))
	copy(cp, s.results)
	return cp, s.lastCheck
}

// FindByName returns the result for the given proxy name (case-insensitive).
func (s *StatusStore) FindByName(name string) (checker.Result, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lower := strings.ToLower(name)
	for _, r := range s.results {
		if strings.ToLower(r.Name) == lower {
			return r, true
		}
	}
	return checker.Result{}, false
}

package fixture

import (
	"errors"
	"fmt"
	"sync"
)

// Sequence replays explicitly ordered responses, including repeated requests and cursor pages.
// Tests must check Done even when adapter code classifies a mismatch as a report failure.
type Sequence struct {
	mu      sync.Mutex
	steps   []File
	failure error
}

func NewSequence(steps ...File) (*Sequence, error) {
	for i := range steps {
		if err := steps[i].Validate(); err != nil {
			return nil, err
		}
		if steps[i].Source != "synthetic" {
			return nil, errors.New("replay sequences require synthetic responses, not recorded evidence")
		}
	}
	return &Sequence{steps: append([]File(nil), steps...)}, nil
}

func (s *Sequence) Next(request Request) (File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != nil {
		return File{}, s.failure
	}
	if len(s.steps) == 0 {
		s.failure = errors.New("unexpected request after replay sequence completed")
		return File{}, s.failure
	}
	expected := s.steps[0]
	if request.Key() != expected.Request.Key() {
		s.failure = fmt.Errorf("unexpected request %s; expected %s", request.Key(), expected.Request.Key())
		return File{}, s.failure
	}
	s.steps = s.steps[1:]
	return expected, nil
}

func (s *Sequence) Done() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != nil {
		return s.failure
	}
	if len(s.steps) > 0 {
		return fmt.Errorf("%d expected replay steps were not consumed", len(s.steps))
	}
	return nil
}

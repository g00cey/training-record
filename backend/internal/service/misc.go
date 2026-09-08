package service

import (
	"training-record/internal/domain"
)

// ExerciseNames returns the sorted DISTINCT exercise-name master list.
func (s *Service) ExerciseNames() ([]string, error) { return s.st.ExerciseNames() }

// GetProfile returns the user profile.
func (s *Service) GetProfile() (domain.Profile, error) { return s.st.GetProfile() }

// UpdateProfile applies the provided (non-nil) fields.
func (s *Service) UpdateProfile(bodyweightKg, heightCm *float64, maxHrEst *int) (domain.Profile, error) {
	return s.st.UpdateProfile(bodyweightKg, heightCm, maxHrEst)
}

// LastSessions returns the most recent strength and spin sessions.
func (s *Service) LastSessions() (*domain.StrengthSession, *domain.SpinSession, error) {
	st, err := s.st.LastStrength()
	if err != nil {
		return nil, nil, err
	}
	sp, err := s.st.LastSpin()
	if err != nil {
		return nil, nil, err
	}
	return st, sp, nil
}

// History returns the most recent strength and spin sessions (limit each).
func (s *Service) History(limit int) ([]domain.StrengthSession, []domain.SpinSession, error) {
	st, _, err := s.st.ListStrength("", "", limit, 0)
	if err != nil {
		return nil, nil, err
	}
	sp, _, err := s.st.ListSpin("", "", limit, 0)
	if err != nil {
		return nil, nil, err
	}
	return st, sp, nil
}

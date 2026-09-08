package service

import (
	"errors"

	"training-record/internal/apperr"
	"training-record/internal/domain"
	"training-record/internal/store"
)

// ListStrength returns sessions in [from,to] plus the total count.
func (s *Service) ListStrength(from, to string, limit, offset int) ([]domain.StrengthSession, int, error) {
	return s.st.ListStrength(from, to, limit, offset)
}

// GetStrength returns the session for date or an apperr.NotFound.
func (s *Service) GetStrength(date string) (*domain.StrengthSession, error) {
	sess, err := s.st.GetStrengthByDate(date)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, apperr.NotFoundf("strength session %s not found", date)
	}
	return sess, nil
}

// CreateStrength inserts a new session; apperr.Conflict if the date exists.
func (s *Service) CreateStrength(date, notes string, exs []ExerciseInput) (*domain.StrengthSession, error) {
	sess, err := s.st.CreateStrength(date, notes, toDomainExercises(exs))
	if errors.Is(err, store.ErrDateExists) {
		return nil, apperr.Conflictf("strength session %s already exists", date)
	}
	return sess, err
}

// ReplaceStrength upserts the session for date. created reports insertion.
func (s *Service) ReplaceStrength(date, notes string, exs []ExerciseInput) (*domain.StrengthSession, bool, error) {
	return s.st.ReplaceStrength(date, notes, toDomainExercises(exs))
}

// PatchStrengthNotes updates only notes; apperr.NotFound if absent.
func (s *Service) PatchStrengthNotes(date, notes string) (*domain.StrengthSession, error) {
	sess, err := s.st.UpdateStrengthNotes(date, notes)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, apperr.NotFoundf("strength session %s not found", date)
	}
	return sess, nil
}

// AppendExercises appends rows to (or creates) the session for date.
func (s *Service) AppendExercises(date string, exs []ExerciseInput, notes string, hasNotes bool) (*domain.StrengthSession, error) {
	return s.st.AppendStrengthExercises(date, toDomainExercises(exs), notes, hasNotes)
}

// UpdateExercise replaces one exercise row in the session for date.
func (s *Service) UpdateExercise(date string, id int64, in ExerciseInput) (*domain.StrengthSession, error) {
	sets := in.Sets
	if sets < 1 {
		sets = 1
	}
	sess, err := s.st.UpdateExercise(date, id, domain.Exercise{
		Name: in.Name, Weight: in.Weight, Reps: in.Reps, Sets: sets, Notes: in.Notes,
	})
	if errors.Is(err, store.ErrNotFound) {
		return nil, apperr.NotFoundf("strength session %s not found", date)
	}
	if errors.Is(err, store.ErrExerciseNotFound) {
		return nil, apperr.NotFoundf("exercise %d not found in session %s", id, date)
	}
	return sess, err
}

// DeleteExercise removes one exercise row; apperr.NotFound if absent.
func (s *Service) DeleteExercise(date string, id int64) error {
	found, err := s.st.DeleteExercise(date, id)
	if err != nil {
		return err
	}
	if !found {
		return apperr.NotFoundf("exercise %d not found in session %s", id, date)
	}
	return nil
}

// ReorderExercises applies a new order; unknown ids -> apperr.Unprocessable.
func (s *Service) ReorderExercises(date string, ids []int64) (*domain.StrengthSession, error) {
	sess, err := s.st.ReorderExercises(date, ids)
	if errors.Is(err, store.ErrNotFound) {
		return nil, apperr.NotFoundf("strength session %s not found", date)
	}
	if errors.Is(err, store.ErrExerciseNotFound) {
		return nil, apperr.Unprocessablef("orderedIds must contain exactly the exercise ids of session %s", date)
	}
	return sess, err
}

// DeleteStrength removes the session for date; apperr.NotFound if absent.
func (s *Service) DeleteStrength(date string) error {
	found, err := s.st.DeleteStrength(date)
	if err != nil {
		return err
	}
	if !found {
		return apperr.NotFoundf("strength session %s not found", date)
	}
	return nil
}

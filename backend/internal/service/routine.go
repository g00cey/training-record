package service

import (
	"errors"

	"training-record/internal/apperr"
	"training-record/internal/domain"
	"training-record/internal/store"
)

// RoutineExerciseInput is a validated routine exercise from a request body.
type RoutineExerciseInput struct {
	Name   string
	Weight *float64
	Reps   int
	Sets   int
}

// CurrentRoutine returns the latest snapshot; apperr.NotFound "no routine".
func (s *Service) CurrentRoutine() (*domain.RoutineSnapshot, error) {
	date, err := s.st.LatestRoutineDate()
	if err != nil {
		return nil, err
	}
	if date == "" {
		return nil, apperr.NotFoundf("no routine")
	}
	return s.routineAt(date)
}

// RoutineHistory returns every snapshot summary, newest first.
func (s *Service) RoutineHistory() ([]store.RoutineHistoryEntry, error) {
	return s.st.RoutineHistory()
}

// RoutineByDate returns the snapshot for date; apperr.NotFound if none.
func (s *Service) RoutineByDate(date string) (*domain.RoutineSnapshot, error) {
	exs, err := s.st.RoutineByDate(date)
	if err != nil {
		return nil, err
	}
	if len(exs) == 0 {
		return nil, apperr.NotFoundf("no routine snapshot for %s", date)
	}
	return &domain.RoutineSnapshot{Date: date, Exercises: exs}, nil
}

func (s *Service) routineAt(date string) (*domain.RoutineSnapshot, error) {
	exs, err := s.st.RoutineByDate(date)
	if err != nil {
		return nil, err
	}
	return &domain.RoutineSnapshot{Date: date, Exercises: exs}, nil
}

// ReplaceRoutine stores a new snapshot. An empty date means "today" (JST).
func (s *Service) ReplaceRoutine(date string, in []RoutineExerciseInput) (*domain.RoutineSnapshot, error) {
	if date == "" {
		date = domain.Today()
	}
	exs := make([]domain.RoutineExercise, len(in))
	for i, e := range in {
		sets := e.Sets
		if sets < 1 {
			sets = 1
		}
		exs[i] = domain.RoutineExercise{Name: e.Name, Weight: e.Weight, Reps: e.Reps, Sets: sets}
	}
	if err := s.st.ReplaceRoutine(date, exs); err != nil {
		return nil, err
	}
	return s.routineAt(date)
}

// PatchRoutineExercise updates/append one exercise in the latest snapshot.
func (s *Service) PatchRoutineExercise(name string, weight *float64, reps, sets *int) (action, routineDate string, err error) {
	action, routineDate, err = s.st.UpdateRoutineExercise(name, weight, reps, sets)
	if errors.Is(err, store.ErrNotFound) {
		return "", "", apperr.NotFoundf("no routine")
	}
	return action, routineDate, err
}

// Package service holds the domain/business logic: input orchestration,
// RPE derivation, and the ported analytics (calendar, volume, summary,
// weekly summary, load report). It maps store sentinels to apperr.E.
package service

import (
	"training-record/internal/domain"
	"training-record/internal/store"
)

// Service is the application service layer.
type Service struct {
	st *store.Store
}

// New builds a Service over st.
func New(st *store.Store) *Service { return &Service{st: st} }

// ExerciseInput is a validated exercise row from a request body.
type ExerciseInput struct {
	Name   string
	Weight *float64
	Reps   int
	Sets   int
	Notes  string
}

func toDomainExercises(in []ExerciseInput) []domain.Exercise {
	out := make([]domain.Exercise, len(in))
	for i, e := range in {
		sets := e.Sets
		if sets < 1 {
			sets = 1
		}
		out[i] = domain.Exercise{
			Name:   e.Name,
			Weight: e.Weight,
			Reps:   e.Reps,
			Sets:   sets,
			Notes:  e.Notes,
		}
	}
	return out
}

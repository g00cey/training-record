package service

import (
	"errors"

	"training-record/internal/apperr"
	"training-record/internal/domain"
	"training-record/internal/store"
)

// SpinInput is a validated POST/PUT spin body.
type SpinInput struct {
	Date            string
	DurationMinutes int
	AvgHeartRate    *int
	MaxHeartRate    *int
	RPE             *int // nil => derive from HR when possible
	DistanceKm      *float64
	Notes           string
}

// SpinPatch is a validated PATCH spin body. The *Set flags distinguish an
// absent key from an explicit null.
type SpinPatch struct {
	DurationMinutes *int
	AvgHeartRate    *int
	AvgHRSet        bool
	MaxHeartRate    *int
	MaxHRSet        bool
	RPE             *int
	RPESet          bool
	DistanceKm      *float64
	DistanceSet     bool
	Notes           *string
}

func applyAutoRPE(sp *domain.SpinSession) {
	if sp.RPE == nil {
		if v, ok := domain.AutoRPE(sp.AvgHeartRate, sp.MaxHeartRate); ok {
			sp.RPE = &v
		}
	}
}

// ListSpin returns spin sessions in [from,to] plus the total count.
func (s *Service) ListSpin(from, to string, limit, offset int) ([]domain.SpinSession, int, error) {
	return s.st.ListSpin(from, to, limit, offset)
}

// GetSpin returns the spin session for date or an apperr.NotFound.
func (s *Service) GetSpin(date string) (*domain.SpinSession, error) {
	sp, err := s.st.GetSpinByDate(date)
	if err != nil {
		return nil, err
	}
	if sp == nil {
		return nil, apperr.NotFoundf("spin session %s not found", date)
	}
	return sp, nil
}

func (in SpinInput) toDomain() domain.SpinSession {
	sp := domain.SpinSession{
		Date:            in.Date,
		DurationMinutes: in.DurationMinutes,
		AvgHeartRate:    in.AvgHeartRate,
		MaxHeartRate:    in.MaxHeartRate,
		RPE:             in.RPE,
		DistanceKm:      in.DistanceKm,
		Notes:           in.Notes,
	}
	applyAutoRPE(&sp)
	return sp
}

// CreateSpin inserts a new spin session; apperr.Conflict if date exists.
func (s *Service) CreateSpin(in SpinInput) (*domain.SpinSession, error) {
	sp, err := s.st.CreateSpin(in.toDomain())
	if errors.Is(err, store.ErrDateExists) {
		return nil, apperr.Conflictf("spin session %s already exists", in.Date)
	}
	return sp, err
}

// ReplaceSpin upserts the spin session for date. created reports insertion.
func (s *Service) ReplaceSpin(date string, in SpinInput) (*domain.SpinSession, bool, error) {
	in.Date = date
	return s.st.ReplaceSpin(in.toDomain())
}

// PatchSpin merges p into the spin session for date and re-derives RPE
// when it ends up null with HR data present. apperr.NotFound if absent.
func (s *Service) PatchSpin(date string, p SpinPatch) (*domain.SpinSession, error) {
	cur, err := s.st.GetSpinByDate(date)
	if err != nil {
		return nil, err
	}
	if cur == nil {
		return nil, apperr.NotFoundf("spin session %s not found", date)
	}
	if p.DurationMinutes != nil {
		cur.DurationMinutes = *p.DurationMinutes
	}
	if p.AvgHRSet {
		cur.AvgHeartRate = p.AvgHeartRate
	}
	if p.MaxHRSet {
		cur.MaxHeartRate = p.MaxHeartRate
	}
	if p.RPESet {
		cur.RPE = p.RPE
	}
	if p.DistanceSet {
		cur.DistanceKm = p.DistanceKm
	}
	if p.Notes != nil {
		cur.Notes = *p.Notes
	}
	applyAutoRPE(cur)

	out, err := s.st.UpdateSpinFull(*cur)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, apperr.NotFoundf("spin session %s not found", date)
	}
	return out, nil
}

// DeleteSpin removes the spin session for date; apperr.NotFound if absent.
func (s *Service) DeleteSpin(date string) error {
	found, err := s.st.DeleteSpin(date)
	if err != nil {
		return err
	}
	if !found {
		return apperr.NotFoundf("spin session %s not found", date)
	}
	return nil
}

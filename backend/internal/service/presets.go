package service

import (
	"errors"

	"training-record/internal/apperr"
	"training-record/internal/domain"
	"training-record/internal/store"
)

// RoutineExerciseInput is a validated routine/preset exercise from a body.
type RoutineExerciseInput struct {
	Name   string
	Weight *float64
	Reps   int
	Sets   int
}

// PresetSnapshot is one preset's current (or historical) content.
type PresetSnapshot struct {
	Name      string
	Date      string
	Exercises []domain.RoutineExercise
}

// CombinedRoutine is the legacy /api/routine merged read view.
type CombinedRoutine struct {
	Date      string // "" when no preset has a snapshot
	Presets   []string
	Exercises []domain.RoutineExercise
}

func toRoutineExercises(in []RoutineExerciseInput) []domain.RoutineExercise {
	out := make([]domain.RoutineExercise, len(in))
	for i, e := range in {
		sets := e.Sets
		if sets < 1 {
			sets = 1
		}
		out[i] = domain.RoutineExercise{Name: e.Name, Weight: e.Weight, Reps: e.Reps, Sets: sets}
	}
	return out
}

// ---------------------------------------------------------------------------
// presets
// ---------------------------------------------------------------------------

// ListPresets returns all presets with latest-snapshot summaries.
func (s *Service) ListPresets() ([]store.PresetInfo, error) {
	return s.st.ListPresets()
}

// GetPreset returns a preset's latest snapshot; apperr.NotFound when the
// preset is unknown or has no snapshot.
func (s *Service) GetPreset(name string) (*PresetSnapshot, error) {
	p, err := s.st.GetPreset(name)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, apperr.NotFoundf("preset %q not found", name)
	}
	date, err := s.st.LatestPresetDate(name)
	if err != nil {
		return nil, err
	}
	if date == "" {
		return nil, apperr.NotFoundf("preset %q has no snapshot", name)
	}
	exs, err := s.st.PresetSnapshot(name, date)
	if err != nil {
		return nil, err
	}
	return &PresetSnapshot{Name: name, Date: date, Exercises: exs}, nil
}

// PresetHistory returns a preset's snapshot history; apperr.NotFound when
// the preset is unknown.
func (s *Service) PresetHistory(name string) ([]store.RoutineHistoryEntry, error) {
	p, err := s.st.GetPreset(name)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, apperr.NotFoundf("preset %q not found", name)
	}
	return s.st.PresetHistory(name)
}

// GetPresetSnapshot returns a specific historical snapshot; apperr.NotFound
// when the preset or the dated snapshot is missing.
func (s *Service) GetPresetSnapshot(name, date string) (*PresetSnapshot, error) {
	p, err := s.st.GetPreset(name)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, apperr.NotFoundf("preset %q not found", name)
	}
	exs, err := s.st.PresetSnapshot(name, date)
	if err != nil {
		return nil, err
	}
	if len(exs) == 0 {
		return nil, apperr.NotFoundf("preset %q has no snapshot for %s", name, date)
	}
	return &PresetSnapshot{Name: name, Date: date, Exercises: exs}, nil
}

// CreatePreset adds a new preset; apperr.Conflict when the name is taken.
func (s *Service) CreatePreset(name string, sortOrder *int) (*store.Preset, error) {
	p, err := s.st.CreatePreset(name, sortOrder)
	if errors.Is(err, store.ErrPresetExists) {
		return nil, apperr.Conflictf("preset %q already exists", name)
	}
	return p, err
}

// ReorderPresets applies a new preset order. `order` must be a permutation of
// the existing preset names (every preset listed exactly once, no unknowns) so
// a malformed request fails loudly instead of silently no-op'ing.
func (s *Service) ReorderPresets(order []string) ([]store.PresetInfo, error) {
	infos, err := s.st.ListPresets()
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(infos))
	for _, in := range infos {
		known[in.Name] = true
	}
	seen := make(map[string]bool, len(order))
	for _, n := range order {
		if !known[n] {
			return nil, apperr.Invalidf("unknown preset %q in order", n)
		}
		if seen[n] {
			return nil, apperr.Invalidf("preset %q listed twice in order", n)
		}
		seen[n] = true
	}
	if len(order) != len(infos) {
		return nil, apperr.Invalidf("order must list all %d presets, got %d", len(infos), len(order))
	}
	return s.st.ReorderPresets(order)
}

// DeletePreset removes a preset and its snapshots; apperr.NotFound if unknown.
func (s *Service) DeletePreset(name string) error {
	found, err := s.st.DeletePreset(name)
	if err != nil {
		return err
	}
	if !found {
		return apperr.NotFoundf("preset %q not found", name)
	}
	return nil
}

// ReplacePreset pushes a new snapshot for a preset (history +1). The preset
// is created (appended) if it does not exist. Empty date means today (JST).
func (s *Service) ReplacePreset(name, date string, in []RoutineExerciseInput) (*PresetSnapshot, error) {
	if date == "" {
		date = domain.Today()
	}
	if err := s.st.EnsurePreset(name); err != nil {
		return nil, err
	}
	if err := s.st.ReplacePresetSnapshot(name, date, toRoutineExercises(in)); err != nil {
		return nil, err
	}
	exs, err := s.st.PresetSnapshot(name, date)
	if err != nil {
		return nil, err
	}
	return &PresetSnapshot{Name: name, Date: date, Exercises: exs}, nil
}

// PatchPresetExercise updates or appends one exercise in a preset's latest
// snapshot; apperr.NotFound when the preset does not exist.
func (s *Service) PatchPresetExercise(name, exName string, weight *float64, reps, sets *int) (action, presetDate string, err error) {
	action, presetDate, err = s.st.UpdatePresetExercise(name, exName, weight, reps, sets)
	if errors.Is(err, store.ErrNotFound) {
		return "", "", apperr.NotFoundf("preset %q not found", name)
	}
	return action, presetDate, err
}

// ---------------------------------------------------------------------------
// legacy /api/routine
// ---------------------------------------------------------------------------

// CurrentRoutine returns the merged latest snapshots of every preset in
// sortOrder order; apperr.NotFound only when no preset exists at all.
func (s *Service) CurrentRoutine() (*CombinedRoutine, error) {
	date, presets, exs, presetCount, err := s.st.CombinedLatestRoutine()
	if err != nil {
		return nil, err
	}
	if presetCount == 0 {
		return nil, apperr.NotFoundf("no routine")
	}
	return &CombinedRoutine{Date: date, Presets: presets, Exercises: exs}, nil
}

// RoutineHistory returns the all-preset snapshot history (counts summed).
func (s *Service) RoutineHistory() ([]store.RoutineHistoryEntry, error) {
	return s.st.CombinedRoutineHistory()
}

// RoutineByDate returns every preset's snapshot for date, merged;
// apperr.NotFound if none.
func (s *Service) RoutineByDate(date string) (*CombinedRoutine, error) {
	presets, exs, found, err := s.st.CombinedRoutineByDate(date)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, apperr.NotFoundf("no routine snapshot for %s", date)
	}
	return &CombinedRoutine{Date: date, Presets: presets, Exercises: exs}, nil
}

// CrossPatchRoutineExercise updates the given exercise in the first preset
// (sortOrder order) whose latest snapshot contains it; apperr.NotFound if
// no preset has it. Never adds.
func (s *Service) CrossPatchRoutineExercise(name string, weight *float64, reps, sets *int) (preset, presetDate string, err error) {
	preset, presetDate, err = s.st.CrossPresetUpdateExercise(name, weight, reps, sets)
	if errors.Is(err, store.ErrNotFound) {
		return "", "", apperr.NotFoundf("exercise %q not found in any preset's latest snapshot", name)
	}
	return preset, presetDate, err
}

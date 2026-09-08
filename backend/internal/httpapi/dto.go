package httpapi

import (
	"training-record/internal/domain"
	"training-record/internal/service"
	"training-record/internal/store"
)

// exerciseDTO is the API representation of a performed exercise.
type exerciseDTO struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Weight    *float64 `json:"weight"`
	Reps      int      `json:"reps"`
	Sets      int      `json:"sets"`
	Notes     string   `json:"notes"`
	SortOrder int      `json:"sortOrder"`
}

// strengthSessionDTO is the API representation of a strength session.
type strengthSessionDTO struct {
	Date      string        `json:"date"`
	Notes     string        `json:"notes"`
	CreatedAt string        `json:"createdAt"`
	UpdatedAt string        `json:"updatedAt"`
	Exercises []exerciseDTO `json:"exercises"`
}

func toStrengthDTO(s *domain.StrengthSession) strengthSessionDTO {
	exs := make([]exerciseDTO, 0, len(s.Exercises))
	for _, e := range s.Exercises {
		exs = append(exs, exerciseDTO{
			ID: e.ID, Name: e.Name, Weight: e.Weight, Reps: e.Reps,
			Sets: e.Sets, Notes: e.Notes, SortOrder: e.SortOrder,
		})
	}
	return strengthSessionDTO{
		Date:      s.Date,
		Notes:     s.Notes,
		CreatedAt: domain.DBTimeToRFC3339(s.CreatedAt),
		UpdatedAt: domain.DBTimeToRFC3339(s.UpdatedAt),
		Exercises: exs,
	}
}

// spinSessionDTO is the API representation of a spin session.
type spinSessionDTO struct {
	Date            string   `json:"date"`
	DurationMinutes int      `json:"durationMinutes"`
	AvgHeartRate    *int     `json:"avgHeartRate"`
	MaxHeartRate    *int     `json:"maxHeartRate"`
	RPE             *int     `json:"rpe"`
	DistanceKm      *float64 `json:"distanceKm"`
	Notes           string   `json:"notes"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
}

func toSpinDTO(s *domain.SpinSession) spinSessionDTO {
	return spinSessionDTO{
		Date:            s.Date,
		DurationMinutes: s.DurationMinutes,
		AvgHeartRate:    s.AvgHeartRate,
		MaxHeartRate:    s.MaxHeartRate,
		RPE:             s.RPE,
		DistanceKm:      s.DistanceKm,
		Notes:           s.Notes,
		CreatedAt:       domain.DBTimeToRFC3339(s.CreatedAt),
		UpdatedAt:       domain.DBTimeToRFC3339(s.UpdatedAt),
	}
}

// routineExerciseDTO is the API representation of a routine/preset exercise
// (exercise_name -> name).
type routineExerciseDTO struct {
	Name   string   `json:"name"`
	Weight *float64 `json:"weight"`
	Reps   int      `json:"reps"`
	Sets   int      `json:"sets"`
}

func toRoutineExerciseDTOs(exs []domain.RoutineExercise) []routineExerciseDTO {
	out := make([]routineExerciseDTO, 0, len(exs))
	for _, e := range exs {
		out = append(out, routineExerciseDTO{Name: e.Name, Weight: e.Weight, Reps: e.Reps, Sets: e.Sets})
	}
	return out
}

// presetInfoDTO is one row of GET /api/presets.
type presetInfoDTO struct {
	Name          string  `json:"name"`
	SortOrder     int     `json:"sortOrder"`
	ExerciseCount int     `json:"exerciseCount"`
	LatestDate    *string `json:"latestDate"`
}

func toPresetInfoDTO(p store.PresetInfo) presetInfoDTO {
	return presetInfoDTO{
		Name: p.Name, SortOrder: p.SortOrder, ExerciseCount: p.ExerciseCount, LatestDate: p.LatestDate,
	}
}

// presetSnapshotDTO is GET/PUT /api/presets/{name} and .../{date}.
type presetSnapshotDTO struct {
	Name      string               `json:"name"`
	Date      string               `json:"date"`
	Exercises []routineExerciseDTO `json:"exercises"`
}

func toPresetSnapshotDTO(s *service.PresetSnapshot) presetSnapshotDTO {
	return presetSnapshotDTO{Name: s.Name, Date: s.Date, Exercises: toRoutineExerciseDTOs(s.Exercises)}
}

// combinedRoutineDTO is the legacy GET /api/routine merged view.
type combinedRoutineDTO struct {
	Date      *string              `json:"date"`
	Presets   []string             `json:"presets"`
	Exercises []routineExerciseDTO `json:"exercises"`
}

func toCombinedRoutineDTO(c *service.CombinedRoutine) combinedRoutineDTO {
	var date *string
	if c.Date != "" {
		d := c.Date
		date = &d
	}
	presets := c.Presets
	if presets == nil {
		presets = []string{}
	}
	return combinedRoutineDTO{Date: date, Presets: presets, Exercises: toRoutineExerciseDTOs(c.Exercises)}
}

// profileDTO is the API representation of the user profile.
type profileDTO struct {
	BodyweightKg float64 `json:"bodyweightKg"`
	HeightCm     float64 `json:"heightCm"`
	MaxHrEst     int     `json:"maxHrEst"`
}

func toProfileDTO(p domain.Profile) profileDTO {
	return profileDTO{BodyweightKg: p.BodyweightKg, HeightCm: p.HeightCm, MaxHrEst: p.MaxHrEst}
}

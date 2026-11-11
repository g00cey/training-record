package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"training-record/internal/apperr"
	"training-record/internal/domain"
	"training-record/internal/llmconsult"
	"training-record/internal/store"
)

// ConsultationContext is the snapshot of user context passed to the LLM.
type ConsultationContext struct {
	AsOf             string                         `json:"asOf"`
	PeriodFrom       string                         `json:"periodFrom"`
	PeriodTo         string                         `json:"periodTo"`
	Profile          ConsultationProfile            `json:"profile"`
	LoadMetrics      ConsultationLoadMetrics        `json:"loadMetrics"`
	Presets          []ConsultationPreset           `json:"presets"`
	StrengthSessions []ConsultationStrengthSession  `json:"strengthSessions"`
	SpinSessions     []ConsultationSpinSession      `json:"spinSessions"`
}

type ConsultationProfile struct {
	BodyweightKg float64 `json:"bodyweightKg"`
	HeightCm     float64 `json:"heightCm"`
	MaxHrEst     int     `json:"maxHrEst"`
}

type ConsultationLoadMetrics struct {
	ACWR             float64 `json:"acwr"`
	Zone             string  `json:"zone"`
	Acute7dTotal     float64 `json:"acute7dTotal"`
	ChronicWeeklyAvg float64 `json:"chronicWeeklyAvg"`
}

type ConsultationPreset struct {
	Name      string                       `json:"name"`
	Date      string                       `json:"date"`
	Exercises []ConsultationPresetExercise `json:"exercises"`
}

type ConsultationPresetExercise struct {
	Name   string   `json:"name"`
	Weight *float64 `json:"weight"`
	Reps   int      `json:"reps"`
	Sets   int      `json:"sets"`
}

type ConsultationStrengthSession struct {
	Date      string                         `json:"date"`
	Notes     string                         `json:"notes"`
	Exercises []ConsultationStrengthExercise `json:"exercises"`
}

type ConsultationStrengthExercise struct {
	Name   string   `json:"name"`
	Weight *float64 `json:"weight"`
	Reps   int      `json:"reps"`
	Sets   int      `json:"sets"`
	Notes  string   `json:"notes"`
}

type ConsultationSpinSession struct {
	Date            string   `json:"date"`
	DurationMinutes int      `json:"durationMinutes"`
	AvgHeartRate    *int     `json:"avgHeartRate"`
	MaxHeartRate    *int     `json:"maxHeartRate"`
	RPE             *int     `json:"rpe"`
	DistanceKm      *float64 `json:"distanceKm"`
	Notes           string   `json:"notes"`
}

// StartConsultation gathers the 2-week training context, calls the LLM, and persists the new session.
func (s *Service) StartConsultation(ctx context.Context, llm *llmconsult.Client, goal string) (*domain.Consultation, error) {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return nil, apperr.Unprocessablef("goal is required")
	}
	if len([]rune(goal)) > 2000 {
		return nil, apperr.Unprocessablef("goal must be 2000 characters or less")
	}
	if llm == nil {
		return nil, &llmconsult.Error{Kind: llmconsult.ErrHTTP, Msg: "consultation service not configured", Status: 503}
	}

	asOf := domain.Today()
	nowTime, err := time.ParseInLocation(domain.DateLayout, asOf, domain.JST)
	if err != nil {
		nowTime = time.Now().In(domain.JST)
	}

	periodFrom := nowTime.AddDate(0, 0, -13).Format(domain.DateLayout)
	fetchFrom := nowTime.AddDate(0, 0, -28).Format(domain.DateLayout)

	prof, err := s.st.GetProfile()
	if err != nil {
		return nil, err
	}

	strengths28, err := s.st.StrengthSessionsInRange(fetchFrom, "")
	if err != nil {
		return nil, err
	}
	spins28, err := s.st.SpinSessionsInRange(fetchFrom, "")
	if err != nil {
		return nil, err
	}

	m := computeLoadMetricsFrom(nowTime, prof, strengths28, spins28)

	// Filter strengths and spins to 2 weeks [periodFrom, asOf]
	var strengths2w []ConsultationStrengthSession
	for _, ss := range strengths28 {
		if ss.Date >= periodFrom && ss.Date <= asOf {
			exs := make([]ConsultationStrengthExercise, len(ss.Exercises))
			for i, e := range ss.Exercises {
				exs[i] = ConsultationStrengthExercise{
					Name:   e.Name,
					Weight: e.Weight,
					Reps:   e.Reps,
					Sets:   e.Sets,
					Notes:  e.Notes,
				}
			}
			strengths2w = append(strengths2w, ConsultationStrengthSession{
				Date:      ss.Date,
				Notes:     ss.Notes,
				Exercises: exs,
			})
		}
	}

	var spins2w []ConsultationSpinSession
	for _, sp := range spins28 {
		if sp.Date >= periodFrom && sp.Date <= asOf {
			spins2w = append(spins2w, ConsultationSpinSession{
				Date:            sp.Date,
				DurationMinutes: sp.DurationMinutes,
				AvgHeartRate:    sp.AvgHeartRate,
				MaxHeartRate:    sp.MaxHeartRate,
				RPE:             sp.RPE,
				DistanceKm:      sp.DistanceKm,
				Notes:           sp.Notes,
			})
		}
	}

	// Presets
	presetList, err := s.st.ListPresets()
	if err != nil {
		return nil, err
	}
	var presets []ConsultationPreset
	for _, p := range presetList {
		if p.LatestDate == nil || *p.LatestDate == "" {
			continue
		}
		exs, err := s.st.PresetSnapshot(p.Name, *p.LatestDate)
		if err != nil {
			continue
		}
		pexs := make([]ConsultationPresetExercise, len(exs))
		for i, e := range exs {
			pexs[i] = ConsultationPresetExercise{
				Name:   e.Name,
				Weight: e.Weight,
				Reps:   e.Reps,
				Sets:   e.Sets,
			}
		}
		presets = append(presets, ConsultationPreset{
			Name:      p.Name,
			Date:      *p.LatestDate,
			Exercises: pexs,
		})
	}

	cContext := ConsultationContext{
		AsOf:       asOf,
		PeriodFrom: periodFrom,
		PeriodTo:   asOf,
		Profile: ConsultationProfile{
			BodyweightKg: prof.BodyweightKg,
			HeightCm:     prof.HeightCm,
			MaxHrEst:     prof.MaxHrEst,
		},
		LoadMetrics: ConsultationLoadMetrics{
			ACWR:             m.acwr,
			Zone:             m.zone,
			Acute7dTotal:     domain.RoundInt(m.acuteTotal),
			ChronicWeeklyAvg: domain.RoundInt(m.chronicWeekly),
		},
		Presets:          presets,
		StrengthSessions: strengths2w,
		SpinSessions:     spins2w,
	}

	contextBytes, err := json.Marshal(cContext)
	if err != nil {
		return nil, err
	}

	llmPayload := llmconsult.RequestPayload{
		Goal:    goal,
		Context: cContext,
		Messages: []llmconsult.MessagePayload{
			{Role: "user", Content: goal},
		},
		RequestMenuProposal: false,
	}

	reply, err := llm.Consult(ctx, llmPayload)
	if err != nil {
		return nil, err
	}

	return s.st.CreateConsultation(goal, string(contextBytes), goal, reply)
}

// AddConsultationMessage sends an additional message or menu proposal request to the LLM and persists it.
func (s *Service) AddConsultationMessage(ctx context.Context, llm *llmconsult.Client, id int64, content string, requestMenuProposal bool) (*domain.Consultation, error) {
	content = strings.TrimSpace(content)
	if requestMenuProposal && content == "" {
		content = "ここまでの相談内容をもとに、具体的なトレーニングメニューを提案してください。"
	}
	if content == "" {
		return nil, apperr.Unprocessablef("content is required")
	}
	if len([]rune(content)) > 2000 {
		return nil, apperr.Unprocessablef("content must be 2000 characters or less")
	}
	if llm == nil {
		return nil, &llmconsult.Error{Kind: llmconsult.ErrHTTP, Msg: "consultation service not configured", Status: 503}
	}

	c, err := s.st.GetConsultation(id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, apperr.NotFoundf("consultation %d not found", id)
	}
	if err != nil {
		return nil, err
	}

	var ctxData any
	if err := json.Unmarshal([]byte(c.ContextJSON), &ctxData); err != nil {
		ctxData = map[string]any{}
	}

	msgs := make([]llmconsult.MessagePayload, 0, len(c.Messages)+1)
	for _, m := range c.Messages {
		msgs = append(msgs, llmconsult.MessagePayload{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	msgs = append(msgs, llmconsult.MessagePayload{
		Role:    "user",
		Content: content,
	})

	llmPayload := llmconsult.RequestPayload{
		Goal:                c.Goal,
		Context:             ctxData,
		Messages:            msgs,
		RequestMenuProposal: requestMenuProposal,
	}

	reply, err := llm.Consult(ctx, llmPayload)
	if err != nil {
		return nil, err
	}

	return s.st.AddConsultationMessage(id, content, reply)
}

// ListConsultations returns a paginated list of consultation summaries.
func (s *Service) ListConsultations(limit, offset int) ([]domain.ConsultationSummary, int, error) {
	return s.st.ListConsultations(limit, offset)
}

// GetConsultation returns a consultation and its messages by ID.
func (s *Service) GetConsultation(id int64) (*domain.Consultation, error) {
	c, err := s.st.GetConsultation(id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, apperr.NotFoundf("consultation %d not found", id)
	}
	return c, err
}

// DeleteConsultation removes a consultation session by ID.
func (s *Service) DeleteConsultation(id int64) error {
	err := s.st.DeleteConsultation(id)
	if errors.Is(err, store.ErrNotFound) {
		return apperr.NotFoundf("consultation %d not found", id)
	}
	return err
}

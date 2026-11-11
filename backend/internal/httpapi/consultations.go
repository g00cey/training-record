package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"training-record/internal/domain"
	"training-record/internal/llmconsult"
)

type consultationMessageDTO struct {
	ID        int64  `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}

type consultationDetailDTO struct {
	ID        int64                    `json:"id"`
	Goal      string                   `json:"goal"`
	CreatedAt string                   `json:"createdAt"`
	UpdatedAt string                   `json:"updatedAt"`
	Messages  []consultationMessageDTO `json:"messages"`
}

type consultationSummaryDTO struct {
	ID                   int64  `json:"id"`
	Goal                 string `json:"goal"`
	MessageCount         int    `json:"messageCount"`
	LatestMessagePreview string `json:"latestMessagePreview"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
}

type consultationListResponse struct {
	Items []consultationSummaryDTO `json:"items"`
	Total int                      `json:"total"`
}

func toConsultationDetailDTO(c *domain.Consultation) consultationDetailDTO {
	msgs := make([]consultationMessageDTO, len(c.Messages))
	for i, m := range c.Messages {
		msgs[i] = consultationMessageDTO{
			ID:        m.ID,
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt,
		}
	}
	return consultationDetailDTO{
		ID:        c.ID,
		Goal:      c.Goal,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
		Messages:  msgs,
	}
}

func toConsultationSummaryDTO(s domain.ConsultationSummary) consultationSummaryDTO {
	return consultationSummaryDTO{
		ID:                   s.ID,
		Goal:                 s.Goal,
		MessageCount:         s.MessageCount,
		LatestMessagePreview: s.LatestMessagePreview,
		CreatedAt:            s.CreatedAt,
		UpdatedAt:            s.UpdatedAt,
	}
}

func mapLLMConsultError(err error) error {
	var ce *llmconsult.Error
	if errors.As(err, &ce) {
		switch ce.Kind {
		case llmconsult.ErrTimeout:
			return &httpError{http.StatusGatewayTimeout, "llm_timeout", "トレーニング相談がタイムアウトしました（90 秒）"}
		case llmconsult.ErrHTTP:
			if ce.Status == 503 {
				return &httpError{http.StatusServiceUnavailable, "llm_not_configured", "トレーニング相談は未設定です"}
			}
			return &httpError{
				http.StatusBadGateway, "llm_error",
				fmt.Sprintf("LLM サーバーがエラーを返しました (%d)", ce.Status),
			}
		case llmconsult.ErrDecode:
			return &httpError{http.StatusBadGateway, "llm_error", "LLM サーバーの応答を解析できませんでした"}
		default:
			return &httpError{http.StatusBadGateway, "llm_unreachable", "LLM サーバーに接続できません"}
		}
	}
	return err
}

type startConsultationBody struct {
	Goal string `json:"goal"`
}

func (h *Handlers) createConsultation(w http.ResponseWriter, r *http.Request) error {
	if h.consult == nil {
		return &httpError{
			http.StatusServiceUnavailable, "llm_not_configured",
			"トレーニング相談は未設定です（サーバの LLM_CONSULT_API_URL / LLM_CONSULT_API_KEY）",
		}
	}

	var b startConsultationBody
	if err := decodeBody(r, &b); err != nil {
		return err
	}

	c, err := h.svc.StartConsultation(r.Context(), h.consult, b.Goal)
	if err != nil {
		return mapLLMConsultError(err)
	}

	writeJSON(w, http.StatusCreated, toConsultationDetailDTO(c))
	return nil
}

func (h *Handlers) listConsultations(w http.ResponseWriter, r *http.Request) error {
	limit, err := qInt(r, "limit", 50)
	if err != nil {
		return err
	}
	if limit > 200 {
		limit = 200
	}
	offset, err := qInt(r, "offset", 0)
	if err != nil {
		return err
	}

	items, total, err := h.svc.ListConsultations(limit, offset)
	if err != nil {
		return err
	}

	dtos := make([]consultationSummaryDTO, len(items))
	for i, it := range items {
		dtos[i] = toConsultationSummaryDTO(it)
	}

	writeJSON(w, http.StatusOK, consultationListResponse{
		Items: dtos,
		Total: total,
	})
	return nil
}

func (h *Handlers) getConsultation(w http.ResponseWriter, r *http.Request) error {
	id, err := pathInt64(r, "id")
	if err != nil {
		return err
	}

	c, err := h.svc.GetConsultation(id)
	if err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, toConsultationDetailDTO(c))
	return nil
}

type addMessageBody struct {
	Content             string `json:"content"`
	RequestMenuProposal bool   `json:"requestMenuProposal"`
}

func (h *Handlers) addConsultationMessage(w http.ResponseWriter, r *http.Request) error {
	if h.consult == nil {
		return &httpError{
			http.StatusServiceUnavailable, "llm_not_configured",
			"トレーニング相談は未設定です（サーバの LLM_CONSULT_API_URL / LLM_CONSULT_API_KEY）",
		}
	}

	id, err := pathInt64(r, "id")
	if err != nil {
		return err
	}

	var b addMessageBody
	if err := decodeBody(r, &b); err != nil {
		return err
	}

	c, err := h.svc.AddConsultationMessage(r.Context(), h.consult, id, b.Content, b.RequestMenuProposal)
	if err != nil {
		return mapLLMConsultError(err)
	}

	writeJSON(w, http.StatusOK, toConsultationDetailDTO(c))
	return nil
}

func (h *Handlers) deleteConsultation(w http.ResponseWriter, r *http.Request) error {
	id, err := pathInt64(r, "id")
	if err != nil {
		return err
	}

	if err := h.svc.DeleteConsultation(id); err != nil {
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}


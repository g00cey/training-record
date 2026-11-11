package store

import (
	"database/sql"
	"errors"

	"training-record/internal/domain"
)

// CreateConsultation creates a new consultation and its initial messages in a single transaction.
func (s *Store) CreateConsultation(goal, contextJSON, userMsg, assistantMsg string) (*domain.Consultation, error) {
	now := domain.NowRFC3339()
	var c domain.Consultation
	err := s.tx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`INSERT INTO consultations (goal, context_json, created_at, updated_at)
			 VALUES (?, ?, ?, ?)`,
			goal, contextJSON, now, now)
		if err != nil {
			return err
		}
		cid, err := res.LastInsertId()
		if err != nil {
			return err
		}
		c.ID = cid
		c.Goal = goal
		c.ContextJSON = contextJSON
		c.CreatedAt = now
		c.UpdatedAt = now

		// Insert user message
		uRes, err := tx.Exec(
			`INSERT INTO consultation_messages (consultation_id, role, content, created_at)
			 VALUES (?, 'user', ?, ?)`,
			cid, userMsg, now)
		if err != nil {
			return err
		}
		uID, err := uRes.LastInsertId()
		if err != nil {
			return err
		}

		// Insert assistant message
		aRes, err := tx.Exec(
			`INSERT INTO consultation_messages (consultation_id, role, content, created_at)
			 VALUES (?, 'assistant', ?, ?)`,
			cid, assistantMsg, now)
		if err != nil {
			return err
		}
		aID, err := aRes.LastInsertId()
		if err != nil {
			return err
		}

		c.Messages = []domain.ConsultationMessage{
			{
				ID:             uID,
				ConsultationID: cid,
				Role:           "user",
				Content:        userMsg,
				CreatedAt:      now,
			},
			{
				ID:             aID,
				ConsultationID: cid,
				Role:           "assistant",
				Content:        assistantMsg,
				CreatedAt:      now,
			},
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListConsultations returns a paginated list of consultation summaries and the total count.
func (s *Store) ListConsultations(limit, offset int) ([]domain.ConsultationSummary, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM consultations`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(
		`SELECT c.id, c.goal, c.created_at, c.updated_at,
		        (SELECT COUNT(*) FROM consultation_messages WHERE consultation_id = c.id),
		        COALESCE((SELECT content FROM consultation_messages WHERE consultation_id = c.id ORDER BY id DESC LIMIT 1), '')
		 FROM consultations c
		 ORDER BY c.updated_at DESC, c.id DESC
		 LIMIT ? OFFSET ?`,
		limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []domain.ConsultationSummary{}
	for rows.Next() {
		var item domain.ConsultationSummary
		var createdAt, updatedAt string
		if err := rows.Scan(&item.ID, &item.Goal, &createdAt, &updatedAt, &item.MessageCount, &item.LatestMessagePreview); err != nil {
			return nil, 0, err
		}
		item.CreatedAt = domain.DBTimeToRFC3339(createdAt)
		item.UpdatedAt = domain.DBTimeToRFC3339(updatedAt)
		// プレビューが長い場合は適度に省略
		if len([]rune(item.LatestMessagePreview)) > 100 {
			item.LatestMessagePreview = string([]rune(item.LatestMessagePreview)[:100]) + "..."
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetConsultation retrieves a consultation by ID along with all its messages.
func (s *Store) GetConsultation(id int64) (*domain.Consultation, error) {
	var c domain.Consultation
	var createdAt, updatedAt string
	err := s.db.QueryRow(
		`SELECT id, goal, context_json, created_at, updated_at
		 FROM consultations WHERE id = ?`, id).
		Scan(&c.ID, &c.Goal, &c.ContextJSON, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.CreatedAt = domain.DBTimeToRFC3339(createdAt)
	c.UpdatedAt = domain.DBTimeToRFC3339(updatedAt)

	rows, err := s.db.Query(
		`SELECT id, consultation_id, role, content, created_at
		 FROM consultation_messages
		 WHERE consultation_id = ?
		 ORDER BY id ASC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	c.Messages = []domain.ConsultationMessage{}
	for rows.Next() {
		var m domain.ConsultationMessage
		var msgCreated string
		if err := rows.Scan(&m.ID, &m.ConsultationID, &m.Role, &m.Content, &msgCreated); err != nil {
			return nil, err
		}
		m.CreatedAt = domain.DBTimeToRFC3339(msgCreated)
		c.Messages = append(c.Messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &c, nil
}

// AddConsultationMessage appends user and assistant messages to a consultation and updates updated_at.
func (s *Store) AddConsultationMessage(consultationID int64, userMsg, assistantMsg string) (*domain.Consultation, error) {
	now := domain.NowRFC3339()
	err := s.tx(func(tx *sql.Tx) error {
		var dummy int
		err := tx.QueryRow(`SELECT 1 FROM consultations WHERE id = ?`, consultationID).Scan(&dummy)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}

		if _, err := tx.Exec(
			`INSERT INTO consultation_messages (consultation_id, role, content, created_at)
			 VALUES (?, 'user', ?, ?)`,
			consultationID, userMsg, now); err != nil {
			return err
		}

		if _, err := tx.Exec(
			`INSERT INTO consultation_messages (consultation_id, role, content, created_at)
			 VALUES (?, 'assistant', ?, ?)`,
			consultationID, assistantMsg, now); err != nil {
			return err
		}

		if _, err := tx.Exec(
			`UPDATE consultations SET updated_at = ? WHERE id = ?`,
			now, consultationID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetConsultation(consultationID)
}

// DeleteConsultation deletes a consultation and its cascade messages.
func (s *Store) DeleteConsultation(id int64) error {
	res, err := s.db.Exec(`DELETE FROM consultations WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

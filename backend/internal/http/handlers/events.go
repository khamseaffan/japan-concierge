// Package handlers contains the HTTP request handlers for the api server.
//
// Handlers are thin: parse request, call service, render response. Validation
// of request shape happens here so the service layer can assume well-formed
// inputs.
package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/khamseaffan/japan-concierge/backend/internal/db/sqlc"
	httpmw "github.com/khamseaffan/japan-concierge/backend/internal/http/middleware"
	"github.com/khamseaffan/japan-concierge/backend/internal/rules"
	"github.com/khamseaffan/japan-concierge/backend/internal/service"
)

// EventsHandler holds dependencies for the life-event endpoints.
type EventsHandler struct {
	Tracker *service.TrackerService
	Logger  *slog.Logger
}

// CreateLifeEventRequest is the JSON body for POST /api/v1/life-events.
type CreateLifeEventRequest struct {
	EventType  string          `json:"event_type"`
	OccurredAt string          `json:"occurred_at"` // YYYY-MM-DD
	Payload    json.RawMessage `json:"payload,omitempty"`
}

// CreateLifeEventResponse is what the API returns after recording an event.
// The full freshly-generated task batch is included so the frontend can show
// the user what was created without an extra round-trip.
type CreateLifeEventResponse struct {
	Event TaskEventDTO `json:"event"`
	Tasks []TaskDTO    `json:"tasks"`
}

// TaskEventDTO mirrors a sqlc.LifeEvent in JSON-friendly form.
type TaskEventDTO struct {
	ID         int64           `json:"id"`
	UserID     int64           `json:"user_id"`
	VisaID     *int64          `json:"visa_id,omitempty"`
	EventType  string          `json:"event_type"`
	OccurredAt string          `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
	CreatedAt  string          `json:"created_at"`
}

// TaskDTO mirrors a sqlc.ComplianceTask in JSON-friendly form.
type TaskDTO struct {
	ID                 int64   `json:"id"`
	UserID             int64   `json:"user_id"`
	VisaID             *int64  `json:"visa_id,omitempty"`
	RuleID             string  `json:"rule_id"`
	TriggeredByEventID *int64  `json:"triggered_by_event_id,omitempty"`
	TitleEN            string  `json:"title_en"`
	TitleJA            string  `json:"title_ja,omitempty"`
	DescriptionEN      string  `json:"description_en"`
	DescriptionJA      string  `json:"description_ja,omitempty"`
	Category           string  `json:"category"`
	Severity           string  `json:"severity"`
	Status             string  `json:"status"`
	DeadlineAt         *string `json:"deadline_at,omitempty"`
	LegalSourceURL     string  `json:"legal_source_url,omitempty"`
	LegalSourceText    string  `json:"legal_source_text,omitempty"`
	LocationHint       string  `json:"location_hint,omitempty"`
}

// Create handles POST /api/v1/life-events.
func (h *EventsHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpmw.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no user context")
		return
	}

	var req CreateLifeEventRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("malformed JSON: %v", err))
		return
	}

	if err := validateCreateRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	occurredAt, err := time.Parse("2006-01-02", req.OccurredAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("occurred_at must be YYYY-MM-DD: %v", err))
		return
	}

	result, err := h.Tracker.RecordLifeEvent(r.Context(), service.RecordLifeEventInput{
		UserID:     userID,
		EventType:  rules.EventType(req.EventType),
		OccurredAt: occurredAt,
		Payload:    req.Payload,
	})
	if err != nil {
		h.Logger.Error("RecordLifeEvent failed", "user_id", userID, "event_type", req.EventType, "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := CreateLifeEventResponse{
		Event: lifeEventToDTO(result.Event),
		Tasks: make([]TaskDTO, 0, len(result.Tasks)),
	}
	for _, t := range result.Tasks {
		resp.Tasks = append(resp.Tasks, taskToDTO(t))
	}

	h.Logger.Info("life event recorded",
		"user_id", userID,
		"event_id", result.Event.ID,
		"event_type", req.EventType,
		"tasks_generated", len(result.Tasks),
	)

	writeJSON(w, http.StatusCreated, resp)
}

// validateCreateRequest is the validator for POST /life-events bodies. Kept
// inline (no go-playground/validator dependency yet) because the surface is
// small and explicit checks are clearer for the first endpoint.
func validateCreateRequest(req CreateLifeEventRequest) error {
	if req.EventType == "" {
		return errors.New("event_type is required")
	}
	if !isKnownEventType(req.EventType) {
		return fmt.Errorf("unknown event_type %q", req.EventType)
	}
	if req.OccurredAt == "" {
		return errors.New("occurred_at is required (YYYY-MM-DD)")
	}
	return nil
}

func isKnownEventType(s string) bool {
	switch rules.EventType(s) {
	case rules.EventVisaApplicationStarted,
		rules.EventVisaApplied,
		rules.EventVisaApproved,
		rules.EventCoEReceived,
		rules.EventLandedJapan,
		rules.EventAddressRegistered,
		rules.EventAddressChanged,
		rules.EventEmployerChanged,
		rules.EventVisaRenewalWindowOpens,
		rules.EventTaxResidencyTriggered:
		return true
	}
	return false
}

func lifeEventToDTO(e sqlc.LifeEvent) TaskEventDTO {
	dto := TaskEventDTO{
		ID:         e.ID,
		UserID:     e.UserID,
		EventType:  e.EventType,
		OccurredAt: e.OccurredAt.Time.Format("2006-01-02"),
		Payload:    e.Payload,
		CreatedAt:  e.CreatedAt.Time.UTC().Format(time.RFC3339),
	}
	if e.VisaID.Valid {
		v := e.VisaID.Int64
		dto.VisaID = &v
	}
	return dto
}

func taskToDTO(t sqlc.ComplianceTask) TaskDTO {
	dto := TaskDTO{
		ID:            t.ID,
		UserID:        t.UserID,
		RuleID:        t.RuleID,
		TitleEN:       t.TitleEn,
		DescriptionEN: t.DescriptionEn,
		Category:      t.Category,
		Severity:      t.Severity,
		Status:        t.Status,
	}
	if t.VisaID.Valid {
		v := t.VisaID.Int64
		dto.VisaID = &v
	}
	if t.TriggeredByEventID.Valid {
		v := t.TriggeredByEventID.Int64
		dto.TriggeredByEventID = &v
	}
	if t.TitleJa.Valid {
		dto.TitleJA = t.TitleJa.String
	}
	if t.DescriptionJa.Valid {
		dto.DescriptionJA = t.DescriptionJa.String
	}
	if t.DeadlineAt.Valid {
		s := t.DeadlineAt.Time.Format("2006-01-02")
		dto.DeadlineAt = &s
	}
	if t.LegalSourceUrl.Valid {
		dto.LegalSourceURL = t.LegalSourceUrl.String
	}
	if t.LegalSourceText.Valid {
		dto.LegalSourceText = t.LegalSourceText.String
	}
	if t.LocationHint.Valid {
		dto.LocationHint = t.LocationHint.String
	}
	return dto
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

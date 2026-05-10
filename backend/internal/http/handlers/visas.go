package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/khamseaffan/japan-concierge/backend/internal/db/sqlc"
	httpmw "github.com/khamseaffan/japan-concierge/backend/internal/http/middleware"
	"github.com/khamseaffan/japan-concierge/backend/internal/service"
)

// VisasHandler holds dependencies for the visa endpoints.
type VisasHandler struct {
	Visas *service.VisaService
}

// CreateVisaRequest is the JSON body for POST /api/v1/visas.
//
// status defaults to "planning" if omitted. The other fields are optional
// pre-arrival metadata that the user can fill in incrementally; we accept
// them now to avoid a follow-up PATCH.
type CreateVisaRequest struct {
	VisaTypeCode string `json:"visa_type_code"`
	Status       string `json:"status,omitempty"`
	Notes        string `json:"notes,omitempty"`
	SponsorName  string `json:"sponsor_name,omitempty"`
	JobTitle     string `json:"job_title,omitempty"`
}

// VisaDTO mirrors a sqlc.Visa for JSON output.
type VisaDTO struct {
	ID                    int64   `json:"id"`
	UserID                int64   `json:"user_id"`
	VisaTypeCode          string  `json:"visa_type_code"`
	Status                string  `json:"status"`
	CoENumber             string  `json:"coe_number,omitempty"`
	CoEIssuedAt           *string `json:"coe_issued_at,omitempty"`
	LandedAt              *string `json:"landed_at,omitempty"`
	ResidenceCardNumber   string  `json:"residence_card_number,omitempty"`
	ResidenceCardIssuedAt *string `json:"residence_card_issued_at,omitempty"`
	PeriodOfStayMonths    *int32  `json:"period_of_stay_months,omitempty"`
	ExpiresAt             *string `json:"expires_at,omitempty"`
	SponsorName           string  `json:"sponsor_name,omitempty"`
	SponsorAddress        string  `json:"sponsor_address,omitempty"`
	JobTitle              string  `json:"job_title,omitempty"`
	Notes                 string  `json:"notes,omitempty"`
	CreatedAt             string  `json:"created_at"`
	UpdatedAt             string  `json:"updated_at"`
}

// VisaTypeDTO mirrors sqlc.VisaType for JSON output.
type VisaTypeDTO struct {
	Code          string `json:"code"`
	DisplayNameEn string `json:"display_name_en"`
	DisplayNameJa string `json:"display_name_ja"`
}

// Create handles POST /api/v1/visas.
func (h *VisasHandler) Create(w http.ResponseWriter, r *http.Request) {
	logger := httpmw.LoggerFromContext(r.Context())

	userID, ok := httpmw.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no user context")
		return
	}

	var req CreateVisaRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("malformed JSON: %v", err))
		return
	}
	if req.VisaTypeCode == "" {
		writeError(w, http.StatusBadRequest, "visa_type_code is required")
		return
	}

	visa, err := h.Visas.CreateVisa(r.Context(), service.CreateVisaInput{
		UserID:       userID,
		VisaTypeCode: req.VisaTypeCode,
		Status:       req.Status,
		Notes:        req.Notes,
		SponsorName:  req.SponsorName,
		JobTitle:     req.JobTitle,
	})
	if err != nil {
		// Currently every failure path here is either bad input (unknown
		// visa_type_code) or a database error. Map known shapes to 400 and
		// everything else to 500. Substring matching is fragile; if a third
		// case appears, refactor to typed sentinel errors in the service.
		if msg := err.Error(); strings.Contains(msg, "unknown visa_type_code") ||
			strings.Contains(msg, "is required") {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		logger.Error("CreateVisa failed", "user_id", userID, "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	logger.Info("visa created",
		"user_id", userID,
		"visa_id", visa.ID,
		"visa_type_code", visa.VisaTypeCode,
		"status", visa.Status,
	)

	writeJSON(w, http.StatusCreated, visaToDTO(visa))
}

// GetActive handles GET /api/v1/visas/active.
func (h *VisasHandler) GetActive(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpmw.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no user context")
		return
	}
	visa, err := h.Visas.GetActiveVisa(r.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "no active visa for user")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, visaToDTO(visa))
}

// ListTypes handles GET /api/v1/visa-types — the catalog the UI uses to
// populate the visa-picker dropdown.
func (h *VisasHandler) ListTypes(w http.ResponseWriter, r *http.Request) {
	types, err := h.Visas.ListVisaTypes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]VisaTypeDTO, 0, len(types))
	for _, t := range types {
		out = append(out, VisaTypeDTO{
			Code:          t.Code,
			DisplayNameEn: t.DisplayNameEn,
			DisplayNameJa: t.DisplayNameJa,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"visa_types": out})
}

func visaToDTO(v sqlc.Visa) VisaDTO {
	dto := VisaDTO{
		ID:           v.ID,
		UserID:       v.UserID,
		VisaTypeCode: v.VisaTypeCode,
		Status:       v.Status,
		CreatedAt:    v.CreatedAt.Time.UTC().Format(time.RFC3339),
		UpdatedAt:    v.UpdatedAt.Time.UTC().Format(time.RFC3339),
	}
	if v.CoENumber.Valid {
		dto.CoENumber = v.CoENumber.String
	}
	if v.CoEIssuedAt.Valid {
		s := v.CoEIssuedAt.Time.Format("2006-01-02")
		dto.CoEIssuedAt = &s
	}
	if v.LandedAt.Valid {
		s := v.LandedAt.Time.Format("2006-01-02")
		dto.LandedAt = &s
	}
	if v.ResidenceCardNumber.Valid {
		dto.ResidenceCardNumber = v.ResidenceCardNumber.String
	}
	if v.ResidenceCardIssuedAt.Valid {
		s := v.ResidenceCardIssuedAt.Time.Format("2006-01-02")
		dto.ResidenceCardIssuedAt = &s
	}
	if v.PeriodOfStayMonths.Valid {
		n := v.PeriodOfStayMonths.Int32
		dto.PeriodOfStayMonths = &n
	}
	if v.ExpiresAt.Valid {
		s := v.ExpiresAt.Time.Format("2006-01-02")
		dto.ExpiresAt = &s
	}
	if v.SponsorName.Valid {
		dto.SponsorName = v.SponsorName.String
	}
	if v.SponsorAddress.Valid {
		dto.SponsorAddress = v.SponsorAddress.String
	}
	if v.JobTitle.Valid {
		dto.JobTitle = v.JobTitle.String
	}
	if v.Notes.Valid {
		dto.Notes = v.Notes.String
	}
	return dto
}


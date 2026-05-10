package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/khamseaffan/japan-concierge/backend/internal/db/sqlc"
)

// ErrNotFound is returned when a record cannot be located. Service callers
// should map this to HTTP 404 at the handler boundary.
var ErrNotFound = errors.New("not found")

// VisaQuerier is the narrow set of database operations VisaService needs.
type VisaQuerier interface {
	GetActiveVisaForUser(ctx context.Context, userID int64) (sqlc.Visa, error)
	GetVisaType(ctx context.Context, code string) (sqlc.VisaType, error)
	ListVisaTypes(ctx context.Context) ([]sqlc.VisaType, error)
	CreateVisa(ctx context.Context, arg sqlc.CreateVisaParams) (sqlc.Visa, error)
}

// VisaService handles visa lifecycle operations.
type VisaService struct {
	q VisaQuerier
}

// NewVisaService constructs a VisaService.
func NewVisaService(q VisaQuerier) *VisaService {
	return &VisaService{q: q}
}

// CreateVisaInput is the service-layer input shape for visa creation.
type CreateVisaInput struct {
	UserID       int64
	VisaTypeCode string
	Status       string // defaults to "planning" if empty
	Notes        string
	SponsorName  string
	JobTitle     string
}

// CreateVisa inserts a new visa for the user, validating the visa_type_code
// against the visa_types reference table.
func (s *VisaService) CreateVisa(ctx context.Context, in CreateVisaInput) (sqlc.Visa, error) {
	if in.UserID == 0 {
		return sqlc.Visa{}, fmt.Errorf("user_id is required")
	}
	if in.VisaTypeCode == "" {
		return sqlc.Visa{}, fmt.Errorf("visa_type_code is required")
	}

	if _, err := s.q.GetVisaType(ctx, in.VisaTypeCode); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Visa{}, fmt.Errorf("unknown visa_type_code %q", in.VisaTypeCode)
		}
		return sqlc.Visa{}, fmt.Errorf("validate visa_type_code: %w", err)
	}

	status := in.Status
	if status == "" {
		status = "planning"
	}

	params := sqlc.CreateVisaParams{
		UserID:       in.UserID,
		VisaTypeCode: in.VisaTypeCode,
		Status:       status,
	}
	if in.Notes != "" {
		params.Notes = pgtype.Text{String: in.Notes, Valid: true}
	}
	if in.SponsorName != "" {
		params.SponsorName = pgtype.Text{String: in.SponsorName, Valid: true}
	}
	if in.JobTitle != "" {
		params.JobTitle = pgtype.Text{String: in.JobTitle, Valid: true}
	}

	visa, err := s.q.CreateVisa(ctx, params)
	if err != nil {
		return sqlc.Visa{}, fmt.Errorf("insert visa: %w", err)
	}
	return visa, nil
}

// GetActiveVisa returns the user's currently-active visa, or ErrNotFound if
// they have no visa rows at all.
func (s *VisaService) GetActiveVisa(ctx context.Context, userID int64) (sqlc.Visa, error) {
	visa, err := s.q.GetActiveVisaForUser(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Visa{}, ErrNotFound
		}
		return sqlc.Visa{}, fmt.Errorf("load active visa: %w", err)
	}
	return visa, nil
}

// ListVisaTypes returns the catalog of supported visa types.
func (s *VisaService) ListVisaTypes(ctx context.Context) ([]sqlc.VisaType, error) {
	return s.q.ListVisaTypes(ctx)
}

var _ VisaQuerier = (*sqlc.Queries)(nil)

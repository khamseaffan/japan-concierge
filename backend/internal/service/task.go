package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/khamseaffan/japan-concierge/backend/internal/db/sqlc"
)

// TaskQuerier is the narrow set of database operations TaskService needs.
type TaskQuerier interface {
	ListTasksFiltered(ctx context.Context, arg sqlc.ListTasksFilteredParams) ([]sqlc.ComplianceTask, error)
}

// TaskService handles task read operations.
type TaskService struct {
	q TaskQuerier
}

// NewTaskService constructs a TaskService.
func NewTaskService(q TaskQuerier) *TaskService {
	return &TaskService{q: q}
}

// ListTasksFilter holds the optional filters for ListTasks. Empty / zero
// values mean "do not filter on this field."
type ListTasksFilter struct {
	UserID   int64
	Status   string
	VisaID   int64
	Category string
}

// ListTasks returns the user's tasks matching the given filters. Optional
// filters with zero values are skipped (the SQL uses sqlc.narg-based
// IS NULL OR equality predicates).
func (s *TaskService) ListTasks(ctx context.Context, f ListTasksFilter) ([]sqlc.ComplianceTask, error) {
	if f.UserID == 0 {
		return nil, fmt.Errorf("user_id is required")
	}
	params := sqlc.ListTasksFilteredParams{UserID: f.UserID}
	if f.Status != "" {
		params.Status = pgtype.Text{String: f.Status, Valid: true}
	}
	if f.VisaID != 0 {
		params.VisaID = pgtype.Int8{Int64: f.VisaID, Valid: true}
	}
	if f.Category != "" {
		params.Category = pgtype.Text{String: f.Category, Valid: true}
	}
	return s.q.ListTasksFiltered(ctx, params)
}

var _ TaskQuerier = (*sqlc.Queries)(nil)

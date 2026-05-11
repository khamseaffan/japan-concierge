package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/khamseaffan/japan-concierge/backend/internal/db/sqlc"
)

// ErrAlreadyDone is returned by MarkDone when the task is already in the
// "done" state. Maps to HTTP 409.
var ErrAlreadyDone = errors.New("task already done")

// TaskQuerier is the narrow set of database operations TaskService needs.
type TaskQuerier interface {
	ListTasksFiltered(ctx context.Context, arg sqlc.ListTasksFilteredParams) ([]sqlc.ComplianceTask, error)
	GetComplianceTask(ctx context.Context, arg sqlc.GetComplianceTaskParams) (sqlc.ComplianceTask, error)
	MarkTaskDone(ctx context.Context, arg sqlc.MarkTaskDoneParams) (sqlc.ComplianceTask, error)
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

// MarkDone flips a task to the "done" status with completed_at = NOW(). Returns
// ErrNotFound if the task does not exist or belongs to another user, and
// ErrAlreadyDone if it is already in the "done" state.
//
// The two errors are distinguished because the UI behavior differs: 404 means
// "stale link, refresh"; 409 means "you (or another tab) already did this,
// no-op". The schema's WHERE status != 'done' clause makes "already done" an
// idempotent no-op at the DB layer; we map it to a sentinel here so the
// caller doesn't have to round-trip a GET to disambiguate.
func (s *TaskService) MarkDone(ctx context.Context, userID, taskID int64) (sqlc.ComplianceTask, error) {
	if userID == 0 {
		return sqlc.ComplianceTask{}, fmt.Errorf("user_id is required")
	}
	if taskID == 0 {
		return sqlc.ComplianceTask{}, fmt.Errorf("task_id is required")
	}

	updated, err := s.q.MarkTaskDone(ctx, sqlc.MarkTaskDoneParams{
		ID:     taskID,
		UserID: userID,
	})
	if err == nil {
		return updated, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.ComplianceTask{}, fmt.Errorf("mark task done: %w", err)
	}

	// MarkTaskDone returned no rows. Two possible reasons:
	//   1. The task does not exist or is not owned by this user (404).
	//   2. The task exists but is already in the "done" state (409),
	//      because the SQL has WHERE status != 'done'.
	// Disambiguate with a GET.
	existing, lookupErr := s.q.GetComplianceTask(ctx, sqlc.GetComplianceTaskParams{
		ID:     taskID,
		UserID: userID,
	})
	if lookupErr != nil {
		if errors.Is(lookupErr, pgx.ErrNoRows) {
			return sqlc.ComplianceTask{}, ErrNotFound
		}
		return sqlc.ComplianceTask{}, fmt.Errorf("disambiguate mark-done: %w", lookupErr)
	}
	if existing.Status == "done" {
		return existing, ErrAlreadyDone
	}
	// Should be unreachable: MarkTaskDone returned no rows but the task is
	// neither missing nor in 'done'. Surface as a generic error.
	return sqlc.ComplianceTask{}, fmt.Errorf("mark-done returned no rows for task in status %q", existing.Status)
}

var _ TaskQuerier = (*sqlc.Queries)(nil)

// Package service contains the orchestration layer that bridges the pure
// rules engine and the persistence layer.
//
// The narrow Querier interface defined here is what TrackerService depends on
// (interface segregation): tests can mock just the methods this service uses
// without faking the full sqlc-generated Querier surface.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/khamseaffan/japan-concierge/backend/internal/db/sqlc"
	"github.com/khamseaffan/japan-concierge/backend/internal/rules"
)

// TrackerQuerier is the narrow set of database operations TrackerService needs.
// Defined here (not in the sqlc package) so tests can mock just these methods.
type TrackerQuerier interface {
	GetActiveVisaForUser(ctx context.Context, userID int64) (sqlc.Visa, error)
	CreateLifeEvent(ctx context.Context, arg sqlc.CreateLifeEventParams) (sqlc.LifeEvent, error)
	CreateComplianceTask(ctx context.Context, arg sqlc.CreateComplianceTaskParams) (sqlc.ComplianceTask, error)
	ListTasksTriggeredByEvent(ctx context.Context, arg sqlc.ListTasksTriggeredByEventParams) ([]sqlc.ComplianceTask, error)
}

// TxBeginner is anything that can start a Postgres transaction. *pgxpool.Pool
// implements this. We accept the interface (not the concrete type) so tests
// can pass a fake.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// TrackerService is the orchestration layer for compliance tracking.
type TrackerService struct {
	pool   TxBeginner
	engine *rules.Engine
}

// NewTrackerService constructs a TrackerService.
func NewTrackerService(pool TxBeginner, engine *rules.Engine) *TrackerService {
	return &TrackerService{pool: pool, engine: engine}
}

// RecordLifeEventInput is the service-layer input shape. Validation happens at
// the HTTP boundary; the service assumes its inputs are well-formed.
type RecordLifeEventInput struct {
	UserID     int64
	EventType  rules.EventType
	OccurredAt time.Time
	// Payload is event-specific JSON metadata. May be nil.
	Payload json.RawMessage
}

// RecordLifeEventResult is what the service returns to its caller.
type RecordLifeEventResult struct {
	Event sqlc.LifeEvent
	// Tasks is every task associated with the event after this call.
	// On a fresh insert this is the newly-generated batch.
	// On a duplicate event-id this would be the existing batch (not currently
	// possible because event_id is generated per call, but the contract is
	// stable for future use).
	Tasks []sqlc.ComplianceTask
}

// RecordLifeEvent persists a life event and the tasks it triggers in a single
// Postgres transaction. If any step fails, the whole thing rolls back.
//
// Idempotency is enforced by the schema: re-running this with the same
// (user_id, rule_id, triggered_by_event_id) is a no-op for the task insert.
// Since triggered_by_event_id is fresh per call, in practice each invocation
// produces a fresh batch — but the unique constraint protects against the
// future case of job-queue retries that re-process the same event_id.
func (s *TrackerService) RecordLifeEvent(ctx context.Context, in RecordLifeEventInput) (*RecordLifeEventResult, error) {
	if in.UserID == 0 {
		return nil, fmt.Errorf("user_id is required")
	}
	if in.EventType == "" {
		return nil, fmt.Errorf("event_type is required")
	}
	if in.OccurredAt.IsZero() {
		return nil, fmt.Errorf("occurred_at is required")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		// Rollback is a no-op if Commit already succeeded.
		_ = tx.Rollback(ctx)
	}()

	q := sqlc.New(tx)

	visa, err := q.GetActiveVisaForUser(ctx, in.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user %d has no active visa to associate this event with", in.UserID)
		}
		return nil, fmt.Errorf("load active visa: %w", err)
	}

	payload := in.Payload
	if payload == nil {
		payload = json.RawMessage(`{}`)
	}

	event, err := q.CreateLifeEvent(ctx, sqlc.CreateLifeEventParams{
		UserID:     in.UserID,
		VisaID:     pgtype.Int8{Int64: visa.ID, Valid: true},
		EventType:  string(in.EventType),
		OccurredAt: pgtype.Date{Time: in.OccurredAt, Valid: true},
		Payload:    payload,
	})
	if err != nil {
		return nil, fmt.Errorf("insert life event: %w", err)
	}

	generated, err := s.engine.Evaluate(rules.EvaluationInput{
		UserID:     in.UserID,
		VisaCode:   visa.VisaTypeCode,
		EventType:  in.EventType,
		OccurredAt: in.OccurredAt,
	})
	if err != nil {
		return nil, fmt.Errorf("evaluate rules: %w", err)
	}

	for _, gt := range generated {
		if _, err := q.CreateComplianceTask(ctx, taskParamsFrom(gt, in.UserID, visa.ID, event.ID)); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Idempotent: task already existed for this (user, rule, event).
				// Nothing to do; the existing row is fine.
				continue
			}
			return nil, fmt.Errorf("insert task %s: %w", gt.RuleID, err)
		}
	}

	tasks, err := q.ListTasksTriggeredByEvent(ctx, sqlc.ListTasksTriggeredByEventParams{
		UserID:             in.UserID,
		TriggeredByEventID: pgtype.Int8{Int64: event.ID, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("list tasks for event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &RecordLifeEventResult{Event: event, Tasks: tasks}, nil
}

// taskParamsFrom converts an engine-generated task into a sqlc insert params struct.
func taskParamsFrom(gt rules.GeneratedTask, userID, visaID, eventID int64) sqlc.CreateComplianceTaskParams {
	params := sqlc.CreateComplianceTaskParams{
		UserID:             userID,
		VisaID:             pgtype.Int8{Int64: visaID, Valid: true},
		RuleID:             gt.RuleID,
		TriggeredByEventID: pgtype.Int8{Int64: eventID, Valid: true},
		TitleEn:            gt.TitleEN,
		DescriptionEn:      gt.DescriptionEN,
		Category:           string(gt.Category),
		Severity:           string(gt.Severity),
		Status:             "pending",
		Metadata:           json.RawMessage(`{}`),
	}
	if gt.TitleJA != "" {
		params.TitleJa = pgtype.Text{String: gt.TitleJA, Valid: true}
	}
	if gt.DescriptionJA != "" {
		params.DescriptionJa = pgtype.Text{String: gt.DescriptionJA, Valid: true}
	}
	if gt.DeadlineAt != nil {
		params.DeadlineAt = pgtype.Date{Time: *gt.DeadlineAt, Valid: true}
	}
	if gt.LegalSourceURL != "" {
		params.LegalSourceUrl = pgtype.Text{String: gt.LegalSourceURL, Valid: true}
	}
	if gt.LegalSourceText != "" {
		params.LegalSourceText = pgtype.Text{String: gt.LegalSourceText, Valid: true}
	}
	if gt.LocationHint != "" {
		params.LocationHint = pgtype.Text{String: gt.LocationHint, Valid: true}
	}
	return params
}

// Compile-time check that sqlc.Queries satisfies our narrow interface so that
// real callers can pass it without an adapter.
var _ TrackerQuerier = (*sqlc.Queries)(nil)

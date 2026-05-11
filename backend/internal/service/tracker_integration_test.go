//go:build integration

package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/khamseaffan/japan-concierge/backend/internal/db/sqlc"
	"github.com/khamseaffan/japan-concierge/backend/internal/rules"
	"github.com/khamseaffan/japan-concierge/backend/internal/service"
	"github.com/khamseaffan/japan-concierge/backend/internal/testutil"
)

// withRulesEngine loads the embedded rule sets for tests.
func withRulesEngine(t *testing.T) *rules.Engine {
	t.Helper()
	sets, err := rules.LoadAll()
	if err != nil {
		t.Fatalf("load rules: %v", err)
	}
	return rules.NewEngine(sets)
}

// seedJFINDVisa creates a planning J-FIND visa for user 1 (the seeded user).
// Returns the new visa.
func seedJFINDVisa(t *testing.T, ctx context.Context, q *sqlc.Queries) sqlc.Visa {
	t.Helper()
	visa, err := q.CreateVisa(ctx, sqlc.CreateVisaParams{
		UserID:       1,
		VisaTypeCode: "jfind",
		Status:       "planning",
	})
	if err != nil {
		t.Fatalf("seed jfind visa: %v", err)
	}
	return visa
}

func TestRecordLifeEvent_HappyPath(t *testing.T) {
	t.Parallel()
	fix := testutil.StartPostgres(t)
	ctx := context.Background()

	q := sqlc.New(fix.Pool)
	visa := seedJFINDVisa(t, ctx, q)

	tracker := service.NewTrackerService(fix.Pool, withRulesEngine(t))

	occurredAt, _ := time.Parse("2006-01-02", "2026-08-01")
	res, err := tracker.RecordLifeEvent(ctx, service.RecordLifeEventInput{
		UserID:     1,
		EventType:  rules.EventLandedJapan,
		OccurredAt: occurredAt,
	})
	if err != nil {
		t.Fatalf("RecordLifeEvent: %v", err)
	}

	if res.Event.ID == 0 {
		t.Error("expected non-zero event ID")
	}
	if res.Event.UserID != 1 {
		t.Errorf("event.UserID = %d, want 1", res.Event.UserID)
	}
	if !res.Event.VisaID.Valid || res.Event.VisaID.Int64 != visa.ID {
		t.Errorf("event.VisaID = %v, want %d", res.Event.VisaID, visa.ID)
	}

	// J-FIND post-landing rule fires 4 tasks.
	if len(res.Tasks) != 4 {
		t.Errorf("tasks generated = %d, want 4 (J-FIND post-landing)", len(res.Tasks))
	}

	// Verify deadlines were computed in the service round-trip.
	wantAddrDeadline, _ := time.Parse("2006-01-02", "2026-08-15")
	var foundAddr bool
	for _, task := range res.Tasks {
		if task.RuleID == "jfind_post_landing_address_registration" {
			foundAddr = true
			if !task.DeadlineAt.Valid || !task.DeadlineAt.Time.Equal(wantAddrDeadline) {
				t.Errorf("address task deadline = %v, want %v", task.DeadlineAt, wantAddrDeadline)
			}
			break
		}
	}
	if !foundAddr {
		t.Error("address registration task not in result set")
	}
}

func TestCreateComplianceTask_IdempotentAtSchema(t *testing.T) {
	t.Parallel()
	fix := testutil.StartPostgres(t)
	ctx := context.Background()

	q := sqlc.New(fix.Pool)
	visa := seedJFINDVisa(t, ctx, q)

	// Insert a real life event we can attach the task to.
	occurredAt := pgtype.Date{Time: time.Now(), Valid: true}
	event, err := q.CreateLifeEvent(ctx, sqlc.CreateLifeEventParams{
		UserID:     1,
		VisaID:     pgtype.Int8{Int64: visa.ID, Valid: true},
		EventType:  string(rules.EventLandedJapan),
		OccurredAt: occurredAt,
		Payload:    []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("create life event: %v", err)
	}

	params := sqlc.CreateComplianceTaskParams{
		UserID:             1,
		VisaID:             pgtype.Int8{Int64: visa.ID, Valid: true},
		RuleID:             "test_rule_for_idempotency",
		TriggeredByEventID: pgtype.Int8{Int64: event.ID, Valid: true},
		TitleEn:            "Test task",
		DescriptionEn:      "Test description",
		Category:           "general",
		Severity:           "informational",
		Status:             "pending",
		Metadata:           []byte(`{}`),
	}

	// First insert: succeeds.
	first, err := q.CreateComplianceTask(ctx, params)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if first.ID == 0 {
		t.Fatal("first insert returned zero ID")
	}

	// Second insert with same (user, rule, event_id): conflicts and returns
	// ErrNoRows per the schema's UNIQUE constraint and ON CONFLICT DO NOTHING.
	_, err = q.CreateComplianceTask(ctx, params)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("second insert error = %v, want pgx.ErrNoRows", err)
	}

	// Verify exactly one row exists.
	tasks, err := q.ListTasksTriggeredByEvent(ctx, sqlc.ListTasksTriggeredByEventParams{
		UserID:             1,
		TriggeredByEventID: pgtype.Int8{Int64: event.ID, Valid: true},
	})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	count := 0
	for _, task := range tasks {
		if task.RuleID == "test_rule_for_idempotency" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("found %d tasks for rule, want exactly 1 (schema-level idempotency)", count)
	}
}

func TestRecordLifeEvent_UnknownVisaCodeRollsBack(t *testing.T) {
	t.Parallel()
	fix := testutil.StartPostgres(t)
	ctx := context.Background()

	// Insert a synthetic visa_type whose code is NOT in the rule engine's
	// known list. This forces engine.Evaluate to return an error inside the
	// service's transaction; we then verify the life_event was rolled back.
	if _, err := fix.Pool.Exec(ctx, `
		INSERT INTO visa_types (code, display_name_en, display_name_ja, rule_file)
		VALUES ('unknown_test_visa', 'Unknown Test Visa', 'テスト', 'unknown.yaml')
	`); err != nil {
		t.Fatalf("seed unknown visa_type: %v", err)
	}

	q := sqlc.New(fix.Pool)
	if _, err := q.CreateVisa(ctx, sqlc.CreateVisaParams{
		UserID:       1,
		VisaTypeCode: "unknown_test_visa",
		Status:       "planning",
	}); err != nil {
		t.Fatalf("seed visa: %v", err)
	}

	tracker := service.NewTrackerService(fix.Pool, withRulesEngine(t))

	_, err := tracker.RecordLifeEvent(ctx, service.RecordLifeEventInput{
		UserID:     1,
		EventType:  rules.EventLandedJapan,
		OccurredAt: time.Now(),
	})
	if err == nil {
		t.Fatal("expected RecordLifeEvent to fail on unknown visa code, got nil")
	}

	// The transaction should have rolled back. No life_events row should exist.
	var count int
	if err := fix.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM life_events WHERE user_id = 1`).Scan(&count); err != nil {
		t.Fatalf("count life_events: %v", err)
	}
	if count != 0 {
		t.Errorf("life_events count = %d, want 0 (transaction should have rolled back)", count)
	}
}

// TestVisaService_UnknownVisaTypeCode_CommitmentDevice exists specifically to
// freeze the substring the visas.go handler matches on for its 400 response
// path. The handler currently does:
//
//	if strings.Contains(msg, "unknown visa_type_code") { return 400 }
//
// That substring matching is fragile. The day someone refactors VisaService
// to return a typed sentinel error and changes the message, the handler
// silently falls through to 500. This test makes that refactor break loudly:
// when it fires, you MUST update the handler to use the typed error and
// remove the substring branch. See visas.go for the corresponding TODO.
func TestVisaService_UnknownVisaTypeCode_CommitmentDevice(t *testing.T) {
	t.Parallel()
	fix := testutil.StartPostgres(t)
	ctx := context.Background()

	q := sqlc.New(fix.Pool)
	svc := service.NewVisaService(q)

	_, err := svc.CreateVisa(ctx, service.CreateVisaInput{
		UserID:       1,
		VisaTypeCode: "made_up_visa_does_not_exist",
	})
	if err == nil {
		t.Fatal("expected error for unknown visa_type_code, got nil")
	}
	const requiredSubstring = "unknown visa_type_code"
	if !strings.Contains(err.Error(), requiredSubstring) {
		t.Fatalf(`error message must contain %q for handler substring matching to work.
got: %q
ACTION: if you are intentionally changing this message, also refactor
handlers/visas.go to use a typed sentinel error (errors.Is) instead of
strings.Contains, and delete this commitment-device test.`,
			requiredSubstring, err.Error())
	}
}

func TestTaskService_MarkDone_HappyPath(t *testing.T) {
	t.Parallel()
	fix := testutil.StartPostgres(t)
	ctx := context.Background()

	q := sqlc.New(fix.Pool)
	seedJFINDVisa(t, ctx, q)

	tracker := service.NewTrackerService(fix.Pool, withRulesEngine(t))
	tasks := service.NewTaskService(q)

	// Generate a real task by recording a life event.
	occurredAt, _ := time.Parse("2006-01-02", "2026-08-01")
	res, err := tracker.RecordLifeEvent(ctx, service.RecordLifeEventInput{
		UserID:     1,
		EventType:  rules.EventLandedJapan,
		OccurredAt: occurredAt,
	})
	if err != nil {
		t.Fatalf("seed event: %v", err)
	}
	if len(res.Tasks) == 0 {
		t.Fatal("seed event produced no tasks")
	}
	target := res.Tasks[0]
	if target.Status != "pending" {
		t.Fatalf("seeded task status = %q, want pending", target.Status)
	}

	updated, err := tasks.MarkDone(ctx, 1, target.ID)
	if err != nil {
		t.Fatalf("MarkDone: %v", err)
	}
	if updated.Status != "done" {
		t.Errorf("status after MarkDone = %q, want done", updated.Status)
	}
	if !updated.CompletedAt.Valid {
		t.Error("CompletedAt should be set after MarkDone")
	}
}

func TestTaskService_MarkDone_AlreadyDone(t *testing.T) {
	t.Parallel()
	fix := testutil.StartPostgres(t)
	ctx := context.Background()

	q := sqlc.New(fix.Pool)
	seedJFINDVisa(t, ctx, q)

	tracker := service.NewTrackerService(fix.Pool, withRulesEngine(t))
	tasks := service.NewTaskService(q)

	res, err := tracker.RecordLifeEvent(ctx, service.RecordLifeEventInput{
		UserID:     1,
		EventType:  rules.EventLandedJapan,
		OccurredAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("seed event: %v", err)
	}
	target := res.Tasks[0]

	// First call succeeds.
	if _, err := tasks.MarkDone(ctx, 1, target.ID); err != nil {
		t.Fatalf("first MarkDone: %v", err)
	}
	// Second call returns ErrAlreadyDone (mapped to 409 by the handler).
	_, err = tasks.MarkDone(ctx, 1, target.ID)
	if !errors.Is(err, service.ErrAlreadyDone) {
		t.Errorf("second MarkDone error = %v, want ErrAlreadyDone", err)
	}
}

func TestTaskService_MarkDone_NotFound(t *testing.T) {
	t.Parallel()
	fix := testutil.StartPostgres(t)
	ctx := context.Background()

	q := sqlc.New(fix.Pool)
	tasks := service.NewTaskService(q)

	_, err := tasks.MarkDone(ctx, 1, 999_999)
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("MarkDone(missing) error = %v, want ErrNotFound", err)
	}
}

func TestRecordLifeEvent_ZeroTaskEvent(t *testing.T) {
	t.Parallel()
	fix := testutil.StartPostgres(t)
	ctx := context.Background()

	q := sqlc.New(fix.Pool)
	seedJFINDVisa(t, ctx, q)

	tracker := service.NewTrackerService(fix.Pool, withRulesEngine(t))

	// EventVisaApproved is a known event type but no jfind rule responds to it.
	res, err := tracker.RecordLifeEvent(ctx, service.RecordLifeEventInput{
		UserID:     1,
		EventType:  rules.EventVisaApproved,
		OccurredAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("RecordLifeEvent: %v", err)
	}

	// Event was persisted...
	if res.Event.ID == 0 {
		t.Error("expected non-zero event ID")
	}
	// ...but no tasks were generated.
	if len(res.Tasks) != 0 {
		t.Errorf("tasks = %d, want 0 for unhandled event", len(res.Tasks))
	}

	// Sanity: the life_event row is in the database.
	var count int
	if err := fix.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM life_events WHERE id = $1`, res.Event.ID).Scan(&count); err != nil {
		t.Fatalf("count life_events: %v", err)
	}
	if count != 1 {
		t.Errorf("life_events count for new event = %d, want 1", count)
	}
}

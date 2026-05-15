package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/khamseaffan/japan-concierge/backend/internal/db/sqlc"
)

// --- test doubles ---

type mockConversationAI struct {
	fn func(context.Context, ConversationAIRequest, func(context.Context, ConversationToolCall) ConversationToolResult) (*ConversationAIResult, error)
}

func (m *mockConversationAI) Respond(ctx context.Context, req ConversationAIRequest, runTool func(context.Context, ConversationToolCall) ConversationToolResult) (*ConversationAIResult, error) {
	return m.fn(ctx, req, runTool)
}

type stubTaskQuerier struct {
	tasks []sqlc.ComplianceTask
}

func (q *stubTaskQuerier) ListTasksFiltered(_ context.Context, _ sqlc.ListTasksFilteredParams) ([]sqlc.ComplianceTask, error) {
	return q.tasks, nil
}

func (q *stubTaskQuerier) GetComplianceTask(_ context.Context, arg sqlc.GetComplianceTaskParams) (sqlc.ComplianceTask, error) {
	for _, t := range q.tasks {
		if t.ID == arg.ID && t.UserID == arg.UserID {
			return t, nil
		}
	}
	return sqlc.ComplianceTask{}, pgx.ErrNoRows
}

func (q *stubTaskQuerier) MarkTaskDone(_ context.Context, arg sqlc.MarkTaskDoneParams) (sqlc.ComplianceTask, error) {
	for i, t := range q.tasks {
		if t.ID == arg.ID && t.UserID == arg.UserID {
			if t.Status == "done" {
				return sqlc.ComplianceTask{}, pgx.ErrNoRows
			}
			q.tasks[i].Status = "done"
			q.tasks[i].CompletedAt = pgtype.Timestamptz{Valid: true}
			return q.tasks[i], nil
		}
	}
	return sqlc.ComplianceTask{}, pgx.ErrNoRows
}

func newTestConversationService(ai ConversationAI, taskQ TaskQuerier) *ConversationService {
	return &ConversationService{
		ai:      ai,
		model:   "test-model",
		tasks:   NewTaskService(taskQ),
		tracker: nil,
		now:     func() time.Time { return time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC) },
	}
}

// --- validation tests ---

func TestReply_NilAI(t *testing.T) {
	t.Parallel()
	svc := &ConversationService{ai: nil, model: "m"}
	_, err := svc.Reply(context.Background(), ConversationInput{
		UserID:   1,
		Messages: []ConversationMessage{{Role: "user", Content: "hi"}},
	})
	if !errors.Is(err, ErrConversationAIUnavailable) {
		t.Errorf("error = %v, want ErrConversationAIUnavailable", err)
	}
}

func TestReply_EmptyModel(t *testing.T) {
	t.Parallel()
	svc := &ConversationService{ai: &mockConversationAI{}, model: ""}
	_, err := svc.Reply(context.Background(), ConversationInput{
		UserID:   1,
		Messages: []ConversationMessage{{Role: "user", Content: "hi"}},
	})
	if !errors.Is(err, ErrConversationAIUnavailable) {
		t.Errorf("error = %v, want ErrConversationAIUnavailable", err)
	}
}

func TestReply_ZeroUserID(t *testing.T) {
	t.Parallel()
	svc := newTestConversationService(&mockConversationAI{}, &stubTaskQuerier{})
	_, err := svc.Reply(context.Background(), ConversationInput{
		UserID:   0,
		Messages: []ConversationMessage{{Role: "user", Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for zero user_id")
	}
}

func TestReply_NoMessages(t *testing.T) {
	t.Parallel()
	svc := newTestConversationService(&mockConversationAI{}, &stubTaskQuerier{})
	_, err := svc.Reply(context.Background(), ConversationInput{
		UserID:   1,
		Messages: nil,
	})
	if err == nil {
		t.Fatal("expected error for empty messages")
	}
}

// --- happy path ---

func TestReply_TextResponse(t *testing.T) {
	t.Parallel()

	ai := &mockConversationAI{
		fn: func(_ context.Context, req ConversationAIRequest, _ func(context.Context, ConversationToolCall) ConversationToolResult) (*ConversationAIResult, error) {
			if req.Model != "test-model" {
				t.Errorf("model = %q, want test-model", req.Model)
			}
			if !strings.Contains(req.Instructions, "2026-05-15") {
				t.Error("instructions should contain today's date")
			}
			if len(req.Messages) != 1 {
				t.Errorf("messages = %d, want 1", len(req.Messages))
			}
			return &ConversationAIResult{
				Message:    "Hello! How can I help?",
				ResponseID: "resp_001",
				Model:      "test-model",
			}, nil
		},
	}

	svc := newTestConversationService(ai, &stubTaskQuerier{})
	result, err := svc.Reply(context.Background(), ConversationInput{
		UserID:   1,
		Messages: []ConversationMessage{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if result.Message != "Hello! How can I help?" {
		t.Errorf("message = %q", result.Message)
	}
	if result.ResponseID != "resp_001" {
		t.Errorf("response_id = %q", result.ResponseID)
	}
}

// --- tool dispatch ---

func TestReply_ListTasksTool(t *testing.T) {
	t.Parallel()

	tasks := []sqlc.ComplianceTask{
		{ID: 1, UserID: 1, TitleEn: "Register address", Category: "municipal", Severity: "mandatory", Status: "pending"},
		{ID: 2, UserID: 1, TitleEn: "Open bank account", Category: "banking", Severity: "recommended", Status: "pending"},
	}

	ai := &mockConversationAI{
		fn: func(ctx context.Context, _ ConversationAIRequest, runTool func(context.Context, ConversationToolCall) ConversationToolResult) (*ConversationAIResult, error) {
			result := runTool(ctx, ConversationToolCall{
				Name:      "list_tasks",
				CallID:    "call_001",
				Arguments: json.RawMessage(`{"status":null,"category":null}`),
			})
			if !result.Success {
				t.Errorf("list_tasks failed: %s", result.Message)
			}
			if result.Message != "found 2 task(s)" {
				t.Errorf("message = %q", result.Message)
			}
			return &ConversationAIResult{
				Message:   "You have 2 pending tasks.",
				ToolCalls: []ConversationToolResult{result},
			}, nil
		},
	}

	svc := newTestConversationService(ai, &stubTaskQuerier{tasks: tasks})
	result, err := svc.Reply(context.Background(), ConversationInput{
		UserID:   1,
		Messages: []ConversationMessage{{Role: "user", Content: "what tasks do I have?"}},
	})
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("tool_calls = %d, want 1", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Name != "list_tasks" {
		t.Errorf("tool name = %q, want list_tasks", result.ToolCalls[0].Name)
	}
}

func TestReply_ListTasksWithFilter(t *testing.T) {
	t.Parallel()

	tasks := []sqlc.ComplianceTask{
		{ID: 1, UserID: 1, TitleEn: "Register address", Category: "municipal", Severity: "mandatory", Status: "done"},
	}

	ai := &mockConversationAI{
		fn: func(ctx context.Context, _ ConversationAIRequest, runTool func(context.Context, ConversationToolCall) ConversationToolResult) (*ConversationAIResult, error) {
			result := runTool(ctx, ConversationToolCall{
				Name:      "list_tasks",
				CallID:    "call_002",
				Arguments: json.RawMessage(`{"status":"done","category":"municipal"}`),
			})
			return &ConversationAIResult{
				Message:   "Found done tasks.",
				ToolCalls: []ConversationToolResult{result},
			}, nil
		},
	}

	svc := newTestConversationService(ai, &stubTaskQuerier{tasks: tasks})
	result, err := svc.Reply(context.Background(), ConversationInput{
		UserID:   1,
		Messages: []ConversationMessage{{Role: "user", Content: "done municipal tasks"}},
	})
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if !result.ToolCalls[0].Success {
		t.Error("expected tool call to succeed")
	}
}

func TestReply_MarkTaskDoneTool(t *testing.T) {
	t.Parallel()

	tasks := []sqlc.ComplianceTask{
		{ID: 42, UserID: 1, TitleEn: "Register address", Category: "municipal", Severity: "mandatory", Status: "pending"},
	}

	ai := &mockConversationAI{
		fn: func(ctx context.Context, _ ConversationAIRequest, runTool func(context.Context, ConversationToolCall) ConversationToolResult) (*ConversationAIResult, error) {
			result := runTool(ctx, ConversationToolCall{
				Name:      "mark_task_done",
				CallID:    "call_003",
				Arguments: json.RawMessage(`{"task_id":42}`),
			})
			if !result.Success {
				t.Errorf("mark_task_done failed: %s", result.Message)
			}
			return &ConversationAIResult{
				Message:   "Done!",
				ToolCalls: []ConversationToolResult{result},
			}, nil
		},
	}

	svc := newTestConversationService(ai, &stubTaskQuerier{tasks: tasks})
	result, err := svc.Reply(context.Background(), ConversationInput{
		UserID:   1,
		Messages: []ConversationMessage{{Role: "user", Content: "mark task 42 done"}},
	})
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if !result.ToolCalls[0].Success || result.ToolCalls[0].Message != "task marked done" {
		t.Errorf("tool result = %+v", result.ToolCalls[0])
	}
}

func TestReply_MarkTaskDone_NotFound(t *testing.T) {
	t.Parallel()

	ai := &mockConversationAI{
		fn: func(ctx context.Context, _ ConversationAIRequest, runTool func(context.Context, ConversationToolCall) ConversationToolResult) (*ConversationAIResult, error) {
			result := runTool(ctx, ConversationToolCall{
				Name:      "mark_task_done",
				CallID:    "call_004",
				Arguments: json.RawMessage(`{"task_id":999}`),
			})
			return &ConversationAIResult{
				Message:   "Task not found.",
				ToolCalls: []ConversationToolResult{result},
			}, nil
		},
	}

	svc := newTestConversationService(ai, &stubTaskQuerier{})
	result, err := svc.Reply(context.Background(), ConversationInput{
		UserID:   1,
		Messages: []ConversationMessage{{Role: "user", Content: "mark 999 done"}},
	})
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if result.ToolCalls[0].Success {
		t.Error("expected tool call to fail for missing task")
	}
}

func TestReply_MarkTaskDone_InvalidID(t *testing.T) {
	t.Parallel()

	ai := &mockConversationAI{
		fn: func(ctx context.Context, _ ConversationAIRequest, runTool func(context.Context, ConversationToolCall) ConversationToolResult) (*ConversationAIResult, error) {
			result := runTool(ctx, ConversationToolCall{
				Name:      "mark_task_done",
				CallID:    "call_005",
				Arguments: json.RawMessage(`{"task_id":0}`),
			})
			return &ConversationAIResult{
				Message:   "Invalid task.",
				ToolCalls: []ConversationToolResult{result},
			}, nil
		},
	}

	svc := newTestConversationService(ai, &stubTaskQuerier{})
	result, err := svc.Reply(context.Background(), ConversationInput{
		UserID:   1,
		Messages: []ConversationMessage{{Role: "user", Content: "mark 0 done"}},
	})
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if result.ToolCalls[0].Success {
		t.Error("expected failure for task_id 0")
	}
	if !strings.Contains(result.ToolCalls[0].Message, "positive") {
		t.Errorf("message = %q, want to mention positive", result.ToolCalls[0].Message)
	}
}

func TestReply_UnknownTool(t *testing.T) {
	t.Parallel()

	ai := &mockConversationAI{
		fn: func(ctx context.Context, _ ConversationAIRequest, runTool func(context.Context, ConversationToolCall) ConversationToolResult) (*ConversationAIResult, error) {
			result := runTool(ctx, ConversationToolCall{
				Name:      "delete_everything",
				CallID:    "call_bad",
				Arguments: json.RawMessage(`{}`),
			})
			return &ConversationAIResult{
				Message:   "I tried.",
				ToolCalls: []ConversationToolResult{result},
			}, nil
		},
	}

	svc := newTestConversationService(ai, &stubTaskQuerier{})
	result, err := svc.Reply(context.Background(), ConversationInput{
		UserID:   1,
		Messages: []ConversationMessage{{Role: "user", Content: "delete everything"}},
	})
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if result.ToolCalls[0].Success {
		t.Error("unknown tool should not succeed")
	}
	if result.ToolCalls[0].Message != "unknown tool" {
		t.Errorf("message = %q, want %q", result.ToolCalls[0].Message, "unknown tool")
	}
}

func TestReply_RecordLifeEvent_InvalidEventType(t *testing.T) {
	t.Parallel()

	ai := &mockConversationAI{
		fn: func(ctx context.Context, _ ConversationAIRequest, runTool func(context.Context, ConversationToolCall) ConversationToolResult) (*ConversationAIResult, error) {
			result := runTool(ctx, ConversationToolCall{
				Name:      "record_life_event",
				CallID:    "call_evt",
				Arguments: json.RawMessage(`{"event_type":"invented_event","occurred_at":"2026-05-15"}`),
			})
			return &ConversationAIResult{
				Message:   "Event failed.",
				ToolCalls: []ConversationToolResult{result},
			}, nil
		},
	}

	svc := newTestConversationService(ai, &stubTaskQuerier{})
	result, err := svc.Reply(context.Background(), ConversationInput{
		UserID:   1,
		Messages: []ConversationMessage{{Role: "user", Content: "something happened"}},
	})
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if result.ToolCalls[0].Success {
		t.Error("expected failure for unknown event_type")
	}
	if !strings.Contains(result.ToolCalls[0].Message, "unknown event_type") {
		t.Errorf("message = %q", result.ToolCalls[0].Message)
	}
}

func TestReply_RecordLifeEvent_InvalidDate(t *testing.T) {
	t.Parallel()

	ai := &mockConversationAI{
		fn: func(ctx context.Context, _ ConversationAIRequest, runTool func(context.Context, ConversationToolCall) ConversationToolResult) (*ConversationAIResult, error) {
			result := runTool(ctx, ConversationToolCall{
				Name:      "record_life_event",
				CallID:    "call_evt2",
				Arguments: json.RawMessage(`{"event_type":"landed_japan","occurred_at":"not-a-date"}`),
			})
			return &ConversationAIResult{
				Message:   "Bad date.",
				ToolCalls: []ConversationToolResult{result},
			}, nil
		},
	}

	svc := newTestConversationService(ai, &stubTaskQuerier{})
	result, err := svc.Reply(context.Background(), ConversationInput{
		UserID:   1,
		Messages: []ConversationMessage{{Role: "user", Content: "I landed"}},
	})
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if result.ToolCalls[0].Success {
		t.Error("expected failure for invalid date")
	}
	if !strings.Contains(result.ToolCalls[0].Message, "YYYY-MM-DD") {
		t.Errorf("message = %q", result.ToolCalls[0].Message)
	}
}

// --- helper function tests ---

func TestCompactMessages(t *testing.T) {
	t.Parallel()

	make16 := func() []ConversationMessage {
		out := make([]ConversationMessage, 20)
		for i := range out {
			out[i] = ConversationMessage{Role: "user", Content: strings.Repeat("x", i+1)}
		}
		return out
	}

	cases := []struct {
		name      string
		messages  []ConversationMessage
		limit     int
		wantLen   int
		wantFirst string
	}{
		{"under limit", []ConversationMessage{{Role: "user", Content: "a"}}, 16, 1, "a"},
		{"at limit", make16()[:16], 16, 16, "x"},
		{"over limit keeps tail", make16(), 16, 16, "xxxxx"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := compactMessages(tc.messages, tc.limit)
			if len(got) != tc.wantLen {
				t.Errorf("len = %d, want %d", len(got), tc.wantLen)
			}
			if got[0].Content != tc.wantFirst {
				t.Errorf("first content = %q, want %q", got[0].Content, tc.wantFirst)
			}
		})
	}
}

func TestDecodeToolArguments(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		raw     json.RawMessage
		wantVal string
	}{
		{"raw JSON", json.RawMessage(`{"status":"done"}`), "done"},
		{"string-encoded JSON", json.RawMessage(`"{\"status\":\"done\"}"`), "done"},
		{"empty", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out struct {
				Status string `json:"status"`
			}
			if err := decodeToolArguments(tc.raw, &out); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if out.Status != tc.wantVal {
				t.Errorf("status = %q, want %q", out.Status, tc.wantVal)
			}
		})
	}
}

func TestConversationTools_Structure(t *testing.T) {
	t.Parallel()

	tools := conversationTools()
	if len(tools) != 3 {
		t.Fatalf("tools = %d, want 3", len(tools))
	}

	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
		if tool.Type != "function" {
			t.Errorf("tool %q type = %q, want function", tool.Name, tool.Type)
		}
		if !tool.Strict {
			t.Errorf("tool %q strict = false, want true", tool.Name)
		}
		if tool.Parameters == nil {
			t.Errorf("tool %q has nil parameters", tool.Name)
		}
	}
	for _, want := range []string{"list_tasks", "mark_task_done", "record_life_event"} {
		if !names[want] {
			t.Errorf("missing tool %q", want)
		}
	}
}

func TestInstructions_ContainsDate(t *testing.T) {
	t.Parallel()

	svc := &ConversationService{
		now: func() time.Time { return time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC) },
	}
	got := svc.instructions()
	if !strings.Contains(got, "2026-08-01") {
		t.Errorf("instructions missing date, got: %s", got[:100])
	}
	if !strings.Contains(got, "Japan Concierge") {
		t.Error("instructions missing identity")
	}
}

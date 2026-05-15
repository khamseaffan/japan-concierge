package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/khamseaffan/japan-concierge/backend/internal/db/sqlc"
	"github.com/khamseaffan/japan-concierge/backend/internal/rules"
)

var ErrConversationAIUnavailable = errors.New("conversation AI is not configured")

type ConversationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ConversationInput struct {
	UserID   int64
	Messages []ConversationMessage
}

type ConversationResult struct {
	Message     string                   `json:"message"`
	ToolCalls   []ConversationToolResult `json:"tool_calls,omitempty"`
	ResponseID  string                   `json:"response_id,omitempty"`
	Model       string                   `json:"model,omitempty"`
	Unavailable bool                     `json:"unavailable,omitempty"`
}

type ConversationToolResult struct {
	Name    string `json:"name"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type ConversationAI interface {
	Respond(ctx context.Context, req ConversationAIRequest, runTool func(context.Context, ConversationToolCall) ConversationToolResult) (*ConversationAIResult, error)
}

type ConversationAIRequest struct {
	Model        string
	Instructions string
	Messages     []ConversationMessage
	Tools        []ConversationToolDefinition
}

type ConversationAIResult struct {
	Message    string
	ToolCalls  []ConversationToolResult
	ResponseID string
	Model      string
}

type ConversationToolDefinition struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Strict      bool           `json:"strict"`
	Parameters  map[string]any `json:"parameters"`
}

type ConversationToolCall struct {
	Name      string
	CallID    string
	Arguments json.RawMessage
}

type ConversationService struct {
	ai      ConversationAI
	model   string
	tasks   *TaskService
	tracker *TrackerService
	now     func() time.Time
}

func NewConversationService(ai ConversationAI, model string, tasks *TaskService, tracker *TrackerService) *ConversationService {
	return &ConversationService{
		ai:      ai,
		model:   model,
		tasks:   tasks,
		tracker: tracker,
		now:     time.Now,
	}
}

func (s *ConversationService) Reply(ctx context.Context, in ConversationInput) (*ConversationResult, error) {
	if s.ai == nil || s.model == "" {
		return nil, ErrConversationAIUnavailable
	}
	if in.UserID == 0 {
		return nil, fmt.Errorf("user_id is required")
	}
	if len(in.Messages) == 0 {
		return nil, fmt.Errorf("at least one message is required")
	}

	result, err := s.ai.Respond(ctx, ConversationAIRequest{
		Model:        s.model,
		Instructions: s.instructions(),
		Messages:     compactMessages(in.Messages, 16),
		Tools:        conversationTools(),
	}, func(ctx context.Context, call ConversationToolCall) ConversationToolResult {
		return s.runTool(ctx, in.UserID, call)
	})
	if err != nil {
		return nil, err
	}

	return &ConversationResult{
		Message:    result.Message,
		ToolCalls:  result.ToolCalls,
		ResponseID: result.ResponseID,
		Model:      result.Model,
	}, nil
}

func (s *ConversationService) instructions() string {
	today := s.now().Format("2006-01-02")
	return fmt.Sprintf(`You are Japan Concierge, a concise immigration compliance assistant.
Today is %s. The user is managing Japanese visa and municipal compliance tasks.

Use the provided tools when the user asks to inspect or change their backend state:
- list_tasks: inspect their tasks.
- mark_task_done: mark a specific task complete only when the user's intent is clear.
- record_life_event: log a known life event and generate compliance tasks.

Known life event types are: visa_application_started, visa_applied, visa_approved, coe_received, landed_japan, address_registered, address_changed, employer_changed, visa_renewal_window_opens, tax_residency_triggered.
If the request is ambiguous, ask a short clarifying question before changing state. Do not claim legal certainty; explain that the app tracks practical compliance tasks and the user should verify high-stakes immigration decisions with official sources.`, today)
}

func compactMessages(messages []ConversationMessage, limit int) []ConversationMessage {
	if len(messages) <= limit {
		return messages
	}
	return messages[len(messages)-limit:]
}

func conversationTools() []ConversationToolDefinition {
	return []ConversationToolDefinition{
		{
			Type:        "function",
			Name:        "list_tasks",
			Description: "List the user's compliance tasks, optionally filtered by status or category.",
			Strict:      true,
			Parameters: objectSchema(
				map[string]any{
					"status": map[string]any{
						"type":        []any{"string", "null"},
						"enum":        []any{"pending", "in_progress", "done", "overdue", "not_applicable", "skipped", nil},
						"description": "Optional task status filter.",
					},
					"category": map[string]any{
						"type":        []any{"string", "null"},
						"enum":        []any{"pre_arrival", "immigration", "municipal", "tax", "health_insurance", "pension", "banking", "telecom", "employer", "housing", "general", nil},
						"description": "Optional task category filter.",
					},
				},
				[]string{"status", "category"},
			),
		},
		{
			Type:        "function",
			Name:        "mark_task_done",
			Description: "Mark one compliance task as done by task id.",
			Strict:      true,
			Parameters: objectSchema(
				map[string]any{
					"task_id": map[string]any{
						"type":        "integer",
						"description": "The numeric task id to mark complete.",
					},
				},
				[]string{"task_id"},
			),
		},
		{
			Type:        "function",
			Name:        "record_life_event",
			Description: "Record a known life event for the active visa and generate any compliance tasks it triggers.",
			Strict:      true,
			Parameters: objectSchema(
				map[string]any{
					"event_type": map[string]any{
						"type":        "string",
						"enum":        []string{"visa_application_started", "visa_applied", "visa_approved", "coe_received", "landed_japan", "address_registered", "address_changed", "employer_changed", "visa_renewal_window_opens", "tax_residency_triggered"},
						"description": "The exact known life event type.",
					},
					"occurred_at": map[string]any{
						"type":        "string",
						"description": "Calendar date in YYYY-MM-DD format.",
					},
				},
				[]string{"event_type", "occurred_at"},
			),
		},
	}
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

func (s *ConversationService) runTool(ctx context.Context, userID int64, call ConversationToolCall) ConversationToolResult {
	switch call.Name {
	case "list_tasks":
		return s.toolListTasks(ctx, userID, call.Arguments)
	case "mark_task_done":
		return s.toolMarkTaskDone(ctx, userID, call.Arguments)
	case "record_life_event":
		return s.toolRecordLifeEvent(ctx, userID, call.Arguments)
	default:
		return ConversationToolResult{Name: call.Name, Success: false, Message: "unknown tool"}
	}
}

func (s *ConversationService) toolListTasks(ctx context.Context, userID int64, raw json.RawMessage) ConversationToolResult {
	var args struct {
		Status   *string `json:"status"`
		Category *string `json:"category"`
	}
	if err := decodeToolArguments(raw, &args); err != nil {
		return ConversationToolResult{Name: "list_tasks", Success: false, Message: fmt.Sprintf("invalid arguments: %v", err)}
	}

	filter := ListTasksFilter{UserID: userID}
	if args.Status != nil {
		filter.Status = *args.Status
	}
	if args.Category != nil {
		filter.Category = *args.Category
	}

	tasks, err := s.tasks.ListTasks(ctx, filter)
	if err != nil {
		return ConversationToolResult{Name: "list_tasks", Success: false, Message: err.Error()}
	}

	snapshots := make([]taskSnapshot, 0, len(tasks))
	for _, task := range tasks {
		snapshots = append(snapshots, taskToSnapshot(task))
	}
	return ConversationToolResult{
		Name:    "list_tasks",
		Success: true,
		Message: fmt.Sprintf("found %d task(s)", len(snapshots)),
		Data: map[string]any{
			"count": len(snapshots),
			"tasks": snapshots,
		},
	}
}

func (s *ConversationService) toolMarkTaskDone(ctx context.Context, userID int64, raw json.RawMessage) ConversationToolResult {
	var args struct {
		TaskID int64 `json:"task_id"`
	}
	if err := decodeToolArguments(raw, &args); err != nil {
		return ConversationToolResult{Name: "mark_task_done", Success: false, Message: fmt.Sprintf("invalid arguments: %v", err)}
	}
	if args.TaskID <= 0 {
		return ConversationToolResult{Name: "mark_task_done", Success: false, Message: "task_id must be positive"}
	}

	task, err := s.tasks.MarkDone(ctx, userID, args.TaskID)
	if err != nil {
		if errors.Is(err, ErrAlreadyDone) {
			return ConversationToolResult{Name: "mark_task_done", Success: true, Message: "task was already done", Data: taskToSnapshot(task)}
		}
		return ConversationToolResult{Name: "mark_task_done", Success: false, Message: err.Error()}
	}
	return ConversationToolResult{Name: "mark_task_done", Success: true, Message: "task marked done", Data: taskToSnapshot(task)}
}

func (s *ConversationService) toolRecordLifeEvent(ctx context.Context, userID int64, raw json.RawMessage) ConversationToolResult {
	var args struct {
		EventType  string `json:"event_type"`
		OccurredAt string `json:"occurred_at"`
	}
	if err := decodeToolArguments(raw, &args); err != nil {
		return ConversationToolResult{Name: "record_life_event", Success: false, Message: fmt.Sprintf("invalid arguments: %v", err)}
	}
	if !rules.IsKnownEventType(args.EventType) {
		return ConversationToolResult{Name: "record_life_event", Success: false, Message: fmt.Sprintf("unknown event_type %q", args.EventType)}
	}
	occurredAt, err := time.Parse("2006-01-02", args.OccurredAt)
	if err != nil {
		return ConversationToolResult{Name: "record_life_event", Success: false, Message: fmt.Sprintf("occurred_at must be YYYY-MM-DD: %v", err)}
	}

	result, err := s.tracker.RecordLifeEvent(ctx, RecordLifeEventInput{
		UserID:     userID,
		EventType:  rules.EventType(args.EventType),
		OccurredAt: occurredAt,
		Payload:    json.RawMessage(`{}`),
	})
	if err != nil {
		return ConversationToolResult{Name: "record_life_event", Success: false, Message: err.Error()}
	}

	tasks := make([]taskSnapshot, 0, len(result.Tasks))
	for _, task := range result.Tasks {
		tasks = append(tasks, taskToSnapshot(task))
	}
	return ConversationToolResult{
		Name:    "record_life_event",
		Success: true,
		Message: fmt.Sprintf("recorded %s and generated %d task(s)", args.EventType, len(tasks)),
		Data: map[string]any{
			"event": map[string]any{
				"id":          result.Event.ID,
				"event_type":  result.Event.EventType,
				"occurred_at": result.Event.OccurredAt.Time.Format("2006-01-02"),
			},
			"tasks": tasks,
		},
	}
}

func decodeToolArguments(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}

	var encoded string
	if err := json.Unmarshal(raw, &encoded); err == nil {
		raw = json.RawMessage(encoded)
	}
	return json.Unmarshal(raw, out)
}

type taskSnapshot struct {
	ID             int64   `json:"id"`
	TitleEN        string  `json:"title_en"`
	Category       string  `json:"category"`
	Severity       string  `json:"severity"`
	Status         string  `json:"status"`
	DeadlineAt     *string `json:"deadline_at,omitempty"`
	LocationHint   string  `json:"location_hint,omitempty"`
	LegalSourceURL string  `json:"legal_source_url,omitempty"`
}

func taskToSnapshot(task sqlc.ComplianceTask) taskSnapshot {
	snapshot := taskSnapshot{
		ID:       task.ID,
		TitleEN:  task.TitleEn,
		Category: task.Category,
		Severity: task.Severity,
		Status:   task.Status,
	}
	if task.DeadlineAt.Valid {
		s := task.DeadlineAt.Time.Format("2006-01-02")
		snapshot.DeadlineAt = &s
	}
	if task.LocationHint.Valid {
		snapshot.LocationHint = task.LocationHint.String
	}
	if task.LegalSourceUrl.Valid {
		snapshot.LegalSourceURL = task.LegalSourceUrl.String
	}
	return snapshot
}

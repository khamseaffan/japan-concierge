package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAIRespond_TextOnly(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/responses" {
			t.Errorf("path = %s, want /responses", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("auth header = %q, want %q", got, "Bearer test-key")
		}

		var body aiResponseRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.Model != "test-model" {
			t.Errorf("model = %q, want test-model", body.Model)
		}

		json.NewEncoder(w).Encode(aiResponse{
			ID:    "resp_001",
			Model: "test-model",
			Output: []aiResponseItem{
				{
					Type: "message",
					Content: []aiResponsePart{
						{Type: "output_text", Text: "Hello from AI"},
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := NewAIResponsesClient("test-key", srv.URL)
	result, err := client.Respond(context.Background(), ConversationAIRequest{
		Model:    "test-model",
		Messages: []ConversationMessage{{Role: "user", Content: "hi"}},
	}, func(_ context.Context, _ ConversationToolCall) ConversationToolResult {
		t.Fatal("runTool should not be called for text-only response")
		return ConversationToolResult{}
	})
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if result.Message != "Hello from AI" {
		t.Errorf("message = %q, want %q", result.Message, "Hello from AI")
	}
	if result.ResponseID != "resp_001" {
		t.Errorf("response_id = %q, want %q", result.ResponseID, "resp_001")
	}
	if result.Model != "test-model" {
		t.Errorf("model = %q, want %q", result.Model, "test-model")
	}
	if len(result.ToolCalls) != 0 {
		t.Errorf("tool_calls = %d, want 0", len(result.ToolCalls))
	}
}

func TestAIRespond_ToolCallRoundTrip(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)

		body, _ := io.ReadAll(r.Body)
		var req aiResponseRequest
		json.Unmarshal(body, &req)

		switch n {
		case 1:
			json.NewEncoder(w).Encode(aiResponse{
				ID:    "resp_001",
				Model: "test-model",
				Output: []aiResponseItem{
					{
						Type:      "function_call",
						Name:      "list_tasks",
						CallID:    "call_abc",
						Arguments: json.RawMessage(`{"status":null,"category":null}`),
					},
				},
			})
		case 2:
			if req.PreviousResponseID != "resp_001" {
				t.Errorf("previous_response_id = %q, want resp_001", req.PreviousResponseID)
			}
			if len(req.Input) != 1 || req.Input[0].Type != "function_call_output" {
				t.Errorf("expected function_call_output input, got %+v", req.Input)
			}
			if req.Input[0].CallID != "call_abc" {
				t.Errorf("input call_id = %q, want call_abc", req.Input[0].CallID)
			}

			json.NewEncoder(w).Encode(aiResponse{
				ID:    "resp_002",
				Model: "test-model",
				Output: []aiResponseItem{
					{
						Type:    "message",
						Content: []aiResponsePart{{Type: "output_text", Text: "Here are your tasks"}},
					},
				},
			})
		default:
			t.Fatalf("unexpected call %d", n)
		}
	}))
	defer srv.Close()

	var toolCalled bool
	client := NewAIResponsesClient("test-key", srv.URL)
	result, err := client.Respond(context.Background(), ConversationAIRequest{
		Model:    "test-model",
		Messages: []ConversationMessage{{Role: "user", Content: "show tasks"}},
	}, func(_ context.Context, call ConversationToolCall) ConversationToolResult {
		toolCalled = true
		if call.Name != "list_tasks" {
			t.Errorf("tool name = %q, want list_tasks", call.Name)
		}
		if call.CallID != "call_abc" {
			t.Errorf("call_id = %q, want call_abc", call.CallID)
		}
		return ConversationToolResult{Name: "list_tasks", Success: true, Message: "found 2 tasks"}
	})
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if !toolCalled {
		t.Error("runTool was never called")
	}
	if result.Message != "Here are your tasks" {
		t.Errorf("message = %q, want %q", result.Message, "Here are your tasks")
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("tool_calls = %d, want 1", len(result.ToolCalls))
	}
	if !result.ToolCalls[0].Success {
		t.Error("tool_calls[0].success = false, want true")
	}
}

func TestAIRespond_HTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	client := NewAIResponsesClient("test-key", srv.URL)
	_, err := client.Respond(context.Background(), ConversationAIRequest{
		Model:    "test-model",
		Messages: []ConversationMessage{{Role: "user", Content: "hi"}},
	}, nil)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want to contain 500", err.Error())
	}
}

func TestAIRespond_MaxRoundsExceeded(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := callCount.Add(1)
		json.NewEncoder(w).Encode(aiResponse{
			ID:    "resp_loop",
			Model: "test-model",
			Output: []aiResponseItem{
				{
					Type:      "function_call",
					Name:      "list_tasks",
					CallID:    "call_" + string(rune('0'+n)),
					Arguments: json.RawMessage(`{}`),
				},
			},
		})
	}))
	defer srv.Close()

	client := NewAIResponsesClient("test-key", srv.URL)
	_, err := client.Respond(context.Background(), ConversationAIRequest{
		Model:    "test-model",
		Messages: []ConversationMessage{{Role: "user", Content: "loop"}},
	}, func(_ context.Context, _ ConversationToolCall) ConversationToolResult {
		return ConversationToolResult{Name: "list_tasks", Success: true, Message: "ok"}
	})
	if err == nil {
		t.Fatal("expected error for max rounds")
	}
	if !strings.Contains(err.Error(), "maximum tool-call rounds") {
		t.Errorf("error = %q, want to contain 'maximum tool-call rounds'", err.Error())
	}
	if got := int(callCount.Load()); got != 4 {
		t.Errorf("API calls = %d, want 4 (max rounds)", got)
	}
}

func TestAIRespond_EmptyCredentials(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		apiKey  string
		baseURL string
	}{
		{"empty key", "", "http://localhost"},
		{"empty url", "key", ""},
		{"both empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewAIResponsesClient(tc.apiKey, tc.baseURL)
			_, err := client.Respond(context.Background(), ConversationAIRequest{
				Model:    "m",
				Messages: []ConversationMessage{{Role: "user", Content: "hi"}},
			}, nil)
			if err != ErrConversationAIUnavailable {
				t.Errorf("error = %v, want ErrConversationAIUnavailable", err)
			}
		})
	}
}

func TestMessagesToResponsesInput(t *testing.T) {
	t.Parallel()

	messages := []ConversationMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
		{Role: "system", Content: "ignored role"},
	}
	got := messagesToResponsesInput(messages)

	if len(got) != 3 {
		t.Fatalf("items = %d, want 3", len(got))
	}

	if got[0].Role != "user" || got[0].Content[0].Type != "input_text" {
		t.Errorf("item[0] role=%q type=%q, want user/input_text", got[0].Role, got[0].Content[0].Type)
	}
	if got[1].Role != "assistant" || got[1].Content[0].Type != "output_text" {
		t.Errorf("item[1] role=%q type=%q, want assistant/output_text", got[1].Role, got[1].Content[0].Type)
	}
	if got[2].Role != "user" {
		t.Errorf("item[2] role=%q, want user (unknown roles default to user)", got[2].Role)
	}
}

func TestOutputText(t *testing.T) {
	t.Parallel()

	resp := aiResponse{
		Output: []aiResponseItem{
			{Type: "function_call", Name: "x"},
			{
				Type: "message",
				Content: []aiResponsePart{
					{Type: "output_text", Text: "first"},
					{Type: "refusal", Text: "ignored"},
				},
			},
			{
				Type:    "message",
				Content: []aiResponsePart{{Type: "output_text", Text: "second"}},
			},
		},
	}
	got := resp.outputText()
	want := "first\n\nsecond"
	if got != want {
		t.Errorf("outputText = %q, want %q", got, want)
	}
}

func TestFunctionCalls(t *testing.T) {
	t.Parallel()

	resp := aiResponse{
		Output: []aiResponseItem{
			{Type: "message", Content: []aiResponsePart{{Type: "output_text", Text: "hi"}}},
			{Type: "function_call", Name: "list_tasks", CallID: "c1", Arguments: json.RawMessage(`{}`)},
			{Type: "function_call", Name: "mark_task_done", CallID: "c2", Arguments: json.RawMessage(`{"task_id":1}`)},
		},
	}
	calls := resp.functionCalls()
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(calls))
	}
	if calls[0].Name != "list_tasks" || calls[0].CallID != "c1" {
		t.Errorf("calls[0] = %+v", calls[0])
	}
	if calls[1].Name != "mark_task_done" || calls[1].CallID != "c2" {
		t.Errorf("calls[1] = %+v", calls[1])
	}
}

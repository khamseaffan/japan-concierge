package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type AIResponsesClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewAIResponsesClient(apiKey, baseURL string) *AIResponsesClient {
	return &AIResponsesClient{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 25 * time.Second},
	}
}

func (c *AIResponsesClient) Respond(ctx context.Context, req ConversationAIRequest, runTool func(context.Context, ConversationToolCall) ConversationToolResult) (*ConversationAIResult, error) {
	if c.apiKey == "" || c.baseURL == "" {
		return nil, ErrConversationAIUnavailable
	}

	input := messagesToResponsesInput(req.Messages)
	var toolResults []ConversationToolResult
	var previousResponseID string
	var model string

	for range 4 {
		response, err := c.create(ctx, aiResponseRequest{
			Model:              req.Model,
			Instructions:       req.Instructions,
			Input:              input,
			Tools:              req.Tools,
			PreviousResponseID: previousResponseID,
			ToolChoice:         "auto",
		})
		if err != nil {
			return nil, err
		}
		previousResponseID = response.ID
		model = response.Model

		calls := response.functionCalls()
		if len(calls) == 0 {
			return &ConversationAIResult{
				Message:    response.outputText(),
				ToolCalls:  toolResults,
				ResponseID: response.ID,
				Model:      model,
			}, nil
		}

		input = make([]aiInputItem, 0, len(calls))
		for _, call := range calls {
			result := runTool(ctx, call)
			toolResults = append(toolResults, result)

			encoded, err := json.Marshal(result)
			if err != nil {
				encoded = []byte(`{"success":false,"message":"failed to encode tool result"}`)
			}
			input = append(input, aiInputItem{
				Type:   "function_call_output",
				CallID: call.CallID,
				Output: string(encoded),
			})
		}
	}

	return nil, fmt.Errorf("conversation exceeded maximum tool-call rounds")
}

func (c *AIResponsesClient) create(ctx context.Context, body aiResponseRequest) (*aiResponse, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode AI request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/responses", bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("build AI request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	res, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call AI responses API: %w", err)
	}
	defer res.Body.Close()

	data, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read AI response: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("AI responses API returned %d: %s", res.StatusCode, string(data))
	}

	var decoded aiResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("decode AI response: %w", err)
	}
	return &decoded, nil
}

type aiResponseRequest struct {
	Model              string                       `json:"model"`
	Instructions       string                       `json:"instructions,omitempty"`
	Input              []aiInputItem                `json:"input"`
	Tools              []ConversationToolDefinition `json:"tools,omitempty"`
	PreviousResponseID string                       `json:"previous_response_id,omitempty"`
	ToolChoice         string                       `json:"tool_choice,omitempty"`
}

type aiInputItem struct {
	Role    string           `json:"role,omitempty"`
	Content []aiContentBlock `json:"content,omitempty"`
	Type    string           `json:"type,omitempty"`
	CallID  string           `json:"call_id,omitempty"`
	Output  string           `json:"output,omitempty"`
}

type aiContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type aiResponse struct {
	ID     string           `json:"id"`
	Model  string           `json:"model"`
	Output []aiResponseItem `json:"output"`
}

type aiResponseItem struct {
	Type      string           `json:"type"`
	Name      string           `json:"name,omitempty"`
	CallID    string           `json:"call_id,omitempty"`
	Arguments json.RawMessage  `json:"arguments,omitempty"`
	Content   []aiResponsePart `json:"content,omitempty"`
}

type aiResponsePart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func messagesToResponsesInput(messages []ConversationMessage) []aiInputItem {
	out := make([]aiInputItem, 0, len(messages))
	for _, msg := range messages {
		role := msg.Role
		if role != "user" && role != "assistant" {
			role = "user"
		}
		out = append(out, aiInputItem{
			Role: role,
			Content: []aiContentBlock{
				{Type: inputContentType(role), Text: msg.Content},
			},
		})
	}
	return out
}

func inputContentType(role string) string {
	if role == "assistant" {
		return "output_text"
	}
	return "input_text"
}

func (r aiResponse) functionCalls() []ConversationToolCall {
	calls := make([]ConversationToolCall, 0)
	for _, item := range r.Output {
		if item.Type != "function_call" {
			continue
		}
		calls = append(calls, ConversationToolCall{
			Name:      item.Name,
			CallID:    item.CallID,
			Arguments: item.Arguments,
		})
	}
	return calls
}

func (r aiResponse) outputText() string {
	var b strings.Builder
	for _, item := range r.Output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if part.Type != "output_text" || part.Text == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(part.Text)
		}
	}
	return b.String()
}

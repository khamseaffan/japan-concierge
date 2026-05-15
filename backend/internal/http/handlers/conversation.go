package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	httpmw "github.com/khamseaffan/japan-concierge/backend/internal/http/middleware"
	"github.com/khamseaffan/japan-concierge/backend/internal/service"
)

type ConversationHandler struct {
	Conversation *service.ConversationService
}

type ConversationRequest struct {
	Messages []service.ConversationMessage `json:"messages"`
}

func (h *ConversationHandler) Reply(w http.ResponseWriter, r *http.Request) {
	logger := httpmw.LoggerFromContext(r.Context())

	userID, ok := httpmw.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no user context")
		return
	}

	var req ConversationRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("malformed JSON: %v", err))
		return
	}
	if err := validateConversationRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.Conversation.Reply(r.Context(), service.ConversationInput{
		UserID:   userID,
		Messages: req.Messages,
	})
	if err != nil {
		if errors.Is(err, service.ErrConversationAIUnavailable) {
			writeError(w, http.StatusServiceUnavailable, "conversation AI is not configured")
			return
		}
		logger.Error("Conversation reply failed", "user_id", userID, "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	logger.Info("conversation reply created",
		"user_id", userID,
		"tool_calls", len(result.ToolCalls),
		"response_id", result.ResponseID,
	)
	writeJSON(w, http.StatusOK, result)
}

func validateConversationRequest(req ConversationRequest) error {
	if len(req.Messages) == 0 {
		return fmt.Errorf("messages is required")
	}
	if len(req.Messages) > 32 {
		return fmt.Errorf("messages is limited to 32 items")
	}
	for i, msg := range req.Messages {
		if msg.Role != "user" && msg.Role != "assistant" {
			return fmt.Errorf("messages[%d].role must be user or assistant", i)
		}
		if strings.TrimSpace(msg.Content) == "" {
			return fmt.Errorf("messages[%d].content is required", i)
		}
		if len(msg.Content) > 4000 {
			return fmt.Errorf("messages[%d].content is too long", i)
		}
	}
	return nil
}

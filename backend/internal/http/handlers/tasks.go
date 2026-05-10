package handlers

import (
	"net/http"
	"strconv"

	httpmw "github.com/khamseaffan/japan-concierge/backend/internal/http/middleware"
	"github.com/khamseaffan/japan-concierge/backend/internal/service"
)

// TasksHandler holds dependencies for the task endpoints.
type TasksHandler struct {
	Tasks *service.TaskService
}

// validTaskStatuses mirrors the CHECK constraint in the schema. Listed here
// so we can reject bad query params with 400 instead of letting them silently
// match nothing.
var validTaskStatuses = map[string]bool{
	"pending":        true,
	"in_progress":    true,
	"done":           true,
	"overdue":        true,
	"not_applicable": true,
	"skipped":        true,
}

var validTaskCategories = map[string]bool{
	"pre_arrival":      true,
	"immigration":      true,
	"municipal":        true,
	"tax":              true,
	"health_insurance": true,
	"pension":          true,
	"banking":          true,
	"telecom":          true,
	"employer":         true,
	"housing":          true,
	"general":          true,
}

// List handles GET /api/v1/tasks?status=&visa_id=&category=.
func (h *TasksHandler) List(w http.ResponseWriter, r *http.Request) {
	logger := httpmw.LoggerFromContext(r.Context())

	userID, ok := httpmw.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no user context")
		return
	}

	q := r.URL.Query()
	filter := service.ListTasksFilter{UserID: userID}

	if s := q.Get("status"); s != "" {
		if !validTaskStatuses[s] {
			writeError(w, http.StatusBadRequest, "invalid status filter")
			return
		}
		filter.Status = s
	}
	if s := q.Get("category"); s != "" {
		if !validTaskCategories[s] {
			writeError(w, http.StatusBadRequest, "invalid category filter")
			return
		}
		filter.Category = s
	}
	if s := q.Get("visa_id"); s != "" {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil || v <= 0 {
			writeError(w, http.StatusBadRequest, "visa_id must be a positive integer")
			return
		}
		filter.VisaID = v
	}

	rows, err := h.Tasks.ListTasks(r.Context(), filter)
	if err != nil {
		logger.Error("ListTasks failed", "user_id", userID, "err", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]TaskDTO, 0, len(rows))
	for _, t := range rows {
		out = append(out, taskToDTO(t))
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": out, "count": len(out)})
}

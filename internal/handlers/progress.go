package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"courses-hub/internal/db"
)

// progressRequest is the JSON body for POST /api/progress.
type progressRequest struct {
	Slug   string `json:"slug"`
	Status string `json:"status"`
}

// UpdateProgress handles POST /api/progress and upserts a course's status.
func (h *Handlers) UpdateProgress(w http.ResponseWriter, r *http.Request) {
	var req progressRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Slug == "" {
		writeError(w, http.StatusBadRequest, "slug is required")
		return
	}
	if !db.ValidStatus(req.Status) {
		writeError(w, http.StatusBadRequest, "status must be one of: not_started, in_progress, completed")
		return
	}

	if err := h.DB.UpsertProgress(req.Slug, req.Status); err != nil {
		if errors.Is(err, db.ErrCourseNotFound) {
			writeError(w, http.StatusNotFound, "unknown course slug")
			return
		}
		log.Printf("UpdateProgress: %v", err)
		writeError(w, http.StatusInternalServerError, "could not update progress")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"slug": req.Slug, "status": req.Status})
}

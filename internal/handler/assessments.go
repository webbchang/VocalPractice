package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"vocal-practice-app/internal/domain"
	"vocal-practice-app/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type UserAssessmentsHandler struct {
	store storage.Store
}

func NewUserAssessmentsHandler(store storage.Store) *UserAssessmentsHandler {
	return &UserAssessmentsHandler{store: store}
}

type submitAssessmentRequest struct {
	SongID                   string                  `json:"song_id"`
	StructureID              *string                 `json:"structure_id,omitempty"`
	TrackID                  string                  `json:"track_id"`
	Score                    float64                 `json:"score"`
	TotalNotes               int                     `json:"total_notes"`
	MatchedNotes             int                     `json:"matched_notes"`
	AveragePitchDeviation    float64                 `json:"average_pitch_deviation"`
	AverageDurationDeviation float64                 `json:"average_duration_deviation"`
	PitchDeviation           []float64               `json:"pitch_deviation"`
	DurationDeviation        []float64               `json:"duration_deviation"`
	NoteComparison           []domain.NoteComparison `json:"note_comparison"`
}

func (h *UserAssessmentsHandler) Submit(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(uuid.UUID)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	var req submitAssessmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	songID, err := uuid.Parse(req.SongID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid song_id")
		return
	}

	trackID, err := uuid.Parse(req.TrackID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid track_id")
		return
	}

	var structureID *uuid.UUID
	if req.StructureID != nil {
		if sid, err := uuid.Parse(*req.StructureID); err == nil {
			structureID = &sid
		}
	}

	assessment := domain.NewUserAssessment(
		userID, songID, trackID, structureID,
		req.Score, req.TotalNotes, req.MatchedNotes,
		req.AveragePitchDeviation, req.AverageDurationDeviation,
		req.PitchDeviation, req.DurationDeviation,
		req.NoteComparison,
	)

	if err := h.store.CreateAssessment(assessment); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save assessment")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"assessment_id": assessment.ID,
		"status":        "saved",
	})
}

func (h *UserAssessmentsHandler) ListMyAssessments(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(uuid.UUID)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	query := r.URL.Query()

	var songID *uuid.UUID
	if sidStr := query.Get("song_id"); sidStr != "" {
		if sid, err := uuid.Parse(sidStr); err == nil {
			songID = &sid
		}
	}

	limit := 20
	offset := 0
	if limitStr := query.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	assessments, err := h.store.ListAssessmentsByUser(userID, songID, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list assessments")
		return
	}

	if assessments == nil {
		assessments = make([]*domain.UserAssessment, 0)
	}

	respondJSON(w, http.StatusOK, assessments)
}

func (h *UserAssessmentsHandler) GetAssessment(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(uuid.UUID)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	assessmentIDStr := chi.URLParam(r, "assessment_id")
	assessmentID, err := uuid.Parse(assessmentIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid assessment_id")
		return
	}

	assessment, err := h.store.GetAssessmentByID(assessmentID)
	if err != nil {
		respondError(w, http.StatusNotFound, "assessment not found")
		return
	}

	if assessment.UserID != userID {
		respondError(w, http.StatusForbidden, "access denied")
		return
	}

	respondJSON(w, http.StatusOK, assessment)
}

func (h *UserAssessmentsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(uuid.UUID)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	stats, err := h.store.GetAssessmentStatsByUser(userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}

	stats["streak_days"] = 0 // Placeholder: would need date tracking in production
	respondJSON(w, http.StatusOK, stats)
}

func (h *UserAssessmentsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(uuid.UUID)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	assessmentIDStr := chi.URLParam(r, "assessment_id")
	assessmentID, err := uuid.Parse(assessmentIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid assessment_id")
		return
	}

	assessment, err := h.store.GetAssessmentByID(assessmentID)
	if err != nil {
		respondError(w, http.StatusNotFound, "assessment not found")
		return
	}

	if assessment.UserID != userID {
		respondError(w, http.StatusForbidden, "access denied")
		return
	}

	if err := h.store.DeleteAssessment(assessmentID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete assessment")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *UserAssessmentsHandler) Download(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(UserIDKey).(uuid.UUID)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	assessmentIDStr := chi.URLParam(r, "assessment_id")
	assessmentID, err := uuid.Parse(assessmentIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid assessment_id")
		return
	}

	assessment, err := h.store.GetAssessmentByID(assessmentID)
	if err != nil {
		respondError(w, http.StatusNotFound, "assessment not found")
		return
	}

	if assessment.UserID != userID {
		respondError(w, http.StatusForbidden, "access denied")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"assessment_"+assessmentID.String()+".json\"")
	respondJSON(w, http.StatusOK, assessment)
}

// TrackAudioHandler serves audio for specific tracks (stub)
type TrackAudioHandler struct {
	store storage.Store
}

func NewTrackAudioHandler(store storage.Store) *TrackAudioHandler {
	return &TrackAudioHandler{store: store}
}

func (h *TrackAudioHandler) ServeTrackAudio(w http.ResponseWriter, r *http.Request) {
	// Stub: In production, this would generate or serve pre-rendered audio for a specific MIDI track
	respondError(w, http.StatusNotImplemented, "track audio generation not yet implemented")
}

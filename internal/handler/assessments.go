package handler

import (
	"encoding/base64"
	"encoding/json"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"vocal-practice-app/internal/domain"
	"vocal-practice-app/internal/service"
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

type analyzeRequest struct {
	AudioData      string            `json:"audio_data"`
	AudioFormat    string            `json:"audio_format,omitempty"`
	ReferenceNotes []domain.MIDINote `json:"reference_notes"`
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

// AnalyzeRecording handles the analysis of uploaded audio data
func (h *UserAssessmentsHandler) AnalyzeRecording(w http.ResponseWriter, r *http.Request) {
	_, ok := r.Context().Value(UserIDKey).(uuid.UUID)
	if !ok {
		respondError(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	var req analyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.AudioData == "" {
		respondError(w, http.StatusBadRequest, "audio_data is required")
		return
	}

	audioBytes, err := base64.StdEncoding.DecodeString(req.AudioData)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid audio data (base64 decode failed)")
		return
	}

	samples, sampleRate, err := service.DecodeAudioData(audioBytes, req.AudioFormat)
	if err != nil {
		respondError(w, http.StatusBadRequest, "audio decode failed: "+err.Error())
		return
	}

	result, err := service.AssessRecordingWithVowelFiltering(samples, sampleRate, req.ReferenceNotes)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "assessment failed: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
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

	// 檢查請求的格式（透過 Accept header 或 query parameter）
	format := r.URL.Query().Get("format")
	if format == "" {
		// 從 Accept header 推斷格式
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "text/csv") {
			format = "csv"
		} else if strings.Contains(accept, "application/pdf") {
			format = "pdf"
		} else {
			format = "json" // 預設格式
		}
	}

	switch format {
	case "csv":
		h.downloadCSV(w, r, assessment)
	case "pdf":
		h.downloadPDF(w, r, assessment)
	default: // json 和其他未知格式預設為 JSON
		h.downloadJSON(w, r, assessment)
	}
}

// downloadJSON 以 JSON 格式下載評估
func (h *UserAssessmentsHandler) downloadJSON(w http.ResponseWriter, r *http.Request, assessment *domain.UserAssessment) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"assessment_"+assessment.ID.String()+".json\"")
	respondJSON(w, http.StatusOK, assessment)
}

// downloadCSV 以 CSV 格式下載評估
func (h *UserAssessmentsHandler) downloadCSV(w http.ResponseWriter, r *http.Request, assessment *domain.UserAssessment) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=\"assessment_"+assessment.ID.String()+".csv\"")
	
	var buf strings.Builder
	
	// 標題行
	header := []string{
		"AssessmentID", "UserID", "SongID", "TrackID", "StructureID",
		"Score", "TotalNotes", "MatchedNotes", 
		"AveragePitchDeviation", "AverageDurationDeviation",
		"CreatedAt",
	}
	
	// 寫入標題
	for i, v := range header {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(strconv.Quote(v))
	}
	buf.WriteString("\n")
	
// 數據行
		var structureIDStr string
		if assessment.StructureID != nil {
			structureIDStr = assessment.StructureID.String()
		}
		record := []string{
			assessment.ID.String(),
			assessment.UserID.String(),
			assessment.SongID.String(),
			assessment.TrackID.String(),
			structureIDStr,
			fmt.Sprintf("%.2f", assessment.Score),
			strconv.Itoa(assessment.TotalNotes),
			strconv.Itoa(assessment.MatchedNotes),
			fmt.Sprintf("%.2f", assessment.AveragePitchDeviation),
			fmt.Sprintf("%.3f", assessment.AverageDurationDeviation),
			assessment.CreatedAt.Format(time.RFC3339),
		}
	
	for i, v := range record {
		if i > 0 {
			buf.WriteString(",")
		}
		if v == "" {
			buf.WriteString(`""`)
		} else {
			buf.WriteString(strconv.Quote(v))
		}
	}
	buf.WriteString("\n")
	
	// 添加詳細的音符比較資料（如果有的話）
	if len(assessment.NoteComparison) > 0 {
		buf.WriteString("\n\nNote Comparison Details\n")
		buf.WriteString("NoteIndex,RefPitch,RefStart,RefEnd,UserPitch,UserStart,UserEnd,MatchStatus,PitchDeviationCents,DurationDeviationSec\n")
		
		for i, nc := range assessment.NoteComparison {
			userPitch := 0
			userStart := 0.0
			userEnd := 0.0
			matchStatus := ""
			pitchDev := 0.0
			durationDev := 0.0
			
			userPitch = nc.UserPitch
			userStart = nc.UserStart
			userEnd = nc.UserEnd
			matchStatus = nc.MatchStatus
			pitchDev = nc.PitchDeviationCents
			durationDev = nc.DurationDeviationSec
			
			buf.WriteString(fmt.Sprintf("%d,%d,%f,%f,%d,%f,%f,%s,%f,%f\n", 
				i, nc.RefPitch, nc.RefStart, nc.RefEnd, userPitch, userStart, userEnd, 
				matchStatus, pitchDev, durationDev))
		}
	}
	
	io.Copy(w, strings.NewReader(buf.String()))
}

// downloadPDF 以 PDF 格式下載評估
// 注意：這是一個簡化的實作。在生產環境中，這應該使用像 gofpdf 這樣的 PDF 函式庫
func (h *UserAssessmentsHandler) downloadPDF(w http.ResponseWriter, r *http.Request, assessment *domain.UserAssessment) {
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=\"assessment_"+assessment.ID.String()+".pdf\"")
	
	// 在真實的應用中，這裡會使用 PDF 函式庫來生成真正的 PDF
	// 為了這個練習，我們將返回一個簡單的文字說明，表示在生產環境中
	// 這裡會使用適當的 PDF 函式庫
	
	var buf bytes.Buffer
	
	// PDF 標頭
	buf.WriteString("%PDF-1.4\n")
	buf.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	buf.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	buf.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Contents 4 0 R >>\nendobj\n")
	buf.WriteString("4 0 obj\n<< /Length 5 0 R >>\nstream\n")
	
	// PDF 內容（簡化的文字）
	text := fmt.Sprintf(
		"Vocal Practice Assessment Report\n\n"+
			"Assessment ID: %s\n"+
			"User ID: %s\n"+
			"Song ID: %s\n"+
			"Track ID: %s\n"+
			"Score: %.2f/100\n"+
			"Total Notes: %d\n"+
			"Matched Notes: %d\n"+
			"Average Pitch Deviation: %.2f cents\n"+
			"Average Duration Deviation: %.3f seconds\n",
		assessment.ID, assessment.UserID, assessment.SongID, assessment.TrackID,
		assessment.Score, assessment.TotalNotes, assessment.MatchedNotes,
		assessment.AveragePitchDeviation, assessment.AverageDurationDeviation)
	
	// 添加音符比較摘要
	if assessment.NoteComparison != nil && len(assessment.NoteComparison) > 0 {
		text += "\nNote Comparison Summary:\n"
		matchedCount := 0
		for _, nc := range assessment.NoteComparison {
			if nc.MatchStatus == "matched" {
				matchedCount++
			}
		}
		text += fmt.Sprintf("Matched Notes: %d/%d\n", matchedCount, len(assessment.NoteComparison))
	}
	
	// 將文字轉換為 PDF 內容（簡化版）
	// 在真實的 PDF 中，你需要正確地編碼文字和位置
	buf.WriteString(fmt.Sprintf("%d 0 obj\n(%s) Tj\nET\nendstream\nendobj\n", 
		len([]byte(text)), strings.ReplaceAll(strings.ReplaceAll(text, ")", "\\)"), "(", "\\(")))
	buf.WriteString("5 0 obj\n")
	buf.WriteString(fmt.Sprintf("%d\n", len(buf.String())-23)) // 簡化的長度計算
	buf.WriteString("\nendobj\n")
	buf.WriteString("xref\n0 6\n0000000000 65535 f \n")
	buf.WriteString("0000000010 00000 n \n")
	buf.WriteString("0000000053 00000 n \n")
	buf.WriteString("0000000109 00000 n \n")
	buf.WriteString("0000000221 00000 n \n")
	buf.WriteString("0000000300 00000 n \n")
	buf.WriteString("trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n380\n%%EOF\n")
	
	io.Copy(w, &buf)
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

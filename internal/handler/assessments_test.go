package handler

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vocal-practice-app/internal/domain"
	"vocal-practice-app/internal/service"
	"vocal-practice-app/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func makeAnalyzeTestRouter(store storage.Store, jwtSecret string) (*chi.Mux, *AuthHandler, uuid.UUID) {
	authHandler := NewAuthHandler(store, jwtSecret)
	assessmentsHandler := NewUserAssessmentsHandler(store)

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authHandler.Middleware)
		r.Post("/assessments/analyze", assessmentsHandler.AnalyzeRecording)
	})
	return r, authHandler, adminID
}

func makeSineWAVBytes(freq float64, durationSec float64, sampleRate int, bitsPerSample int) []byte {
	numSamples := int(durationSec * float64(sampleRate))
	bytesPerSample := bitsPerSample / 8
	audioFormat := 1 // PCM
	if bitsPerSample == 32 {
		audioFormat = 3 // IEEE float
	}

	buf := make([]byte, 44+numSamples*bytesPerSample)

	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+numSamples*bytesPerSample))
	copy(buf[8:12], "WAVE")

	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], uint16(audioFormat))
	binary.LittleEndian.PutUint16(buf[22:24], 1) // mono
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(sampleRate*bytesPerSample))
	binary.LittleEndian.PutUint16(buf[32:34], uint16(bytesPerSample))
	binary.LittleEndian.PutUint16(buf[34:36], uint16(bitsPerSample))

	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(numSamples*bytesPerSample))

	for i := 0; i < numSamples; i++ {
		sec := float64(i) / float64(sampleRate)
		sample := math.Sin(2 * math.Pi * freq * sec)
		off := 44 + i*bytesPerSample
		if bitsPerSample == 16 {
			val := int16(sample * 32767)
			binary.LittleEndian.PutUint16(buf[off:], uint16(val))
		} else if bitsPerSample == 32 {
			binary.LittleEndian.PutUint32(buf[off:], math.Float32bits(float32(sample)))
		}
	}

	return buf
}

func makeInvalidWAV() []byte {
	buf := make([]byte, 44)
	copy(buf[0:4], "RIFF")
	copy(buf[8:12], "WAVE")
	return buf
}

func TestAnalyzeRecordingRejectsMissingAuth(t *testing.T) {
	store := storage.NewMemoryStore()
	r, _, _ := makeAnalyzeTestRouter(store, "test-secret")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assessments/analyze", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestAnalyzeRecordingRejectsInvalidJSON(t *testing.T) {
	store := storage.NewMemoryStore()
	r, authHandler, userID := makeAnalyzeTestRouter(store, "test-secret")
	token, _ := authHandler.generateToken(userID, "admin@localhost")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assessments/analyze", strings.NewReader("not json"))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestAnalyzeRecordingRejectsMissingAudioData(t *testing.T) {
	store := storage.NewMemoryStore()
	r, authHandler, userID := makeAnalyzeTestRouter(store, "test-secret")
	token, _ := authHandler.generateToken(userID, "admin@localhost")

	body, _ := json.Marshal(map[string]interface{}{
		"audio_format":    "wav",
		"reference_notes": []domain.MIDINote{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assessments/analyze", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestAnalyzeRecordingRejectsInvalidBase64(t *testing.T) {
	store := storage.NewMemoryStore()
	r, authHandler, userID := makeAnalyzeTestRouter(store, "test-secret")
	token, _ := authHandler.generateToken(userID, "admin@localhost")

	body, _ := json.Marshal(map[string]interface{}{
		"audio_data":      "!!!not-base64!!!",
		"audio_format":    "wav",
		"reference_notes": []domain.MIDINote{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assessments/analyze", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestAnalyzeRecordingRejectsInvalidWAV(t *testing.T) {
	store := storage.NewMemoryStore()
	r, authHandler, userID := makeAnalyzeTestRouter(store, "test-secret")
	token, _ := authHandler.generateToken(userID, "admin@localhost")

	wavBytes := makeInvalidWAV()
	audioB64 := base64.StdEncoding.EncodeToString(wavBytes)

	body, _ := json.Marshal(map[string]interface{}{
		"audio_data":      audioB64,
		"audio_format":    "wav",
		"reference_notes": []domain.MIDINote{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assessments/analyze", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestAnalyzeRecordingRejectsUnsupportedFormat(t *testing.T) {
	store := storage.NewMemoryStore()
	r, authHandler, userID := makeAnalyzeTestRouter(store, "test-secret")
	token, _ := authHandler.generateToken(userID, "admin@localhost")

	body, _ := json.Marshal(map[string]interface{}{
		"audio_data":      base64.StdEncoding.EncodeToString([]byte("fake-data")),
		"audio_format":    "mp3",
		"reference_notes": []domain.MIDINote{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assessments/analyze", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestAnalyzeRecordingWithEmptyReferenceNotes(t *testing.T) {
	store := storage.NewMemoryStore()
	r, authHandler, userID := makeAnalyzeTestRouter(store, "test-secret")
	token, _ := authHandler.generateToken(userID, "admin@localhost")

	wavBytes := makeSineWAVBytes(440.0, 1.0, 44100, 16)
	audioB64 := base64.StdEncoding.EncodeToString(wavBytes)

	body, _ := json.Marshal(map[string]interface{}{
		"audio_data":      audioB64,
		"audio_format":    "wav",
		"reference_notes": []domain.MIDINote{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assessments/analyze", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["score"] != float64(0) {
		t.Fatalf("expected score=0 for empty reference notes, got %v", resp["score"])
	}
	if resp["total_notes"] != float64(0) {
		t.Fatalf("expected total_notes=0, got %v", resp["total_notes"])
	}
}

func TestAnalyzeRecordingWithValidWAV(t *testing.T) {
	store := storage.NewMemoryStore()
	r, authHandler, userID := makeAnalyzeTestRouter(store, "test-secret")
	token, _ := authHandler.generateToken(userID, "admin@localhost")

	// 440 Hz = MIDI 69 (A4)
	freq := 440.0
	sampleRate := 44100
	duration := 3.0
	wavBytes := makeSineWAVBytes(freq, duration, sampleRate, 16)
	audioB64 := base64.StdEncoding.EncodeToString(wavBytes)

	refNotes := []domain.MIDINote{
		{Pitch: 69, Velocity: 100, StartTime: 0.2, EndTime: 2.8},
	}

	body, _ := json.Marshal(map[string]interface{}{
		"audio_data":     audioB64,
		"audio_format":   "wav",
		"reference_notes": refNotes,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assessments/analyze", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp service.AssessmentResultForAssessment
	// NoteComparison is []domain.NoteComparison which has json tags
	respBytes := rec.Body.Bytes()
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("failed to decode response: %v\nraw: %s", err, string(respBytes))
	}

	if resp.TotalNotes != 1 {
		t.Errorf("expected total_notes=1, got %d", resp.TotalNotes)
	}
	if resp.Score < 0 || resp.Score > 100 {
		t.Errorf("score out of range [0,100]: %d", resp.Score)
	}
	if len(resp.PitchDeviation) != resp.TotalNotes {
		t.Errorf("pitch_deviation length mismatch: expected %d, got %d", resp.TotalNotes, len(resp.PitchDeviation))
	}
	if len(resp.DurationDeviation) != resp.TotalNotes {
		t.Errorf("duration_deviation length mismatch: expected %d, got %d", resp.TotalNotes, len(resp.DurationDeviation))
	}
}

func TestAnalyzeRecordingWithFloat32WAV(t *testing.T) {
	store := storage.NewMemoryStore()
	r, authHandler, userID := makeAnalyzeTestRouter(store, "test-secret")
	token, _ := authHandler.generateToken(userID, "admin@localhost")

	freq := 440.0
	sampleRate := 44100
	wavBytes := makeSineWAVBytes(freq, 2.0, sampleRate, 32)
	audioB64 := base64.StdEncoding.EncodeToString(wavBytes)

	refNotes := []domain.MIDINote{
		{Pitch: 69, Velocity: 100, StartTime: 0.2, EndTime: 1.8},
	}

	body, _ := json.Marshal(map[string]interface{}{
		"audio_data":     audioB64,
		"audio_format":   "wav",
		"reference_notes": refNotes,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assessments/analyze", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["total_notes"] != float64(1) {
		t.Errorf("expected total_notes=1, got %v", resp["total_notes"])
	}
}

func TestAnalyzeRecordingDefaultFormat(t *testing.T) {
	store := storage.NewMemoryStore()
	r, authHandler, userID := makeAnalyzeTestRouter(store, "test-secret")
	token, _ := authHandler.generateToken(userID, "admin@localhost")

	wavBytes := makeSineWAVBytes(440.0, 2.0, 44100, 16)
	audioB64 := base64.StdEncoding.EncodeToString(wavBytes)

	refNotes := []domain.MIDINote{
		{Pitch: 69, Velocity: 100, StartTime: 0.2, EndTime: 1.8},
	}

	body, _ := json.Marshal(map[string]interface{}{
		"audio_data":     audioB64,
		"reference_notes": refNotes,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assessments/analyze", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestAnalyzeRecordingRejectsWebM(t *testing.T) {
	store := storage.NewMemoryStore()
	r, authHandler, userID := makeAnalyzeTestRouter(store, "test-secret")
	token, _ := authHandler.generateToken(userID, "admin@localhost")

	body, _ := json.Marshal(map[string]interface{}{
		"audio_data":     base64.StdEncoding.EncodeToString([]byte("fake-webm-data")),
		"audio_format":   "webm",
		"reference_notes": []domain.MIDINote{},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assessments/analyze", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

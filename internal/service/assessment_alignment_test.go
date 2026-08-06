package service

import (
	"math"
	"testing"

	"vocal-practice-app/internal/domain"
)

// TestAssessRecordingWithOffsetAlignment verifies that detected notes (which
// have recording-relative times) are correctly shifted by recordingStartTime
// to align with absolute-timestamp reference notes.
//
// Scenario:
//   - Reference note: MIDI 69 (A4 = 440Hz), absolute time [8.89, 10.55]
//   - Recording: 440Hz sine wave starting at relative time 0.0 (after count-in trim)
//   - recordingStartTime = 8.89 (the absolute time corresponding to recording t=0)
//
// Without the fix, the detected note at t=0.0 would never overlap with the ref
// at 8.89 → all notes marked "missed".
// With the fix, the detected note is shifted to 8.89 → correct alignment.
func TestAssessRecordingWithOffsetAlignment(t *testing.T) {
	sampleRate := 44100
	freq := 440.0 // A4 = MIDI 69

	// Create 2 seconds of pure A4 sine wave (simulates trimmed recording)
	duration := 2.0
	n := int(float64(sampleRate) * duration)
	samples := make([]float64, n)
	for i := 0; i < n; i++ {
		samples[i] = math.Sin(2 * math.Pi * freq * float64(i) / float64(sampleRate))
	}

	// Reference note at absolute MIDI time 8.89-10.55
	refNotes := []domain.MIDINote{
		{Pitch: 69, Velocity: 100, StartTime: 8.89, EndTime: 10.55},
	}

	// Test with offset alignment (recordingStartTime = 8.89)
	result, err := AssessRecordingWithVowelFiltering(samples, sampleRate, refNotes, 8.89)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("With offset=8.89: Score=%d, TotalNotes=%d, MatchedNotes=%d",
		result.Score, result.TotalNotes, result.MatchedNotes)

	if result.TotalNotes != 1 {
		t.Errorf("expected total_notes=1, got %d", result.TotalNotes)
	}
	if result.MatchedNotes != 1 {
		t.Errorf("expected matched_notes=1 (alignment should work), got %d", result.MatchedNotes)
	}
	if result.Score == 0 {
		t.Errorf("expected non-zero score (notes should match after alignment), got 0")
	}

	// Print note comparison for debugging
	for i, nc := range result.NoteComparison {
		t.Logf("  NoteComparison[%d]: RefStart=%.3f, RefEnd=%.3f, UserStart=%.3f, UserEnd=%.3f, Status=%s, UserPitch=%d",
			i, nc.RefStart, nc.RefEnd, nc.UserStart, nc.UserEnd, nc.MatchStatus, nc.UserPitch)
	}
}

// TestAssessRecordingWithoutOffset verifies the bug: without offset, notes don't align.
func TestAssessRecordingWithoutOffset(t *testing.T) {
	sampleRate := 44100
	freq := 440.0

	duration := 2.0
	n := int(float64(sampleRate) * duration)
	samples := make([]float64, n)
	for i := 0; i < n; i++ {
		samples[i] = math.Sin(2 * math.Pi * freq * float64(i) / float64(sampleRate))
	}

	// Reference note at absolute time 8.89-10.55
	refNotes := []domain.MIDINote{
		{Pitch: 69, Velocity: 100, StartTime: 8.89, EndTime: 10.55},
	}

	// Test WITHOUT offset alignment (recordingStartTime = 0)
	result, err := AssessRecordingWithVowelFiltering(samples, sampleRate, refNotes, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Without offset (0.0): Score=%d, TotalNotes=%d, MatchedNotes=%d",
		result.Score, result.TotalNotes, result.MatchedNotes)

	// Without alignment, detected note at t=0 should NOT match ref at t=8.89
	if result.MatchedNotes != 0 {
		t.Errorf("expected 0 matched notes without alignment, got %d", result.MatchedNotes)
	}

	for i, nc := range result.NoteComparison {
		t.Logf("  NoteComparison[%d]: RefStart=%.3f, RefEnd=%.3f, UserStart=%.3f, UserEnd=%.3f, Status=%s",
			i, nc.RefStart, nc.RefEnd, nc.UserStart, nc.UserEnd, nc.MatchStatus)
	}
}

// TestAssessRecordingPitchDeviation verifies that sub-semitone pitch deviations
// are captured. When the user sings slightly sharp (e.g., A4 + 10 cents),
// the pitch deviation should be non-zero, not rounded to 0.
func TestAssessRecordingPitchDeviation(t *testing.T) {
	sampleRate := 44100
	// A4 = 440Hz. To generate 10 cents sharp: 440 * 2^(10/1200) ≈ 442.64 Hz
	// MIDI note for 442.64 Hz: 69 + 10/100 = 69.1
	freq := 440.0 * math.Pow(2, 10.0/1200.0) // ~442.64 Hz, 10 cents sharp

	duration := 2.0
	n := int(float64(sampleRate) * duration)
	samples := make([]float64, n)
	for i := 0; i < n; i++ {
		samples[i] = math.Sin(2 * math.Pi * freq * float64(i) / float64(sampleRate))
	}

	// Reference note at absolute MIDI time 8.89-10.55, pitch 69 (A4)
	refNotes := []domain.MIDINote{
		{Pitch: 69, Velocity: 100, StartTime: 8.89, EndTime: 10.55},
	}

	// Test with offset alignment (recordingStartTime = 8.89)
	result, err := AssessRecordingWithVowelFiltering(samples, sampleRate, refNotes, 8.89)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("With 10-cent sharp: Score=%d, MatchedNotes=%d, AvgPitchDev=%.1f, PitchDeviation=%v",
		result.Score, result.MatchedNotes, result.AveragePitchDeviation, result.PitchDeviation)

	if result.MatchedNotes != 1 {
		t.Errorf("expected matched_notes=1, got %d", result.MatchedNotes)
	}

	// The pitch deviation should be approximately 10 cents (not 0)
	if len(result.PitchDeviation) > 0 {
		dev := result.PitchDeviation[0]
		t.Logf("Pitch deviation for matched note: %.2f cents", dev)
		if math.Abs(dev) < 5.0 {
			t.Errorf("expected pitch deviation near 10 cents (10-cent sharp), got %.2f", dev)
		}
	}

	if result.AveragePitchDeviation == 0 {
		t.Errorf("expected non-zero average pitch deviation for sharp note, got 0")
	}
}

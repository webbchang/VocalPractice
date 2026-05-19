package service

import (
	"math"
	"testing"
)

// ============================================================
// Tests for mergeSamePitchNotes
// ============================================================

func TestMergeSamePitchNotes_Empty(t *testing.T) {
	result := mergeSamePitchNotes(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}

	result = mergeSamePitchNotes([]MIDINoteForTest{})
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestMergeSamePitchNotes_SingleNote(t *testing.T) {
	notes := []MIDINoteForTest{
		{Pitch: 60, StartTime: 1.0, EndTime: 2.0},
	}
	result := mergeSamePitchNotes(notes)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged note, got %d", len(result))
	}
	if result[0].Pitch != 60 || result[0].StartTime != 1.0 || result[0].EndTime != 2.0 {
		t.Errorf("unexpected merged note: %+v", result[0])
	}
	if len(result[0].EventIdx) != 1 || result[0].EventIdx[0] != 0 {
		t.Errorf("unexpected EventIdx: %v", result[0].EventIdx)
	}
}

func TestMergeSamePitchNotes_ConsecutiveSamePitch(t *testing.T) {
	notes := []MIDINoteForTest{
		{Pitch: 60, StartTime: 1.0, EndTime: 1.5},
		{Pitch: 60, StartTime: 1.52, EndTime: 2.0}, // gap 0.02 < 0.05
	}
	result := mergeSamePitchNotes(notes)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged note, got %d", len(result))
	}
	if result[0].Pitch != 60 || result[0].StartTime != 1.0 || result[0].EndTime != 2.0 {
		t.Errorf("unexpected merged: start=%.2f end=%.2f", result[0].StartTime, result[0].EndTime)
	}
	if len(result[0].EventIdx) != 2 {
		t.Errorf("expected 2 event idx, got %d", len(result[0].EventIdx))
	}
}

func TestMergeSamePitchNotes_GapTooLarge(t *testing.T) {
	notes := []MIDINoteForTest{
		{Pitch: 60, StartTime: 1.0, EndTime: 1.5},
		{Pitch: 60, StartTime: 2.0, EndTime: 2.5}, // gap 0.5 > 0.05
	}
	result := mergeSamePitchNotes(notes)
	if len(result) != 2 {
		t.Fatalf("expected 2 merged notes, got %d", len(result))
	}
	if result[0].Pitch != 60 || result[1].Pitch != 60 {
		t.Errorf("both should be pitch 60")
	}
}

func TestMergeSamePitchNotes_DifferentPitch(t *testing.T) {
	notes := []MIDINoteForTest{
		{Pitch: 60, StartTime: 1.0, EndTime: 1.5},
		{Pitch: 62, StartTime: 1.52, EndTime: 2.0}, // different pitch, gap small but not same pitch
	}
	result := mergeSamePitchNotes(notes)
	if len(result) != 2 {
		t.Fatalf("expected 2 merged notes for different pitches, got %d", len(result))
	}
	if result[0].Pitch != 60 || result[1].Pitch != 62 {
		t.Errorf("pitches should be 60 and 62, got %d and %d", result[0].Pitch, result[1].Pitch)
	}
}

func TestMergeSamePitchNotes_PreservesIndices(t *testing.T) {
	notes := []MIDINoteForTest{
		{Pitch: 60, StartTime: 3.0, EndTime: 3.5},  // index 0
		{Pitch: 62, StartTime: 1.0, EndTime: 1.5},  // index 1 — will be sorted first
		{Pitch: 60, StartTime: 3.52, EndTime: 4.0}, // index 2
	}
	result := mergeSamePitchNotes(notes)
	// After sorting: [1] 62@1.0, [0] 60@3.0, [2] 60@3.52
	// Merge: 62 alone, 60+60 merged
	if len(result) != 2 {
		t.Fatalf("expected 2 merged, got %d", len(result))
	}
	// First merged: pitch 62, eventIdx should point to original index 1
	if result[0].Pitch != 62 {
		t.Errorf("first merged should be pitch 62")
	}
	if len(result[0].EventIdx) != 1 || result[0].EventIdx[0] != 1 {
		t.Errorf("expected EventIdx [1], got %v", result[0].EventIdx)
	}
	// Second merged: pitch 60, eventIdx should point to original index 0 and 2
	if result[1].Pitch != 60 {
		t.Errorf("second merged should be pitch 60")
	}
	if len(result[1].EventIdx) != 2 {
		t.Fatalf("expected 2 event idx for merged 60s, got %d", len(result[1].EventIdx))
	}
	if result[1].EventIdx[0] != 0 || result[1].EventIdx[1] != 2 {
		t.Errorf("expected EventIdx [0,2], got %v", result[1].EventIdx)
	}
}

// ============================================================
// Tests for compareMergedNotes
// ============================================================

func TestCompareMergedNotes_EmptyDetected_ScoreZero(t *testing.T) {
	refs := []MIDINoteForTest{
		{Pitch: 60, StartTime: 1.0, EndTime: 1.5},
		{Pitch: 62, StartTime: 2.0, EndTime: 2.5},
	}
	merged := mergeSamePitchNotes(refs)
	detected := []MIDINoteForTest{} // empty

	result := compareMergedNotes(refs, merged, detected)
	if result.Score != 0 {
		t.Errorf("expected score 0 when no detected notes, got %d", result.Score)
	}
	if result.MatchedNotes != 0 {
		t.Errorf("expected 0 matched notes, got %d", result.MatchedNotes)
	}
	if result.TotalNotes != 2 {
		t.Errorf("expected total notes 2, got %d", result.TotalNotes)
	}
	if len(result.NoteComparison) != 2 {
		t.Errorf("expected 2 comparison entries, got %d", len(result.NoteComparison))
	}
	for i, nc := range result.NoteComparison {
		if nc.MatchStatus != "missed" {
			t.Errorf("entry %d should have status 'missed', got '%s'", i, nc.MatchStatus)
		}
	}
}

func TestCompareMergedNotes_PerfectMatch(t *testing.T) {
	refs := []MIDINoteForTest{
		{Pitch: 60, StartTime: 1.0, EndTime: 2.0},
		{Pitch: 62, StartTime: 3.0, EndTime: 4.0},
	}
	merged := mergeSamePitchNotes(refs)
	detected := []MIDINoteForTest{
		{Pitch: 60, StartTime: 1.0, EndTime: 2.0},
		{Pitch: 62, StartTime: 3.0, EndTime: 4.0},
	}

	result := compareMergedNotes(refs, merged, detected)
	if result.Score != 100 {
		t.Errorf("expected score 100 for perfect match, got %d", result.Score)
	}
	if result.MatchedNotes != 2 {
		t.Errorf("expected 2 matched notes, got %d", result.MatchedNotes)
	}
	if result.AveragePitchDeviation != 0 {
		t.Errorf("expected 0 pitch deviation, got %.2f", result.AveragePitchDeviation)
	}
	if result.AverageDurationDeviation != 0 {
		t.Errorf("expected 0 duration deviation, got %.3f", result.AverageDurationDeviation)
	}
}

func TestCompareMergedNotes_PartialMatch(t *testing.T) {
	refs := []MIDINoteForTest{
		{Pitch: 60, StartTime: 1.0, EndTime: 2.0},
		{Pitch: 62, StartTime: 3.0, EndTime: 4.0},
	}
	merged := mergeSamePitchNotes(refs)
	detected := []MIDINoteForTest{
		{Pitch: 60, StartTime: 1.0, EndTime: 2.0}, // match
		// second ref has no match
	}

	result := compareMergedNotes(refs, merged, detected)
	if result.MatchedNotes != 1 {
		t.Errorf("expected 1 matched note, got %d", result.MatchedNotes)
	}
	if result.TotalNotes != 2 {
		t.Errorf("expected total notes 2, got %d", result.TotalNotes)
	}
	// Score reflects quality of matched notes only (100 for perfect 1/1 match)
	// Unmatched refs do not penalize the score
	if result.Score != 100 {
		t.Errorf("expected score 100 (perfect single match), got %d", result.Score)
	}
	// first ref matched, second missed
	if result.NoteComparison[0].MatchStatus != "matched" {
		t.Errorf("first note should be matched, got '%s'", result.NoteComparison[0].MatchStatus)
	}
	if result.NoteComparison[1].MatchStatus != "missed" {
		t.Errorf("second note should be missed, got '%s'", result.NoteComparison[1].MatchStatus)
	}
}

func TestCompareMergedNotes_OverlapThreshold(t *testing.T) {
	// Overlap ratio < 0.8 should not match
	refs := []MIDINoteForTest{
		{Pitch: 60, StartTime: 1.0, EndTime: 2.0},
	}
	merged := mergeSamePitchNotes(refs)
	detected := []MIDINoteForTest{
		{Pitch: 60, StartTime: 1.0, EndTime: 1.15}, // overlap 0.15 / min(1.0, 0.15) = 1.0 — actually this is 100% of detected
	}
	result := compareMergedNotes(refs, merged, detected)
	// overlapLen = 0.15, mgLen = 1.0, detLen = 0.15, shorterLen = 0.15, ratio = 0.15/0.15 = 1.0 >= 0.8 => should match
	if result.MatchedNotes != 1 {
		t.Errorf("expected 1 matched note (detected note fully inside ref), got %d", result.MatchedNotes)
	}
}

func TestCompareMergedNotes_WrongPitchNoMatch(t *testing.T) {
	refs := []MIDINoteForTest{
		{Pitch: 60, StartTime: 1.0, EndTime: 2.0},
	}
	merged := mergeSamePitchNotes(refs)
	detected := []MIDINoteForTest{
		{Pitch: 72, StartTime: 1.0, EndTime: 2.0}, // different pitch
	}
	result := compareMergedNotes(refs, merged, detected)
	if result.MatchedNotes != 0 {
		t.Errorf("expected 0 matched notes (wrong pitch), got %d", result.MatchedNotes)
	}
	if result.Score != 0 {
		t.Errorf("expected score 0 when no match, got %d", result.Score)
	}
}

func TestCompareMergedNotes_PitchMismatchNoMatch(t *testing.T) {
	// Algorithm requires exact pitch matching before computing deviation
	// Different pitch => no match, score=0
	refs := []MIDINoteForTest{
		{Pitch: 60, StartTime: 1.0, EndTime: 2.0},
	}
	merged := mergeSamePitchNotes(refs)
	detected := []MIDINoteForTest{
		{Pitch: 61, StartTime: 1.0, EndTime: 2.0}, // different pitch
	}
	result := compareMergedNotes(refs, merged, detected)
	if result.MatchedNotes != 0 {
		t.Errorf("expected 0 matched notes (different pitch), got %d", result.MatchedNotes)
	}
	if result.Score != 0 {
		t.Errorf("expected score 0 for pitch mismatch, got %d", result.Score)
	}
}

// ============================================================
// Tests for compareNotesGo (legacy comparison)
// ============================================================

func TestCompareNotesGo_EmptyDetected_ScoreZero(t *testing.T) {
	refs := []MIDINoteForTest{
		{Pitch: 60, StartTime: 1.0, EndTime: 1.5},
	}
	detected := []MIDINoteForTest{}

	result := compareNotesGo(refs, detected)
	if result.Score != 0 {
		t.Errorf("expected score 0 when no detected notes, got %d", result.Score)
	}
	if result.MatchedNotes != 0 {
		t.Errorf("expected 0 matched notes, got %d", result.MatchedNotes)
	}
}

func TestCompareNotesGo_PerfectMatch(t *testing.T) {
	refs := []MIDINoteForTest{
		{Pitch: 60, StartTime: 1.0, EndTime: 2.0},
	}
	detected := []MIDINoteForTest{
		{Pitch: 60, StartTime: 1.0, EndTime: 2.0},
	}
	result := compareNotesGo(refs, detected)
	if result.Score != 100 {
		t.Errorf("expected score 100, got %d", result.Score)
	}
	if result.MatchedNotes != 1 {
		t.Errorf("expected 1 matched, got %d", result.MatchedNotes)
	}
}

func TestCompareNotesGo_OutOfTolerance(t *testing.T) {
	refs := []MIDINoteForTest{
		{Pitch: 60, StartTime: 1.0, EndTime: 2.0},
	}
	detected := []MIDINoteForTest{
		{Pitch: 60, StartTime: 2.0, EndTime: 3.0}, // start time diff = 1.0 > 0.5 tolerance
	}
	result := compareNotesGo(refs, detected)
	if result.MatchedNotes != 0 {
		t.Errorf("expected 0 matched (out of tolerance), got %d", result.MatchedNotes)
	}
	if result.Score != 0 {
		t.Errorf("expected score 0, got %d", result.Score)
	}
}

// ============================================================
// Tests for detectPitchGo
// ============================================================

func TestDetectPitchGo_Silence(t *testing.T) {
	sampleRate := 44100
	samples := make([]float64, sampleRate*2) // 2 seconds of silence
	notes := detectPitchGo(samples, sampleRate)
	if len(notes) != 0 {
		t.Errorf("expected 0 notes from silence, got %d", len(notes))
	}
}

func TestDetectPitchGo_SineWave(t *testing.T) {
	sampleRate := 44100
	freq := 440.0 // A4
	duration := 1.0
	n := int(float64(sampleRate) * duration)
	samples := make([]float64, n)
	for i := 0; i < n; i++ {
		samples[i] = math.Sin(2 * math.Pi * freq * float64(i) / float64(sampleRate))
	}

	notes := detectPitchGo(samples, sampleRate)
	// Should detect some notes around pitch 69 (A4 = MIDI 69)
	if len(notes) == 0 {
		t.Fatal("expected at least 1 note from sine wave, got 0")
	}
	for i, n := range notes {
		if n.Pitch < 65 || n.Pitch > 73 {
			t.Errorf("note %d: pitch %d out of expected range around 69 (A4=440Hz)", i, n.Pitch)
		}
	}
}

func TestDetectPitchGo_MultipleSineWaves(t *testing.T) {
	sampleRate := 44100
	duration := 2.0
	n := int(float64(sampleRate) * duration)
	samples := make([]float64, n)

	// A4 (440Hz) for first second, then C5 (523.25Hz) for second second
	halfN := n / 2
	for i := 0; i < halfN; i++ {
		samples[i] = math.Sin(2 * math.Pi * 440.0 * float64(i) / float64(sampleRate))
	}
	for i := halfN; i < n; i++ {
		samples[i] = math.Sin(2 * math.Pi * 523.25 * float64(i) / float64(sampleRate))
	}

	notes := detectPitchGo(samples, sampleRate)
	if len(notes) == 0 {
		t.Fatal("expected at least 1 note, got 0")
	}

	// Should detect at least one note near A4 (69) and one near C5 (72)
	hasA4 := false
	hasC5 := false
	for _, n := range notes {
		if n.Pitch >= 67 && n.Pitch <= 71 {
			hasA4 = true
		}
		if n.Pitch >= 70 && n.Pitch <= 74 {
			hasC5 = true
		}
	}
	if !hasA4 && !hasC5 {
		t.Errorf("expected notes near A4(69) or C5(72), got pitches: ")
		for _, n := range notes {
			t.Errorf("  pitch=%d start=%.2f end=%.2f", n.Pitch, n.StartTime, n.EndTime)
		}
	}
}

func TestDetectPitchGo_FilterShortNotes(t *testing.T) {
	sampleRate := 44100
	// Generate a quick burst that's shorter than 0.1s filter threshold
	n := int(float64(sampleRate) * 0.05) // 50ms
	samples := make([]float64, n)
	for i := 0; i < n; i++ {
		samples[i] = math.Sin(2 * math.Pi * 440.0 * float64(i) / float64(sampleRate))
	}
	notes := detectPitchGo(samples, sampleRate)
	// Notes shorter than 0.1s should be filtered out
	if len(notes) != 0 {
		t.Errorf("expected 0 notes (all should be filtered as too short), got %d", len(notes))
	}
}

// ============================================================
// Tests for min helper
// ============================================================

func TestMin(t *testing.T) {
	if min(1, 2) != 1 {
		t.Error("min(1,2) should be 1")
	}
	if min(2, 1) != 1 {
		t.Error("min(2,1) should be 1")
	}
	if min(5, 5) != 5 {
		t.Error("min(5,5) should be 5")
	}
	if min(-1, 1) != -1 {
		t.Error("min(-1,1) should be -1")
	}
}

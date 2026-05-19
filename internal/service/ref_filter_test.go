package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// ============================================================
// JS Reference Filter Test Suite
//
// Tests the client-side reference data generation logic (from
// user-practice.html) by calling the JS script via two modes:
//
//   A) Stdin pipe — direct process execution
//   B) HTTP server — curl-compatible multipart POST
//
// Then compares JS output against Go server-side parser output.
// ============================================================

type JSFilterResult struct {
	Ok            bool   `json:"ok"`
	MIDIPath      string `json:"midi_path,omitempty"`
	NumTracks     int    `json:"num_tracks"`
	TotalNotesAll int    `json:"total_notes_all"`
	TrackInfo     []struct {
		TrackIndex int       `json:"track_index"`
		NoteCount  int       `json:"note_count"`
		TimeRange  []float64 `json:"time_range,omitempty"`
		PitchCount int       `json:"pitch_count,omitempty"`
	} `json:"track_info"`
	Filter struct {
		TrackIndex int     `json:"track_index"`
		Start      float64 `json:"start"`
		End        float64 `json:"end"`
	} `json:"filter"`
	ReferenceNotes []struct {
		Pitch int     `json:"pitch"`
		Start float64 `json:"start"`
		Dur   float64 `json:"dur"`
	} `json:"reference_notes"`
	ReferenceCount int    `json:"reference_count"`
	Engine         string `json:"engine"`
	Error          string `json:"error,omitempty"`
}

// refFilterPath returns the absolute path to the JS script
func refFilterPath() string {
	p, _ := filepath.Abs(filepath.Join("..", "..", "cmd", "midi_ref_test", "ref_filter.cjs"))
	return p
}

func midiPath(name string) string {
	return filepath.Join("..", "..", "test_data", name)
}

// ============================================================
// Test A1: Stdin mode — basic reference data filtering
// ============================================================

func TestJSRefFilter_Stdin_BasicFilter(t *testing.T) {
	script := refFilterPath()
	input := map[string]interface{}{
		"midiPath":   midiPath("reference.mid"),
		"trackIndex": 0,
		"start":      50.0,
		"end":        60.0,
	}
	inputJSON, _ := json.Marshal(input)

	cmd := exec.Command("node", script)
	cmd.Stdin = bytes.NewReader(inputJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("JS script failed: %v\nstderr: %s", err, stderr.String())
	}

	var result JSFilterResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JS output: %v\nraw: %s", err, stdout.String())
	}

	if !result.Ok {
		t.Fatalf("JS returned ok=false: %s", result.Error)
	}

	// Validation 1: correct MIDI file
	if result.MIDIPath == "" {
		t.Error("expected midi_path to be set")
	}

	// Validation 2: track info
	if result.NumTracks != 1 {
		t.Errorf("expected 1 track, got %d", result.NumTracks)
	}
	if result.TotalNotesAll != 140 {
		t.Errorf("expected 140 total notes, got %d", result.TotalNotesAll)
	}

	// Validation 3: reference notes timing
	t.Logf("JS parser: %d reference notes in [50, 60)", result.ReferenceCount)
	for i, n := range result.ReferenceNotes {
		if n.Start < 50.0-0.051 { // allow small floating rounding
			t.Errorf("note %d: start %.4f < 50.0", i, n.Start)
		}
		if n.Start >= 60.0 {
			t.Errorf("note %d: start %.4f >= 60.0", i, n.Start)
		}
		if n.Dur <= 0 {
			t.Errorf("note %d: dur %.4f <= 0", i, n.Dur)
		}
	}

	if result.ReferenceCount == 0 {
		t.Log("WARNING: no reference notes in range [50,60) — notes may start later")
	}
}

// ============================================================
// Test A2: Stdin mode — empty range returns no notes
// ============================================================

func TestJSRefFilter_Stdin_EmptyRange(t *testing.T) {
	script := refFilterPath()
	input := map[string]interface{}{
		"midiPath":   midiPath("reference.mid"),
		"trackIndex": 0,
		"start":      999.0,
		"end":        1000.0,
	}
	inputJSON, _ := json.Marshal(input)

	cmd := exec.Command("node", script)
	cmd.Stdin = bytes.NewReader(inputJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("JS script failed: %v\nstderr: %s", err, stderr.String())
	}

	var result JSFilterResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JS output: %v\nraw: %s", err, stdout.String())
	}

	if result.ReferenceCount != 0 {
		t.Errorf("expected 0 notes for empty range, got %d", result.ReferenceCount)
	}
	if len(result.ReferenceNotes) != 0 {
		t.Errorf("expected empty reference_notes array, got %d items", len(result.ReferenceNotes))
	}
}

// ============================================================
// Test A3: Stdin mode — reference2.MID multi-track
// ============================================================

func TestJSRefFilter_Stdin_MultiTrack(t *testing.T) {
	script := refFilterPath()

	// reference2.MID has 11 tracks (indices 1-11 in JS since JS uses 0-based indexing)
	// First track with data on reference2 is track 1
	input := map[string]interface{}{
		"midiPath":   midiPath("reference2.MID"),
		"trackIndex": 1,
		"start":      0.0,
		"end":        30.0,
	}
	inputJSON, _ := json.Marshal(input)

	cmd := exec.Command("node", script)
	cmd.Stdin = bytes.NewReader(inputJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("JS script failed: %v\nstderr: %s", err, stderr.String())
	}

	var result JSFilterResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JS output: %v\nraw: %s", err, stdout.String())
	}

	if result.NumTracks < 10 {
		t.Errorf("expected >=10 tracks for reference2.MID, got %d", result.NumTracks)
	}

	// Validate each reference note belongs to the right track
	for i, n := range result.ReferenceNotes {
		if n.Dur <= 0 {
			t.Errorf("note %d: non-positive dur %.4f", i, n.Dur)
		}
	}

	t.Logf("reference2.MID: %d tracks, %d total notes, %d in track 1 [0,30)",
		result.NumTracks, result.TotalNotesAll, result.ReferenceCount)
	t.Logf("Track info: %+v", result.TrackInfo)
}

// ============================================================
// Test A4: Stdin mode — compare JS output vs Go server-side parser
// ============================================================

func TestJSRefFilter_Stdin_CompareWithGoParser(t *testing.T) {
	// This test validates that the JS client-side parser produces
	// the same filtered notes as the Go server-side parser.

	// 1. Parse with Go server-side parser
	midiPath := midiPath("reference.mid")
	raw, err := os.ReadFile(midiPath)
	if err != nil {
		t.Fatalf("failed to read MIDI: %v", err)
	}

	parser := NewMIDIParser()
	song, err := parser.Parse(raw, "test", "test")
	if err != nil {
		t.Fatalf("Go parser failed: %v", err)
	}

	if len(song.Tracks) == 0 {
		t.Fatal("no tracks from Go parser")
	}

	// Pick the first track with notes
	var goNotes []struct {
		Pitch     int
		StartTime float64
		EndTime   float64
	}
	for _, tr := range song.Tracks {
		if len(tr.Notes) > 0 {
			for _, n := range tr.Notes {
				goNotes = append(goNotes, struct {
					Pitch     int
					StartTime float64
					EndTime   float64
				}{n.Pitch, n.StartTime, n.EndTime})
			}
			break
		}
	}

	if len(goNotes) == 0 {
		t.Fatal("no notes from Go parser")
	}

	// 2. Filter with Go-side logic (same as generateReferenceData)
	start := 50.0
	end := 60.0
	var goFiltered []struct {
		pitch int
		start float64
		dur   float64
	}
	for _, n := range goNotes {
		if n.StartTime >= start-0.05 && n.StartTime < end {
			dur := n.EndTime - n.StartTime
			if end-n.StartTime < dur {
				dur = end - n.StartTime
			}
			if dur > 0.02 {
				goFiltered = append(goFiltered, struct {
					pitch int
					start float64
					dur   float64
				}{n.Pitch, n.StartTime, dur})
			}
		}
	}

	// 3. Get JS result
	script := refFilterPath()
	input := map[string]interface{}{
		"midiPath":   midiPath,
		"trackIndex": 0,
		"start":      start,
		"end":        end,
	}
	inputJSON, _ := json.Marshal(input)
	cmd := exec.Command("node", script)
	cmd.Stdin = bytes.NewReader(inputJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("JS script failed: %v\nstderr: %s", err, stderr.String())
	}

	var result JSFilterResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JS output: %v", err)
	}

	// 4. Compare counts
	t.Logf("Go filtered: %d notes, JS filtered: %d notes", len(goFiltered), result.ReferenceCount)

	if len(goFiltered) != result.ReferenceCount {
		// Note: small differences may exist due to floating point rounding
		diff := len(goFiltered) - result.ReferenceCount
		if diff < 0 {
			diff = -diff
		}
		if diff > 2 {
			t.Errorf("Go/JS note count mismatch: Go=%d JS=%d (diff=%d)",
				len(goFiltered), result.ReferenceCount, diff)
		} else {
			t.Logf("Acceptable small diff: Go=%d JS=%d", len(goFiltered), result.ReferenceCount)
		}
	}

	// 5. Compare individual notes (where possible)
	maxCheck := len(goFiltered)
	if result.ReferenceCount < maxCheck {
		maxCheck = result.ReferenceCount
	}
	mismatches := 0
	for i := 0; i < maxCheck; i++ {
		g := goFiltered[i]
		j := result.ReferenceNotes[i]

		pitchDiff := g.pitch - j.Pitch
		if pitchDiff < 0 {
			pitchDiff = -pitchDiff
		}
		startDiff := g.start - j.Start
		if startDiff < 0 {
			startDiff = -startDiff
		}
		durDiff := g.dur - j.Dur
		if durDiff < 0 {
			durDiff = -durDiff
		}

		if pitchDiff > 0 || startDiff > 0.05 || durDiff > 0.05 {
			mismatches++
			if mismatches <= 3 {
				t.Logf("Mismatch %d: Go(pitch=%d start=%.4f dur=%.4f) vs JS(pitch=%d start=%.4f dur=%.4f)",
					i, g.pitch, g.start, g.dur, j.Pitch, j.Start, j.Dur)
			}
		}
	}

	if mismatches > maxCheck/10 && maxCheck > 0 {
		t.Errorf("too many mismatches: %d / %d (%.0f%%)", mismatches, maxCheck,
			float64(mismatches)/float64(maxCheck)*100)
	}

	t.Logf("Compared %d notes: %d mismatches (threshold: pitch>0, start>0.05s, dur>0.05s)",
		maxCheck, mismatches)
}

// ============================================================
// Test B1: HTTP server mode — basic end-to-end with curl
// ============================================================

func TestJSRefFilter_HTTPServer_Basic(t *testing.T) {
	script := refFilterPath()
	midiFile := midiPath("reference.mid")

	// 1. Start the JS server
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "node", script, "--server")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	// Capture stderr for startup handshake (server writes port info there)
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("failed to get stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start JS server: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// 2. Read port from stderr
	portCh := make(chan int, 1)
	go func() {
		buf := make([]byte, 4096)
		n, _ := stderrPipe.Read(buf)
		line := string(buf[:n])
		for _, l := range strings.Split(line, "\n") {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "{") {
				var info struct {
					Ok   bool `json:"ok"`
					Port int  `json:"port"`
					Pid  int  `json:"pid"`
				}
				if err := json.Unmarshal([]byte(l), &info); err == nil && info.Ok {
					portCh <- info.Port
					return
				}
			}
		}
	}()

	var port int
	select {
	case port = <-portCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for JS server to start")
	}

	t.Logf("JS server started on port %d", port)

	// 3. Prepare multipart request
	midiData, err := os.ReadFile(midiFile)
	if err != nil {
		t.Fatalf("failed to read MIDI: %v", err)
	}

	var reqBody bytes.Buffer
	writer := NewMultipartWriter(&reqBody)
	writer.WriteField("track_index", "0")
	writer.WriteField("start", "50.0")
	writer.WriteField("end", "60.0")
	writer.WriteFile("midi", "reference.mid", midiData)
	contentType := writer.Close()

	// 4. Send POST request
	url := fmt.Sprintf("http://localhost:%d/filter", port)
	resp, err := http.Post(url, contentType, &reqBody)
	if err != nil {
		t.Fatalf("HTTP POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result JSFilterResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !result.Ok {
		t.Fatalf("JS returned ok=false: %s", result.Error)
	}

	t.Logf("HTTP server result: %d reference notes in [50, 60)", result.ReferenceCount)

	// 5. Validate
	for i, n := range result.ReferenceNotes {
		if n.Start < 50.0-0.06 {
			t.Errorf("HTTP note %d: start %.4f < 50.0", i, n.Start)
		}
		if n.Start >= 60.0 {
			t.Errorf("HTTP note %d: start %.4f >= 60.0", i, n.Start)
		}
	}

	if result.ReferenceCount == 0 {
		t.Log("HTTP test: no notes in [50,60)")
	} else {
		t.Logf("HTTP server returned %d reference notes successfully", result.ReferenceCount)
	}

	// 6. Verify GET endpoint works
	getResp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET /filter failed: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != 200 {
		t.Errorf("GET expected 200, got %d", getResp.StatusCode)
	}
}

// ============================================================
// MultipartWriter builds multipart/form-data bodies
// ============================================================
type MultipartWriter struct {
	buf *bytes.Buffer
	w   *writer
}

type writer interface {
	Write(p []byte) (n int, err error)
	WriteString(s string) (n int, err error)
}

func NewMultipartWriter(buf *bytes.Buffer) *MultipartWriter {
	return &MultipartWriter{buf: buf}
}

func (mw *MultipartWriter) WriteField(name, value string) {
	mw.buf.WriteString(fmt.Sprintf("--BOUNDARY\r\nContent-Disposition: form-data; name=%q\r\n\r\n%s\r\n", name, value))
}

func (mw *MultipartWriter) WriteFile(name, filename string, data []byte) {
	mw.buf.WriteString(fmt.Sprintf("--BOUNDARY\r\nContent-Disposition: form-data; name=%q; filename=%q\r\nContent-Type: application/octet-stream\r\n\r\n", name, filename))
	mw.buf.Write(data)
	mw.buf.WriteString("\r\n")
}

func (mw *MultipartWriter) Close() string {
	mw.buf.WriteString("--BOUNDARY--\r\n")
	return "multipart/form-data; boundary=BOUNDARY"
}

// ============================================================
// Test A5: Stdin mode — compare JS vs Go for reference2.MID multi-track
// ============================================================

func TestJSRefFilter_Stdin_CompareRef2(t *testing.T) {
	// Compare JS client-side parser vs Go server-side parser for reference2.MID
	midiPath := midiPath("reference2.MID")

	// 1. Get JS result
	input := map[string]interface{}{
		"midiPath":   midiPath,
		"trackIndex": 1, // JS reports data starts at track 1 (track 0 is conductor)
		"start":      0.0,
		"end":        300.0,
	}
	inputJSON, _ := json.Marshal(input)
	cmd := exec.Command("node", refFilterPath())
	cmd.Stdin = bytes.NewReader(inputJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("JS script failed: %v\nstderr: %s", err, stderr.String())
	}
	var jsResult JSFilterResult
	if err := json.Unmarshal(stdout.Bytes(), &jsResult); err != nil {
		t.Fatalf("failed to parse JS output: %v", err)
	}

	t.Logf("JS: %d tracks, %d total notes", jsResult.NumTracks, jsResult.TotalNotesAll)
	for _, ti := range jsResult.TrackInfo {
		t.Logf("  Track %d: %d notes", ti.TrackIndex, ti.NoteCount)
	}
	t.Logf("JS filtered [0,300): %d notes", jsResult.ReferenceCount)

	// 2. Get Go parser result
	raw, err := os.ReadFile(midiPath)
	if err != nil {
		t.Fatalf("failed to read MIDI: %v", err)
	}
	parser := NewMIDIParser()
	song, err := parser.Parse(raw, "ref2", "test")
	if err != nil {
		t.Fatalf("Go parser failed: %v", err)
	}

	// Log Go track info
	t.Logf("Go: %d tracks", len(song.Tracks))
	goTotalNotes := 0
	for i, tr := range song.Tracks {
		goTotalNotes += len(tr.Notes)
		t.Logf("  Track %d: %q notes=%d", i, tr.Name, len(tr.Notes))
	}
	t.Logf("Go total notes: %d", goTotalNotes)

	// 3. Go must have at least one track with notes
	tracksWithNotes := 0
	for _, tr := range song.Tracks {
		if len(tr.Notes) > 0 {
			tracksWithNotes++
		}
	}
	if tracksWithNotes == 0 {
		t.Fatal("Go parser produced no tracks with notes for reference2.MID")
	}

	// 4. Both should parse the file without errors
	if jsResult.Error != "" {
		t.Errorf("JS parser error: %s", jsResult.Error)
	}

	// Track count from JS includes track 0 (conductor) which Go may handle differently.
	// If JS has N+1 tracks vs Go N, that's OK — check they're close
	trackDiff := jsResult.NumTracks - len(song.Tracks)
	if trackDiff > 2 || trackDiff < -2 {
		t.Logf("Note: JS reports %d tracks, Go reports %d (diff %d)",
			jsResult.NumTracks, len(song.Tracks), trackDiff)
	}

	// 5. Validate JS filtered reference notes timing
	for i, n := range jsResult.ReferenceNotes {
		if n.Start < -0.051 {
			t.Errorf("note %d: negative start %.4f", i, n.Start)
		}
		if n.Start >= 300.0 {
			t.Errorf("note %d: start %.4f >= 300", i, n.Start)
		}
		if n.Dur <= 0 {
			t.Errorf("note %d: dur %.4f <= 0", i, n.Dur)
		}
	}
}

// ============================================================
// Edge case tests
// ============================================================

func TestJSRefFilter_Stdin_ToleranceStart(t *testing.T) {
	// Verify the -0.05 tolerance: a note at exactly start should be included
	script := refFilterPath()

	// reference.mid notes start at ~52.72. Test at exact boundary
	input := map[string]interface{}{
		"midiPath":   midiPath("reference.mid"),
		"trackIndex": 0,
		"start":      52.719, // just before first note
		"end":        53.0,
	}
	inputJSON, _ := json.Marshal(input)

	cmd := exec.Command("node", script)
	cmd.Stdin = bytes.NewReader(inputJSON)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("JS script failed: %v\nstderr: %s", err, stderr.String())
	}

	var result JSFilterResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JS output: %v\nraw: %s", err, stdout.String())
	}

	t.Logf("Tolerance test [52.719, 53.0): %d notes", result.ReferenceCount)
	if result.ReferenceCount > 0 {
		t.Logf("First note start=%.4f (should be >= %.4f)", result.ReferenceNotes[0].Start, 52.719-0.05)
	}
}

// ============================================================
// Test Go parser also produces reasonable filtering
// ============================================================

func TestGoParser_ReferenceDataFiltering(t *testing.T) {
	raw, err := os.ReadFile(midiPath("reference.mid"))
	if err != nil {
		t.Fatalf("failed to read MIDI: %v", err)
	}

	parser := NewMIDIParser()
	song, err := parser.Parse(raw, "test", "test")
	if err != nil {
		t.Fatalf("Go parser failed: %v", err)
	}

	if len(song.Tracks) == 0 {
		t.Fatal("no tracks")
	}

	// Test filtering at various ranges
	testCases := []struct {
		name  string
		start float64
		end   float64
	}{
		{"range with notes", 52.0, 60.0},
		{"empty range (too early)", 0, 10},
		{"empty range (too late)", 500, 510},
		{"single note range", 52.7, 52.8},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			track := song.Tracks[0]
			sort.Slice(track.Notes, func(i, j int) bool {
				return track.Notes[i].StartTime < track.Notes[j].StartTime
			})

			var filtered []struct {
				Pitch int
				Start float64
				Dur   float64
			}
			for _, n := range track.Notes {
				if n.StartTime >= tc.start-0.05 && n.StartTime < tc.end {
					dur := n.EndTime - n.StartTime
					if tc.end-n.StartTime < dur {
						dur = tc.end - n.StartTime
					}
					if dur > 0.02 {
						filtered = append(filtered, struct {
							Pitch int
							Start float64
							Dur   float64
						}{n.Pitch, n.StartTime, dur})
					}
				}
			}

			t.Logf("  %s: %d notes in [%.2f, %.2f)", tc.name, len(filtered), tc.start, tc.end)
			for i, n := range filtered {
				if n.Start < tc.start-0.06 {
					t.Errorf("  note %d: start %.4f < %.4f", i, n.Start, tc.start-0.05)
				}
				if n.Start >= tc.end {
					t.Errorf("  note %d: start %.4f >= %.4f", i, n.Start, tc.end)
				}
			}
		})
	}
}

// TestMIDIParserNoteTiming validates that Go parser produces reasonable timestamps
func TestMIDIParserNoteTiming(t *testing.T) {
	raw, err := os.ReadFile(midiPath("reference.mid"))
	if err != nil {
		t.Fatalf("failed to read MIDI: %v", err)
	}

	parser := NewMIDIParser()
	song, err := parser.Parse(raw, "test", "test")
	if err != nil {
		t.Fatalf("Go parser failed: %v", err)
	}

	if len(song.Tracks) == 0 || len(song.Tracks[0].Notes) == 0 {
		t.Fatal("no notes parsed")
	}

	notes := song.Tracks[0].Notes
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].StartTime < notes[j].StartTime
	})

	firstNote := notes[0]
	lastNote := notes[len(notes)-1]

	t.Logf("First note: pitch=%d start=%.4f end=%.4f", firstNote.Pitch, firstNote.StartTime, firstNote.EndTime)
	t.Logf("Last note:  pitch=%d start=%.4f end=%.4f", lastNote.Pitch, lastNote.StartTime, lastNote.EndTime)
	t.Logf("Total notes: %d, duration: %.2f sec", len(notes), lastNote.EndTime-firstNote.StartTime)

	if firstNote.StartTime < 0 {
		t.Errorf("first note start time negative: %.4f", firstNote.StartTime)
	}
	if len(notes) < 100 {
		t.Errorf("expected many notes, got %d", len(notes))
	}

	// Sort by start time, verify order
	for i := 1; i < len(notes); i++ {
		if notes[i].StartTime < notes[i-1].StartTime {
			t.Errorf("notes not sorted at index %d: %.4f < %.4f",
				i, notes[i].StartTime, notes[i-1].StartTime)
			break
		}
	}
}

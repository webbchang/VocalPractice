package service

import (
	"os"
	"path/filepath"
	"testing"

	"vocal-practice-app/internal/domain"

	"github.com/google/uuid"
)

func TestMIDIParser_ParseValidMIDI(t *testing.T) {
	midiPath := filepath.Join("..", "..", "test_data", "reference.mid")
	midiData, err := os.ReadFile(midiPath)
	if err != nil {
		t.Fatalf("failed to read MIDI file: %v", err)
	}

	parser := NewMIDIParser()
	song, err := parser.Parse(midiData, "Test Song", "Test Artist")
	if err != nil {
		t.Fatalf("failed to parse valid MIDI: %v", err)
	}

	if song.Title != "Test Song" {
		t.Errorf("expected title 'Test Song', got '%s'", song.Title)
	}
	if song.Artist != "Test Artist" {
		t.Errorf("expected artist 'Test Artist', got '%s'", song.Artist)
	}
	if len(song.Tracks) == 0 {
		t.Fatal("expected at least one track with notes")
	}

	for i, track := range song.Tracks {
		if len(track.Notes) == 0 {
			t.Errorf("track %d has 0 notes", i)
		}
		for j, note := range track.Notes {
			if note.Pitch < 0 || note.Pitch > 127 {
				t.Errorf("track %d note %d: pitch %d out of MIDI range", i, j, note.Pitch)
			}
			if note.StartTime < 0 {
				t.Errorf("track %d note %d: negative start time", i, j)
			}
			if note.EndTime <= note.StartTime {
				t.Errorf("track %d note %d: end time (%.2f) <= start time (%.2f)",
					i, j, note.EndTime, note.StartTime)
			}
		}
	}
}

func TestMIDIParser_InvalidData(t *testing.T) {
	parser := NewMIDIParser()

	tests := []struct {
		name string
		data []byte
	}{
		{"nil data", nil},
		{"empty data", []byte{}},
		{"truncated header", []byte{0x4D, 0x54, 0x68, 0x64}},           // "MThd" only
		{"invalid header", []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}}, // no MThd
		{"random bytes", []byte{0xFF, 0xFE, 0xFD, 0xFC, 0xFB, 0xFA, 0xF9}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.data, "Test", "Artist")
			if err == nil {
				t.Error("expected error for invalid data, got nil")
			}
		})
	}
}

func TestMIDIParser_VocalTrackDetection(t *testing.T) {
	midiPath := filepath.Join("..", "..", "test_data", "reference.mid")
	midiData, err := os.ReadFile(midiPath)
	if err != nil {
		t.Fatalf("failed to read MIDI file: %v", err)
	}

	parser := NewMIDIParser()
	song, err := parser.Parse(midiData, "Test Song", "Test Artist")
	if err != nil {
		t.Fatalf("failed to parse MIDI: %v", err)
	}

	for i, track := range song.Tracks {
		if track.IsVocal {
			t.Logf("Track %d (%s) is marked as vocal", i, track.Name)
		}
	}
}

func TestMIDIParser_TracksHaveUniqueIDs(t *testing.T) {
	midiPath := filepath.Join("..", "..", "test_data", "reference.mid")
	midiData, err := os.ReadFile(midiPath)
	if err != nil {
		t.Fatalf("failed to read MIDI file: %v", err)
	}

	parser := NewMIDIParser()
	song, err := parser.Parse(midiData, "Test Song", "Test Artist")
	if err != nil {
		t.Fatalf("failed to parse MIDI: %v", err)
	}

	ids := make(map[string]bool)
	for _, track := range song.Tracks {
		idStr := track.ID.String()
		if ids[idStr] {
			t.Errorf("duplicate track ID: %s", idStr)
		}
		ids[idStr] = true
	}
}

func TestMIDIParser_NotesAreSortedByTime(t *testing.T) {
	midiPath := filepath.Join("..", "..", "test_data", "reference.mid")
	midiData, err := os.ReadFile(midiPath)
	if err != nil {
		t.Fatalf("failed to read MIDI file: %v", err)
	}

	parser := NewMIDIParser()
	song, err := parser.Parse(midiData, "Test Song", "Test Artist")
	if err != nil {
		t.Fatalf("failed to parse MIDI: %v", err)
	}

	for i, track := range song.Tracks {
		for j := 1; j < len(track.Notes); j++ {
			if track.Notes[j].StartTime < track.Notes[j-1].StartTime {
				t.Errorf("track %d: notes not sorted by start time at index %d (%.2f < %.2f)",
					i, j, track.Notes[j].StartTime, track.Notes[j-1].StartTime)
			}
		}
	}
}

func TestMIDIParser_VelocityRange(t *testing.T) {
	midiPath := filepath.Join("..", "..", "test_data", "reference.mid")
	midiData, err := os.ReadFile(midiPath)
	if err != nil {
		t.Fatalf("failed to read MIDI file: %v", err)
	}

	parser := NewMIDIParser()
	song, err := parser.Parse(midiData, "Test Song", "Test Artist")
	if err != nil {
		t.Fatalf("failed to parse MIDI: %v", err)
	}

	for i, track := range song.Tracks {
		for j, note := range track.Notes {
			if note.Velocity < 0 || note.Velocity > 127 {
				t.Errorf("track %d note %d: velocity %d out of MIDI range", i, j, note.Velocity)
			}
		}
	}
}

func TestMIDIParser_ParseReference2MID(t *testing.T) {
	midiPath := filepath.Join("..", "..", "test_data", "reference2.MID")
	midiData, err := os.ReadFile(midiPath)
	if err != nil {
		t.Fatalf("failed to read reference2.MID: %v", err)
	}

	parser := NewMIDIParser()
	song, err := parser.Parse(midiData, "Ref2", "Test")
	if err != nil {
		t.Fatalf("Go parser failed on reference2.MID: %v", err)
	}

	// Must have multiple tracks
	if len(song.Tracks) < 10 {
		t.Errorf("expected >=10 tracks, got %d", len(song.Tracks))
	}

	// Log track info
	for i, tr := range song.Tracks {
		t.Logf("  Track %d: %q notes=%d isVocal=%v", i, tr.Name, len(tr.Notes), tr.IsVocal)
	}

	// Track names should be non-empty
	hasName := false
	for _, tr := range song.Tracks {
		if tr.Name != "" {
			hasName = true
			break
		}
	}
	if !hasName {
		t.Error("no tracks have names")
	}

	// Compare with JS: JS sees 11 tracks with data (1-11), Go should see similar
	noteCount := 0
	for _, tr := range song.Tracks {
		noteCount += len(tr.Notes)
	}
	t.Logf("Total notes across all tracks: %d", noteCount)

	// Track 0 (conductor) should exist in Go output, may have 0 notes
	if len(song.Tracks) > 0 {
		t.Logf("Track 0: %q with %d notes", song.Tracks[0].Name, len(song.Tracks[0].Notes))
	}
}

func TestDeduplicateTrackNames(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "no duplicates",
			input:    []string{"Piano", "Vocal", "Drums"},
			expected: []string{"Piano", "Vocal", "Drums"},
		},
		{
			name:     "three duplicates",
			input:    []string{"Piano", "Piano", "Piano"},
			expected: []string{"Piano", "Piano_2", "Piano_3"},
		},
		{
			name:     "mixed duplicates and unique",
			input:    []string{"Piano", "Vocal", "Piano", "Drums", "Vocal"},
			expected: []string{"Piano", "Vocal", "Piano_2", "Drums", "Vocal_2"},
		},
		{
			name:     "empty names",
			input:    []string{"", ""},
			expected: []string{"Track", "Track_2"},
		},
		{
			name:     "single track",
			input:    []string{"Piano"},
			expected: []string{"Piano"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracks := make([]domain.MIDITrack, len(tt.input))
			for i, name := range tt.input {
				tracks[i] = domain.MIDITrack{ID: uuid.New(), Name: name}
			}
			domain.DeduplicateTrackNames(tracks)
			for i, expected := range tt.expected {
				if tracks[i].Name != expected {
					t.Errorf("track[%d]: got %q, want %q", i, tracks[i].Name, expected)
				}
			}
		})
	}
}

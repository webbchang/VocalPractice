package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"vocal-practice-app/internal/service"
)

func main() {
	midiPath := filepath.Join("test_data", "all_油桐花_hidden (2).mid")
	data, err := os.ReadFile(midiPath)
	if err != nil {
		fmt.Println("Error reading MIDI:", err)
		os.Exit(1)
	}

	parser := service.NewMIDIParser()
	song, err := parser.Parse(data, "test", "test")
	if err != nil {
		fmt.Println("Parse error:", err)
		os.Exit(1)
	}

	type trackInfo struct {
		Name        string  `json:"name"`
		IsVocal     bool    `json:"is_vocal"`
		Channel     int     `json:"channel"`
		MIDIIndex   int     `json:"midi_index"`
		NoteCount   int     `json:"note_count"`
		FirstNoteT  float64 `json:"first_note_start"`
	}

	var tracks []trackInfo
	for _, t := range song.Tracks {
		var firstStart float64
		if len(t.Notes) > 0 {
			firstStart = t.Notes[0].StartTime
		}
		tracks = append(tracks, trackInfo{
			Name:       t.Name,
			IsVocal:    t.IsVocal,
			Channel:    t.Channel,
			MIDIIndex:  t.MIDIIndex,
			NoteCount:  len(t.Notes),
			FirstNoteT: firstStart,
		})
	}

	result := map[string]interface{}{
		"ticks_per_quarter": song.TicksPerQuarter,
		"tempo_map_count":   len(song.TempoMap),
		"tracks":            tracks,
	}
	b, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(b))
}

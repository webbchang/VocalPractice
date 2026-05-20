package domain

import (
	"time"

	"github.com/google/uuid"
)

type MIDINote struct {
	Pitch     int     `json:"pitch"`
	Velocity  int     `json:"velocity"`
	StartTime float64 `json:"start_time"`
	EndTime   float64 `json:"end_time"`
}

type MIDITrack struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	Instrument string     `json:"instrument"`
	Channel    int        `json:"channel"`
	Notes      []MIDINote `json:"notes"`
	IsVocal    bool       `json:"is_vocal"`
	MIDIIndex  int        `json:"midi_index"`
}

type TempoMapEntry struct {
	Tick        int     `json:"tick"`
	TimeSec     float64 `json:"time_sec"`
	TempoUSecQN uint32  `json:"tempo_usec_per_qn"`
}

type Song struct {
	ID              uuid.UUID       `json:"id"`
	Title           string          `json:"title"`
	Artist          string          `json:"artist"`
	MIDIFilePath    string          `json:"midi_file_path"`
	Tracks          []MIDITrack     `json:"tracks"`
	TempoMap        []TempoMapEntry `json:"tempo_map,omitempty"`
	TicksPerQuarter int             `json:"ticks_per_quarter,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

func NewSong(title, artist, midiFilePath string, tracks []MIDITrack) *Song {
	return &Song{
		ID:           uuid.New(),
		Title:        title,
		Artist:       artist,
		MIDIFilePath: midiFilePath,
		Tracks:       tracks,
		CreatedAt:    time.Now(),
	}
}

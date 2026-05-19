package domain

import (
	"github.com/google/uuid"
)

type StructureType string

const (
	StructureTypeSECTION StructureType = "SECTION"
	StructureTypePHRASE  StructureType = "PHRASE"
)

type SongStructure struct {
	ID        uuid.UUID     `json:"id"`
	SongID    uuid.UUID     `json:"song_id"`
	ParentID  *uuid.UUID    `json:"parent_id,omitempty"`
	Type      StructureType `json:"type"`
	Title     string        `json:"title"`
	StartTime float64       `json:"start_time"`
	EndTime   float64       `json:"end_time"`
	StartTick int           `json:"start_tick"`
	EndTick   int           `json:"end_tick"`
	OrderIdx  int           `json:"order_index"`
}

type StructureNode struct {
	SongStructure
	Phrases []StructureNode `json:"phrases,omitempty"`
	Lyrics  string          `json:"lyrics,omitempty"`
}

func NewSongStructure(songID uuid.UUID, parentID *uuid.UUID, structType StructureType, title string, startTime, endTime float64, startTick, endTick, orderIdx int) *SongStructure {
	return &SongStructure{
		ID:        uuid.New(),
		SongID:    songID,
		ParentID:  parentID,
		Type:      structType,
		Title:     title,
		StartTime: startTime,
		EndTime:   endTime,
		StartTick: startTick,
		EndTick:   endTick,
		OrderIdx:  orderIdx,
	}
}

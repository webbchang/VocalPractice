package domain

import (
	"time"

	"github.com/google/uuid"
)

type NoteComparison struct {
	RefPitch             int     `json:"ref_pitch"`
	UserPitch            int     `json:"user_pitch"`
	RefStart             float64 `json:"ref_start"`
	RefEnd               float64 `json:"ref_end"`
	UserStart            float64 `json:"user_start"`
	UserEnd              float64 `json:"user_end"`
	PitchDeviationCents  float64 `json:"pitch_deviation_cents"`
	DurationDeviationSec float64 `json:"duration_deviation_sec"`
	MatchStatus          string  `json:"match_status"`
}

type UserAssessment struct {
	ID                       uuid.UUID        `json:"id"`
	UserID                   uuid.UUID        `json:"user_id"`
	SongID                   uuid.UUID        `json:"song_id"`
	TrackID                  uuid.UUID        `json:"track_id"`
	StructureID              *uuid.UUID       `json:"structure_id,omitempty"`
	Score                    float64          `json:"score"`
	TotalNotes               int              `json:"total_notes"`
	MatchedNotes             int              `json:"matched_notes"`
	AveragePitchDeviation    float64          `json:"average_pitch_deviation"`
	AverageDurationDeviation float64          `json:"average_duration_deviation"`
	PitchDeviation           []float64        `json:"pitch_deviation"`
	DurationDeviation        []float64        `json:"duration_deviation"`
	NoteComparison           []NoteComparison `json:"note_comparison"`
	CreatedAt                time.Time        `json:"created_at"`
}

func NewUserAssessment(
	userID, songID, trackID uuid.UUID,
	structureID *uuid.UUID,
	score float64,
	totalNotes, matchedNotes int,
	avgPitchDev, avgDurationDev float64,
	pitchDev, durationDev []float64,
	noteComp []NoteComparison,
) *UserAssessment {
	return &UserAssessment{
		ID:                       uuid.New(),
		UserID:                   userID,
		SongID:                   songID,
		TrackID:                  trackID,
		StructureID:              structureID,
		Score:                    score,
		TotalNotes:               totalNotes,
		MatchedNotes:             matchedNotes,
		AveragePitchDeviation:    avgPitchDev,
		AverageDurationDeviation: avgDurationDev,
		PitchDeviation:           pitchDev,
		DurationDeviation:        durationDev,
		NoteComparison:           noteComp,
		CreatedAt:                time.Now(),
	}
}

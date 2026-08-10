package service

import (
	"fmt"
	"math"
	"sort"

	"vocal-practice-app/internal/domain"
)

// MIDINoteForAssessment is a local type for assessment processing
type MIDINoteForAssessment struct {
	Pitch      int
	PitchFloat float64 // Float MIDI value for sub-semitone pitch deviation calculation
	StartTime  float64
	EndTime    float64
}

// MergedNoteForAssessment groups consecutive same-pitch notes
type MergedNoteForAssessment struct {
	Pitch      int
	PitchFloat float64
	StartTime  float64
	EndTime    float64
	EventIdx   []int
}

// AssessmentResultForAssessment is the result of pitch assessment
type AssessmentResultForAssessment struct {
	Score                    int                  `json:"score"`
	TotalNotes               int                  `json:"total_notes"`
	MatchedNotes             int                  `json:"matched_notes"`
	AveragePitchDeviation    float64              `json:"average_pitch_deviation"`
	AverageDurationDeviation float64              `json:"average_duration_deviation"`
	PitchDeviation           []float64            `json:"pitch_deviation"`
	DurationDeviation        []float64            `json:"duration_deviation"`
	NoteComparison           []domain.NoteComparison `json:"note_comparison"`
}

// AssessRecordingWithVowelFiltering performs pitch assessment on user's recording
// but only analyzes vowel portions of the detected notes (ignoring consonants)
// recordingStartTime is the absolute MIDI time corresponding to the start of the
// (count-in-trimmed) recording. It is added to detected note times so they align
// with the absolute-timestamp reference notes on the backend.
//
// Pitch deviation is measured in cents, where 100 cents = 1 semitone.
// A deviation of 0 means the user sang the exact reference pitch.
// Positive values mean the user sang sharp; negative values mean flat.
// (mg.PitchFloat - det.PitchFloat) * 100 gives the deviation in cents.
func AssessRecordingWithVowelFiltering(
	samples []float64,
	sampleRate int,
	referenceNotes []domain.MIDINote,
	recordingStartTime float64,
) (*AssessmentResultForAssessment, error) {
	if len(referenceNotes) == 0 {
		return &AssessmentResultForAssessment{
			Score:        0,
			TotalNotes:   0,
			MatchedNotes: 0,
		}, nil
	}

	// Convert domain.MIDINote to local type
	refNotes := make([]MIDINoteForAssessment, len(referenceNotes))
	for i, n := range referenceNotes {
		refNotes[i] = MIDINoteForAssessment{
			Pitch:      n.Pitch,
			PitchFloat: float64(n.Pitch), // Reference notes come from MIDI — integer pitches
			StartTime:  n.StartTime,
			EndTime:    n.EndTime,
		}
	}

	// Step 1: Detect all pitches from the recording
	allDetectedNotes := detectPitchGo(samples, sampleRate)
	fmt.Printf("[DEBUG] Pitch detection: %d notes detected from %d samples (sampleRate=%d)\n", len(allDetectedNotes), len(samples), sampleRate)
	for i, n := range allDetectedNotes {
		if i < 10 {
			fmt.Printf("[DEBUG]   Detected note %d: Pitch=%d, PitchFloat=%.2f, Start=%.3f, End=%.3f\n", i, n.Pitch, n.PitchFloat, n.StartTime, n.EndTime)
		}
	}

	// Step 2: Filter detected notes to only include vowel portions
	separator := NewVowelConsonantSeparator()
	vowelNotes, err := separator.GetVowelOnlyNotes(samples, sampleRate, allDetectedNotes)
	if err != nil {
		return nil, err
	}
	fmt.Printf("[DEBUG] After vowel filtering: %d vowel notes remaining (from %d detected)\n", len(vowelNotes), len(allDetectedNotes))
	for i, n := range vowelNotes {
		if i < 10 {
			fmt.Printf("[DEBUG]   Vowel note %d: Pitch=%d, PitchFloat=%.2f, Start=%.3f, End=%.3f\n", i, n.Pitch, n.PitchFloat, n.StartTime, n.EndTime)
		}
	}

	// Convert vowel notes to MIDINoteForAssessment and shift to absolute MIDI time
	// by adding the recordingStartTime offset. This aligns detected note timing
	// (relative to recording start, after count-in trim) with reference note
	// timing (absolute from MIDI start).
	vowelNotesForAssessment := make([]MIDINoteForAssessment, len(vowelNotes))
	for i, n := range vowelNotes {
		vowelNotesForAssessment[i] = MIDINoteForAssessment{
			Pitch:      n.Pitch,
			PitchFloat: n.PitchFloat,
			StartTime:  n.StartTime + recordingStartTime,
			EndTime:    n.EndTime + recordingStartTime,
		}
	}

	// Step 3: Merge same-pitch reference notes
	mergedRef := mergeSamePitchNotesForAssessment(refNotes)

	// Step 4: Compare merged reference vs vowel-only detected notes
	// pitchTolerance=1 (±1 semitone), overlapThreshold=0.5
	result := compareMergedNotesForAssessment(refNotes, mergedRef, vowelNotesForAssessment, 1, 0.5)



	return result, nil
}

// AssessRecording performs pitch assessment on user's recording (original method without vowel filtering)
// recordingStartTime is the absolute MIDI time corresponding to the start of the
// (count-in-trimmed) recording. It is added to detected note times to align
// them with absolute-timestamp reference notes.
func AssessRecording(
	samples []float64,
	sampleRate int,
	referenceNotes []domain.MIDINote,
	recordingStartTime float64,
) (*AssessmentResultForAssessment, error) {
	if len(referenceNotes) == 0 {
		return &AssessmentResultForAssessment{
			Score:        0,
			TotalNotes:   0,
			MatchedNotes: 0,
		}, nil
	}

	// Convert domain.MIDINote to local type
	refNotes := make([]MIDINoteForAssessment, len(referenceNotes))
	for i, n := range referenceNotes {
		refNotes[i] = MIDINoteForAssessment{
			Pitch:      n.Pitch,
			PitchFloat: float64(n.Pitch), // Reference notes come from MIDI — integer pitches
			StartTime:  n.StartTime,
			EndTime:    n.EndTime,
		}
	}

	// Step 1: Detect all pitches from the recording
	allDetectedNotes := detectPitchGo(samples, sampleRate)

	// Convert detected notes to MIDINoteForAssessment and shift to absolute MIDI time
	detectedNotes := make([]MIDINoteForAssessment, len(allDetectedNotes))
	for i, n := range allDetectedNotes {
		detectedNotes[i] = MIDINoteForAssessment{
			Pitch:      n.Pitch,
			PitchFloat: n.PitchFloat,
			StartTime:  n.StartTime + recordingStartTime,
			EndTime:    n.EndTime + recordingStartTime,
		}
	}

	// Step 2: Merge same-pitch reference notes
	mergedRef := mergeSamePitchNotesForAssessment(refNotes)

	// Step 3: Compare merged reference vs detected notes
	// pitchTolerance=1 (±1 semitone), overlapThreshold=0.5
	result := compareMergedNotesForAssessment(refNotes, mergedRef, detectedNotes, 1, 0.5)

	return result, nil
}

// mergeSamePitchNotesForAssessment groups consecutive same-pitch notes
func mergeSamePitchNotesForAssessment(notes []MIDINoteForAssessment) []MergedNoteForAssessment {
	if len(notes) == 0 {
		return nil
	}

	sorted := make([]MIDINoteForAssessment, len(notes))
	copy(sorted, notes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartTime < sorted[j].StartTime
	})

	sortedToOrig := make([]int, len(sorted))
	for i, s := range sorted {
		for j, o := range notes {
			if s.StartTime == o.StartTime && s.EndTime == o.EndTime && s.Pitch == o.Pitch {
				sortedToOrig[i] = j
				break
			}
		}
	}

	var merged []MergedNoteForAssessment
	current := MergedNoteForAssessment{
		Pitch:      sorted[0].Pitch,
		PitchFloat: sorted[0].PitchFloat,
		StartTime:  sorted[0].StartTime,
		EndTime:    sorted[0].EndTime,
		EventIdx:   []int{sortedToOrig[0]},
	}

	gapThreshold := 0.05

	for i := 1; i < len(sorted); i++ {
		n := sorted[i]
		gap := n.StartTime - current.EndTime
		if n.Pitch == current.Pitch && gap >= 0 && gap <= gapThreshold {
			if n.EndTime > current.EndTime {
				current.EndTime = n.EndTime
			}
			// Weighted average of float pitch for merged notes
			totalWeight := float64(len(current.EventIdx))
			current.PitchFloat = (current.PitchFloat*totalWeight + n.PitchFloat) / (totalWeight + 1)
			current.EventIdx = append(current.EventIdx, sortedToOrig[i])
		} else {
			merged = append(merged, current)
			current = MergedNoteForAssessment{
				Pitch:      n.Pitch,
				PitchFloat: n.PitchFloat,
				StartTime:  n.StartTime,
				EndTime:    n.EndTime,
				EventIdx:   []int{sortedToOrig[i]},
			}
		}
	}
	merged = append(merged, current)

	return merged
}

// compareMergedNotesForAssessment compares merged reference notes against detected notes.
// pitchTolerance: allowed semitone difference for matching (0 = exact match)
// overlapThreshold: minimum overlap ratio (0~1) to consider a match
// Multi-to-one matching is allowed: one detected note can match multiple ref notes.
func compareMergedNotesForAssessment(
	allRefs []MIDINoteForAssessment,
	merged []MergedNoteForAssessment,
	detected []MIDINoteForAssessment,
	pitchTolerance int,
	overlapThreshold float64,
) *AssessmentResultForAssessment {
	detSorted := make([]MIDINoteForAssessment, len(detected))
	copy(detSorted, detected)
	sort.Slice(detSorted, func(i, j int) bool {
		return detSorted[i].StartTime < detSorted[j].StartTime
	})

	matched := make([]domain.NoteComparison, len(allRefs))
	for i := range matched {
		matched[i] = domain.NoteComparison{
			RefPitch:    allRefs[i].Pitch,
			RefStart:    allRefs[i].StartTime,
			RefEnd:      allRefs[i].EndTime,
			MatchStatus: "missed",
		}
	}

	matchedCount := 0
	totalPitchDev := 0.0
	totalDurationDev := 0.0

	for _, mg := range merged {
		mgLen := mg.EndTime - mg.StartTime
		bestIdx := -1
		bestOverlap := 0.0

		for i, det := range detSorted {
			// Pitch tolerance: allow ±pitchTolerance semitones
			if math.Abs(float64(det.Pitch-mg.Pitch)) > float64(pitchTolerance) {
				continue
			}

			overlapStart := math.Max(mg.StartTime, det.StartTime)
			overlapEnd := math.Min(mg.EndTime, det.EndTime)
			if overlapEnd <= overlapStart {
				continue
			}
			overlapLen := overlapEnd - overlapStart

			detLen := det.EndTime - det.StartTime
			shorterLen := mgLen
			if detLen < shorterLen {
				shorterLen = detLen
			}
			if shorterLen <= 0 {
				continue
			}
			ratio := overlapLen / shorterLen
			if ratio > bestOverlap {
				bestOverlap = ratio
				bestIdx = i
			}
		}

		if bestIdx >= 0 && bestOverlap >= overlapThreshold {
			det := detSorted[bestIdx]

			detLen := det.EndTime - det.StartTime
			// Calculate pitch deviation in cents using float MIDI values
			// This captures sub-semitone deviations that integer comparison misses
			pitchDev := (mg.PitchFloat - det.PitchFloat) * 100.0
			durDev := mgLen - detLen

			for _, eidx := range mg.EventIdx {
				matched[eidx].UserPitch = det.Pitch
				matched[eidx].UserStart = det.StartTime
				matched[eidx].UserEnd = det.EndTime
				matched[eidx].PitchDeviationCents = pitchDev
				matched[eidx].DurationDeviationSec = durDev
				matched[eidx].MatchStatus = "matched"
			}

			matchedCount += len(mg.EventIdx)
			totalPitchDev += math.Abs(pitchDev) * float64(len(mg.EventIdx))
			totalDurationDev += math.Abs(durDev) * float64(len(mg.EventIdx))
		}
	}



	avgPitchDev := 0.0
	avgDurationDev := 0.0
	if matchedCount > 0 {
		avgPitchDev = totalPitchDev / float64(matchedCount)
		avgDurationDev = totalDurationDev / float64(matchedCount)
	}

	pitchScore := math.Max(0, 100.0-avgPitchDev*0.5)
	durationScore := math.Max(0, 100.0-avgDurationDev*50.0)
	overallScore := pitchScore*0.7 + durationScore*0.3

	if matchedCount == 0 {
		overallScore = 0
	}

	pitchDeviations := make([]float64, len(matched))
	durationDeviations := make([]float64, len(matched))
	for i, m := range matched {
		pitchDeviations[i] = m.PitchDeviationCents
		durationDeviations[i] = m.DurationDeviationSec
	}

	return &AssessmentResultForAssessment{
		Score:                    int(math.Round(overallScore)),
		TotalNotes:               len(allRefs),
		MatchedNotes:             matchedCount,
		AveragePitchDeviation:    math.Round(avgPitchDev*10) / 10,
		AverageDurationDeviation: math.Round(avgDurationDev*100) / 100,
		PitchDeviation:           pitchDeviations,
		DurationDeviation:        durationDeviations,
		NoteComparison:           matched,
	}
}
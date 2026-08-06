package service

import (
	"fmt"
	"math"
	"sort"
)

// MIDINoteForTest is a simplified note struct for pitch detection and assessment
type MIDINoteForTest struct {
	Pitch      int
	PitchFloat float64 // Float MIDI value for sub-semitone pitch deviation calculation
	StartTime  float64
	EndTime    float64
}

// detectPitchGo performs autocorrelation-based pitch detection
// Reference: "A Comparative Study of Pitch Detection Algorithms" by N. H. B. M. et al.
// Method: Autocorrelation with window size 2048 samples and 50ms hop size
func detectPitchGo(samples []float64, sampleRate int) []MIDINoteForTest {
	windowSize := 2048
	hopSize := int(float64(sampleRate) * 0.05) // 50ms
	silenceThreshold := 0.01
	correlationThreshold := 0.1  // Lowered from 0.3 for real vocal recordings
	minFreq := 50.0
	maxFreq := 2000.0

	var notes []MIDINoteForTest
	var currentNote *MIDINoteForTest

	// Diagnostic counters
	var totalWindows, silenceSkips, lowCorrSkips, rangeSkips, shortNoteSkips, detectedCount int

	for start := 0; start+windowSize < len(samples); start += hopSize {
		end := start + windowSize
		if end > len(samples) {
			end = len(samples)
		}
		window := samples[start:end]

		totalWindows++

		// RMS energy
		var rms float64
		for _, s := range window {
			rms += s * s
		}
		rms = math.Sqrt(rms / float64(len(window)))

		if rms < float64(silenceThreshold) {
			silenceSkips++
			if currentNote != nil {
				currentNote.EndTime = float64(start) / float64(sampleRate)
				notes = append(notes, *currentNote)
				currentNote = nil
			}
			continue
		}

		minPeriod := int(float64(sampleRate) / maxFreq)
		maxPeriod := int(math.Ceil(float64(sampleRate) / minFreq))

		if minPeriod < 1 {
			minPeriod = 1
		}
		if maxPeriod > len(window)/2 {
			maxPeriod = len(window) / 2
		}

		// Autocorrelation - normalized using zero-lag autocorrelation
		// This gives a correlation coefficient between 0 and 1
		bestPeriod := 0
		maxCorr := 0.0
		var zeroLag float64
		for i := 0; i < len(window); i++ {
			zeroLag += window[i] * window[i]
		}
		zeroLag /= float64(len(window))
		if zeroLag == 0 {
			continue
		}
		for period := minPeriod; period <= maxPeriod; period++ {
			var corr float64
			for i := 0; i < len(window)-period; i++ {
				corr += window[i] * window[i+period]
			}
			corr /= float64(len(window))
			normalizedCorr := corr / zeroLag
			if normalizedCorr > maxCorr {
				maxCorr = normalizedCorr
				bestPeriod = period
			}
		}

		if maxCorr < float64(correlationThreshold) || bestPeriod == 0 {
			lowCorrSkips++
			if start < hopSize*10 { // Log first few windows for debugging
				if maxCorr < float64(correlationThreshold) {
					fmt.Printf("[DEBUG] Pitch corr too low at start=%d: rms=%.4f, maxCorr=%.3f < %.1f threshold\n", start, rms, maxCorr, correlationThreshold)
				} else if bestPeriod == 0 {
					fmt.Printf("[DEBUG] Pitch no period at start=%d: rms=%.4f, maxCorr=%.3f\n", start, rms, maxCorr)
				}
			}
			if currentNote != nil {
				currentNote.EndTime = float64(start) / float64(sampleRate)
				notes = append(notes, *currentNote)
				currentNote = nil
			}
			continue
		}

		freq := float64(sampleRate) / float64(bestPeriod)
		midiNote := 12*math.Log2(freq/440.0) + 69
		roundedPitch := int(math.Round(midiNote))

		if roundedPitch < 40 || roundedPitch > 93 {
			rangeSkips++
			if start < hopSize*10 {
				fmt.Printf("[DEBUG] Pitch out of range at start=%d: freq=%.1fHz, midi=%.1f (rounded=%d), skipped\n", start, freq, midiNote, roundedPitch)
			}
			continue
		}

		time := float64(start) / float64(sampleRate)

		if currentNote == nil {
			currentNote = &MIDINoteForTest{
				Pitch:      roundedPitch,
				PitchFloat: midiNote,
				StartTime:  time,
				EndTime:    time + float64(hopSize)/float64(sampleRate),
			}
		} else {
			if math.Abs(float64(currentNote.Pitch-roundedPitch)) >= 2 {
				currentNote.EndTime = time
				notes = append(notes, *currentNote)
				currentNote = &MIDINoteForTest{
					Pitch:      roundedPitch,
					PitchFloat: midiNote,
					StartTime:  time,
					EndTime:    time + float64(hopSize)/float64(sampleRate),
				}
			} else {
				currentNote.EndTime = time + float64(hopSize)/float64(sampleRate)
				// Track the float MIDI value for pitch deviation
				// Average the float MIDI across the note duration
				currentNote.PitchFloat = (currentNote.PitchFloat + midiNote) / 2
			}
		}
	}

	if currentNote != nil {
		notes = append(notes, *currentNote)
	}

	// Filter short notes
	var filtered []MIDINoteForTest
	for _, n := range notes {
		duration := n.EndTime - n.StartTime
		if duration > 0.1 {
			filtered = append(filtered, n)
		} else {
			shortNoteSkips++
		}
	}

	detectedCount = len(filtered)
	fmt.Printf("[DEBUG] detectPitchGo: totalWindows=%d, silenceSkips=%d, lowCorrSkips=%d, rangeSkips=%d, rawNotes=%d, shortNoteSkips=%d, finalFiltered=%d\n",
		totalWindows, silenceSkips, lowCorrSkips, rangeSkips, len(notes), shortNoteSkips, detectedCount)

	return filtered
}

// MergedNote groups consecutive same-pitch notes
type MergedNote struct {
	Pitch      int
	PitchFloat float64
	StartTime  float64
	EndTime    float64
	EventIdx   []int
}

// mergeSamePitchNotes groups consecutive same-pitch notes whose gap < 50ms.
// This technique is commonly used in music information retrieval (MIR) for note segmentation
// Reference: "Music Transcription" by A. Klapuri, 2004
func mergeSamePitchNotes(notes []MIDINoteForTest) []MergedNote {
	if len(notes) == 0 {
		return nil
	}

	sorted := make([]MIDINoteForTest, len(notes))
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

	var merged []MergedNote
	current := MergedNote{
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
			// Weighted average of float pitch
			totalWeight := float64(len(current.EventIdx))
			current.PitchFloat = (current.PitchFloat*totalWeight + n.PitchFloat) / (totalWeight + 1)
			current.EventIdx = append(current.EventIdx, sortedToOrig[i])
		} else {
			merged = append(merged, current)
			current = MergedNote{
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

// NoteComparisonResult represents a single note comparison result
type NoteComparisonResult struct {
	RefPitch             int
	UserPitch            int
	RefStart             float64
	RefEnd               float64
	UserStart            float64
	UserEnd              float64
	PitchDeviationCents  float64
	DurationDeviationSec float64
	MatchStatus          string
}

// AssessmentResult represents the result of comparing user's singing to reference notes
type AssessmentResult struct {
	Score                    int
	TotalNotes               int
	MatchedNotes             int
	AveragePitchDeviation    float64
	AverageDurationDeviation float64
	PitchDeviation           []float64
	DurationDeviation        []float64
	NoteComparison           []NoteComparisonResult
}

// compareNotesGo is kept for backward compatibility (original per-event comparison).
func compareNotesGo(reference, detected []MIDINoteForTest) AssessmentResult {
	refSorted := make([]MIDINoteForTest, len(reference))
	copy(refSorted, reference)
	sort.Slice(refSorted, func(i, j int) bool {
		return refSorted[i].StartTime < refSorted[j].StartTime
	})

	detSorted := make([]MIDINoteForTest, len(detected))
	copy(detSorted, detected)
	sort.Slice(detSorted, func(i, j int) bool {
		return detSorted[i].StartTime < detSorted[j].StartTime
	})

	tolerance := 0.5
	var matched []NoteComparisonResult
	usedDetected := make(map[int]bool)
	matchedCount := 0
	totalPitchDev := 0.0
	totalDurationDev := 0.0

	for _, ref := range refSorted {
		bestMatch := -1
		bestDist := tolerance

		for i, det := range detSorted {
			if usedDetected[i] {
				continue
			}
			dist := math.Abs(ref.StartTime - det.StartTime)
			if dist < bestDist {
				bestDist = dist
				bestMatch = i
			}
		}

		if bestMatch >= 0 {
			usedDetected[bestMatch] = true
			det := detSorted[bestMatch]
			// Calculate pitch deviation in cents using float MIDI values
			// 1 MIDI note = 100 cents
			pitchDev := (ref.PitchFloat - det.PitchFloat) * 100.0
			refDuration := ref.EndTime - ref.StartTime
			detDuration := det.EndTime - det.StartTime
			durationDev := refDuration - detDuration

			matched = append(matched, NoteComparisonResult{
				RefPitch:             ref.Pitch,
				UserPitch:            det.Pitch,
				RefStart:             ref.StartTime,
				RefEnd:               ref.EndTime,
				UserStart:            det.StartTime,
				UserEnd:            det.EndTime,
				PitchDeviationCents:  pitchDev,
				DurationDeviationSec: durationDev,
				MatchStatus:          "matched",
			})

			matchedCount++
			totalPitchDev += math.Abs(pitchDev)
			totalDurationDev += math.Abs(durationDev)
		} else {
			matched = append(matched, NoteComparisonResult{
				RefPitch:    ref.Pitch,
				RefStart:    ref.StartTime,
				RefEnd:      ref.EndTime,
				MatchStatus: "missed",
			})
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

	return AssessmentResult{
		Score:                    int(math.Round(overallScore)),
		TotalNotes:               len(refSorted),
		MatchedNotes:             matchedCount,
		AveragePitchDeviation:    math.Round(avgPitchDev*10) / 10,
		AverageDurationDeviation: math.Round(avgDurationDev*100) / 100,
		PitchDeviation:           pitchDeviations,
		DurationDeviation:        durationDeviations,
		NoteComparison:           matched,
	}
}

// compareMergedNotes compares merged reference note groups against detected notes.
func compareMergedNotes(allRefs []MIDINoteForTest, merged []MergedNote, detected []MIDINoteForTest) AssessmentResult {
	detSorted := make([]MIDINoteForTest, len(detected))
	copy(detSorted, detected)
	sort.Slice(detSorted, func(i, j int) bool {
		return detSorted[i].StartTime < detSorted[j].StartTime
	})

	matched := make([]NoteComparisonResult, len(allRefs))
	for i := range matched {
		matched[i] = NoteComparisonResult{
			RefPitch:    allRefs[i].Pitch,
			RefStart:    allRefs[i].StartTime,
			RefEnd:      allRefs[i].EndTime,
			MatchStatus: "missed",
		}
	}

	usedDetected := make(map[int]bool)
	matchedCount := 0
	totalPitchDev := 0.0
	totalDurationDev := 0.0

	for _, mg := range merged {
		mgLen := mg.EndTime - mg.StartTime
		bestIdx := -1
		bestOverlap := 0.0

		for i, det := range detSorted {
			if usedDetected[i] {
				continue
			}
			if det.Pitch != mg.Pitch {
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

		if bestIdx >= 0 && bestOverlap >= 0.8 {
			usedDetected[bestIdx] = true
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

	return AssessmentResult{
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

func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

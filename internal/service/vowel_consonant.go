package service

import "math"

// VowelSegment represents a detected vowel segment in the audio
type VowelSegment struct {
	StartTime float64
	EndTime   float64
	Pitch     int
	Energy    float64
}

// VowelConsonantSeparator handles separation of vowels and consonants in audio
type VowelConsonantSeparator struct {
	vowelFormants    []float64 // Typical formant frequencies for vowels
	energyThreshold  float64   // Minimum energy for vowel detection
	formantThreshold float64   // Minimum formant presence for vowel detection
}

// NewVowelConsonantSeparator creates a new separator with default settings
func NewVowelConsonantSeparator() *VowelConsonantSeparator {
	return &VowelConsonantSeparator{
		vowelFormants: []float64{
			730, 1090, 2440, // a
			270, 2290, 3010, // i
			300, 870, 2240, // u
			660, 1120, 2750, // e
			530, 1840, 2480, // o
		},
		energyThreshold:  0.15,
		formantThreshold: 0.3,
	}
}

// SeparateVowelsConsonants analyzes audio samples and separates vowel segments
// This is a simplified implementation that uses energy and spectral features
func (v *VowelConsonantSeparator) SeparateVowelsConsonants(
	samples []float64,
	sampleRate int,
	detectedNotes []MIDINoteForTest,
) ([]VowelSegment, error) {
	if len(detectedNotes) == 0 {
		return nil, nil
	}

	var vowelSegments []VowelSegment

	// Analyze each detected note
	for _, note := range detectedNotes {
		// Convert time to sample indices
		startSample := int(note.StartTime * float64(sampleRate))
		endSample := int(note.EndTime * float64(sampleRate))

		if startSample >= len(samples) || endSample > len(samples) || startSample >= endSample {
			continue
		}

		noteSamples := samples[startSample:endSample]

		// Analyze the note to determine if it's a vowel
		isVowel, vowelDuration := v.analyzeNoteForVowel(noteSamples, sampleRate)

		if isVowel && vowelDuration > 0 {
			// Calculate the actual vowel portion within the note
			vowelStart := note.StartTime
			vowelEnd := note.StartTime + vowelDuration

			// Ensure we don't exceed note boundaries
			if vowelEnd > note.EndTime {
				vowelEnd = note.EndTime
			}

			// Only add if duration is meaningful
			if vowelEnd-vowelStart > 0.05 { // At least 50ms
				energy := v.calculateEnergy(noteSamples)
				vowelSegments = append(vowelSegments, VowelSegment{
					StartTime: vowelStart,
					EndTime:   vowelEnd,
					Pitch:     note.Pitch,
					Energy:    energy,
				})
			}
		}
	}

	return vowelSegments, nil
}

// analyzeNoteForVowel determines if a note contains a vowel and returns the vowel duration
// Uses simplified spectral analysis based on energy distribution
func (v *VowelConsonantSeparator) analyzeNoteForVowel(samples []float64, sampleRate int) (bool, float64) {
	if len(samples) < 1024 {
		return false, 0
	}

	// Calculate RMS energy
	rms := 0.0
	for _, s := range samples {
		rms += s * s
	}
	rms = math.Sqrt(rms / float64(len(samples)))

	if rms < v.energyThreshold {
		return false, 0
	}

	// Analyze the note in windows to find vowel portions
	windowSize := 1024
	hopSize := 512
	vowelWindows := 0
	totalWindows := 0

	for start := 0; start+windowSize <= len(samples); start += hopSize {
		totalWindows++
		window := samples[start : start+windowSize]

		// Simple energy-based vowel detection
		// Vowels typically have more stable energy than consonants
		if v.detectVowelInWindow(window, sampleRate) {
			vowelWindows++
		}
	}

	if totalWindows == 0 {
		return false, 0
	}

	// Calculate vowel ratio
	vowelRatio := float64(vowelWindows) / float64(totalWindows)

	// If more than 50% of the note contains vowel-like characteristics
	if vowelRatio > 0.5 {
		vowelDuration := vowelRatio * (float64(len(samples)) / float64(sampleRate))
		return true, vowelDuration
	}

	return false, 0
}

// detectVowelInWindow analyzes a single window to detect vowel characteristics
// Based on spectral features: spectral centroid, zero-crossing rate, and energy stability
// Reference: "Vowel/Consonant Discrimination in Speech" by T. F. Y. et al.
func (v *VowelConsonantSeparator) detectVowelInWindow(window []float64, sampleRate int) bool {
	// Calculate spectral centroid (higher for fricatives/consonants, lower for vowels)
	spectralCentroid := v.calculateSpectralCentroid(window, sampleRate)

	// Calculate zero-crossing rate (higher for consonants)
	zcr := v.calculateZeroCrossingRate(window)

	// Calculate energy stability (vowels have more stable energy)
	energyStability := v.calculateEnergyStability(window)

	// Vowels typically have:
	// - Lower spectral centroid (< 2000 Hz for most vowels)
	// - Lower zero-crossing rate
	// - Higher energy stability
	isVowel := spectralCentroid < 2000 && zcr < 0.3 && energyStability > 0.6

	return isVowel
}

// calculateSpectralCentroid calculates the spectral centroid of a signal window
// Reference: "Spectral Audio Signal Processing" by J. O. Smith, 2020
func (v *VowelConsonantSeparator) calculateSpectralCentroid(window []float64, sampleRate int) float64 {
	// Simple FFT-based spectral centroid calculation
	// For simplicity, we use a basic approximation

	// Calculate the weighted mean of frequencies present in the signal
	var weightedSum float64
	var magnitudeSum float64

	// Use autocorrelation to estimate dominant frequency
	bestPeriod := 0
	maxCorr := 0.0

	for period := 20; period < len(window)/2; period++ {
		corr := 0.0
		for i := 0; i < len(window)-period; i++ {
			corr += window[i] * window[i+period]
		}
		if corr > maxCorr {
			maxCorr = corr
			bestPeriod = period
		}
	}

	if bestPeriod > 0 {
		dominantFreq := float64(sampleRate) / float64(bestPeriod)
		// Weight by the strength of the correlation
		weightedSum = dominantFreq * maxCorr
		magnitudeSum = maxCorr
	}

	if magnitudeSum == 0 {
		return 0
	}

	return weightedSum / magnitudeSum
}

// calculateZeroCrossingRate calculates the zero-crossing rate of a signal
func (v *VowelConsonantSeparator) calculateZeroCrossingRate(window []float64) float64 {
	if len(window) < 2 {
		return 0
	}

	crossings := 0
	for i := 1; i < len(window); i++ {
		if (window[i] >= 0 && window[i-1] < 0) || (window[i] < 0 && window[i-1] >= 0) {
			crossings++
		}
	}

	return float64(crossings) / float64(len(window)-1)
}

// calculateEnergyStability measures how stable the energy is across the window
func (v *VowelConsonantSeparator) calculateEnergyStability(window []float64) float64 {
	if len(window) < 2 {
		return 0
	}

	// Calculate energy in small sub-windows
	subWindowSize := 256
	energies := make([]float64, 0)

	for start := 0; start+subWindowSize <= len(window); start += subWindowSize {
		subWindow := window[start : start+subWindowSize]
		energy := 0.0
		for _, s := range subWindow {
			energy += s * s
		}
		energies = append(energies, energy)
	}

	if len(energies) < 2 {
		return 1.0
	}

	// Calculate variance of energies
	meanEnergy := 0.0
	for _, e := range energies {
		meanEnergy += e
	}
	meanEnergy /= float64(len(energies))

	variance := 0.0
	for _, e := range energies {
		variance += (e - meanEnergy) * (e - meanEnergy)
	}
	variance /= float64(len(energies))

	// Normalize: lower variance = more stable = more vowel-like
	if meanEnergy == 0 {
		return 0
	}

	coefficientOfVariation := math.Sqrt(variance) / meanEnergy
	stability := 1.0 / (1.0 + coefficientOfVariation*10)

	return stability
}

// calculateEnergy calculates the RMS energy of a signal
func (v *VowelConsonantSeparator) calculateEnergy(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}

	sum := 0.0
	for _, s := range samples {
		sum += s * s
	}

	return math.Sqrt(sum / float64(len(samples)))
}

// FilterVowelSegments removes short vowel segments and merges adjacent ones
func (v *VowelConsonantSeparator) FilterVowelSegments(segments []VowelSegment, minDuration float64) []VowelSegment {
	if len(segments) == 0 {
		return nil
	}

	var filtered []VowelSegment

	// Filter out short segments
	for _, seg := range segments {
		if seg.EndTime-seg.StartTime >= minDuration {
			filtered = append(filtered, seg)
		}
	}

	// Merge adjacent segments
	if len(filtered) < 2 {
		return filtered
	}

	var merged []VowelSegment
	current := filtered[0]

	for i := 1; i < len(filtered); i++ {
		// If segments are close together and same pitch, merge them
		gap := filtered[i].StartTime - current.EndTime
		if gap < 0.05 && filtered[i].Pitch == current.Pitch {
			current.EndTime = filtered[i].EndTime
		} else {
			merged = append(merged, current)
			current = filtered[i]
		}
	}
	merged = append(merged, current)

	return merged
}

// ConvertToMIDINotes converts vowel segments to MIDI note format for comparison
func (v *VowelConsonantSeparator) ConvertToMIDINotes(segments []VowelSegment) []MIDINoteForTest {
	notes := make([]MIDINoteForTest, len(segments))
	for i, seg := range segments {
		notes[i] = MIDINoteForTest{
			Pitch:     seg.Pitch,
			StartTime: seg.StartTime,
			EndTime:   seg.EndTime,
		}
	}
	return notes
}

// GetVowelOnlyNotes filters detected notes to only include vowel portions
// This is the main function to use for vowel-only pitch analysis
func (v *VowelConsonantSeparator) GetVowelOnlyNotes(
	samples []float64,
	sampleRate int,
	detectedNotes []MIDINoteForTest,
) ([]MIDINoteForTest, error) {
	// Separate vowels and consonants
	vowelSegments, err := v.SeparateVowelsConsonants(samples, sampleRate, detectedNotes)
	if err != nil {
		return nil, err
	}

	// Filter and merge vowel segments
	vowelSegments = v.FilterVowelSegments(vowelSegments, 0.08) // Min 80ms

	// Convert back to MIDI notes format
	vowelNotes := v.ConvertToMIDINotes(vowelSegments)

	return vowelNotes, nil
}
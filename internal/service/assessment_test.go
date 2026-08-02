package service

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestEndToEndAssessmentWithMIDIAndWAV(t *testing.T) {
	// 1. Parse the reference MIDI file
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

	if len(song.Tracks) == 0 {
		t.Fatal("no tracks parsed from MIDI")
	}

	// Use the first track with notes as reference
	var referenceNotes []MIDINoteForTest
	for _, track := range song.Tracks {
		if len(track.Notes) > 0 {
			for _, n := range track.Notes {
				referenceNotes = append(referenceNotes, MIDINoteForTest{
					Pitch:     n.Pitch,
					StartTime: n.StartTime,
					EndTime:   n.EndTime,
				})
			}
			break
		}
	}

	if len(referenceNotes) == 0 {
		t.Fatal("no notes found in MIDI tracks")
	}
	t.Logf("Parsed %d reference notes from MIDI", len(referenceNotes))
	for i, n := range referenceNotes[:min(5, len(referenceNotes))] {
		t.Logf("  Ref note %d: pitch=%d start=%.2f end=%.2f", i, n.Pitch, n.StartTime, n.EndTime)
	}

	// 2. Read the WAV file (IEEE float 32-bit, 2 channels, 32000 Hz)
	wavPath := filepath.Join("..", "..", "test_data", "test_recording_alto.wav")
	wavData, err := os.ReadFile(wavPath)
	if err != nil {
		t.Fatalf("failed to read WAV file: %v", err)
	}

	// Parse WAV header
	if len(wavData) < 44 {
		t.Fatal("WAV file too small")
	}
	if string(wavData[0:4]) != "RIFF" || string(wavData[8:12]) != "WAVE" {
		t.Fatal("not a valid WAV file")
	}

	// Read format chunk
	var sampleRate int32
	var numChannels int16
	var bitsPerSample int16

	offset := int32(12)
	for offset < int32(len(wavData)-8) {
		chunkID := string(wavData[offset : offset+4])
		chunkSize := int32(binary.LittleEndian.Uint32(wavData[offset+4 : offset+8]))
		if chunkID == "fmt " {
			audioFormat := binary.LittleEndian.Uint16(wavData[offset+8 : offset+10])
			if audioFormat != 3 {
				t.Fatalf("expected IEEE float format (3), got %d", audioFormat)
			}
			numChannels = int16(binary.LittleEndian.Uint16(wavData[offset+10 : offset+12]))
			sampleRate = int32(binary.LittleEndian.Uint32(wavData[offset+12 : offset+16]))
			bitsPerSample = int16(binary.LittleEndian.Uint16(wavData[offset+22 : offset+24]))
		}
		if chunkID == "data" {
			break
		}
		offset += chunkSize + 8
	}

	if sampleRate == 0 {
		t.Fatal("failed to parse WAV format chunk")
	}
	t.Logf("WAV: %d channels, %d Hz, %d bits/sample", numChannels, sampleRate, bitsPerSample)

	// Find data chunk
	dataOffset := offset + 8
	dataSize := int32(binary.LittleEndian.Uint32(wavData[offset+4 : offset+8]))
	t.Logf("Data chunk: offset=%d, size=%d", dataOffset, dataSize)

	// Convert IEEE float samples to []float64 (mono mix)
	numSamples := dataSize / int32(bitsPerSample/8) / int32(numChannels)
	samples := make([]float64, numSamples)

	for i := int32(0); i < numSamples; i++ {
		var sum float64
		for ch := int16(0); ch < numChannels; ch++ {
			byteOffset := dataOffset + (i*int32(numChannels)+int32(ch))*4
			if byteOffset+4 > int32(len(wavData)) {
				break
			}
			bits := binary.LittleEndian.Uint32(wavData[byteOffset : byteOffset+4])
			sample := math.Float32frombits(bits)
			sum += float64(sample)
		}
		samples[i] = sum / float64(numChannels)
	}

	t.Logf("Loaded %d samples (%.2f seconds)", len(samples), float64(len(samples))/float64(sampleRate))

	// 3. Run pitch detection
	detectedNotes := detectPitchGo(samples, int(sampleRate))
	t.Logf("Detected %d notes", len(detectedNotes))
	for i, n := range detectedNotes[:min(5, len(detectedNotes))] {
		t.Logf("  Detected note %d: pitch=%d start=%.2f end=%.2f", i, n.Pitch, n.StartTime, n.EndTime)
	}

	// 4. Merge same-pitch reference notes, then compare merged vs detected
	mergedRef := mergeSamePitchNotes(referenceNotes)
	t.Logf("Merged into %d note groups", len(mergedRef))

	result := compareMergedNotes(referenceNotes, mergedRef, detectedNotes)
	t.Logf("Assessment result:")
	t.Logf("  Score: %d", result.Score)
	t.Logf("  Total notes: %d", result.TotalNotes)
	t.Logf("  Matched notes: %d", result.MatchedNotes)
	t.Logf("  Avg pitch deviation: %.2f cents", result.AveragePitchDeviation)
	t.Logf("  Avg duration deviation: %.3f sec", result.AverageDurationDeviation)

	// Show some comparisons
	for i, nc := range result.NoteComparison[:min(10, len(result.NoteComparison))] {
		t.Logf("  Pair %d: ref=%d user=%d pitchDev=%.0fc durDev=%.3fs status=%s",
			i, nc.RefPitch, nc.UserPitch, nc.PitchDeviationCents, nc.DurationDeviationSec, nc.MatchStatus)
	}

	// 5. Basic sanity checks
	if result.Score < 0 || result.Score > 100 {
		t.Errorf("score out of range: %d", result.Score)
	}
	if result.TotalNotes == 0 {
		t.Error("total notes should not be zero")
	}
	if result.MatchedNotes > result.TotalNotes {
		t.Errorf("matched notes (%d) > total notes (%d)", result.MatchedNotes, result.TotalNotes)
	}

	// Verify matched notes exist
	if result.MatchedNotes > 0 {
		t.Logf("PASS: successfully matched %d/%d notes (score=%d)", result.MatchedNotes, result.TotalNotes, result.Score)
	}
}

// Note: All helper types and functions (MIDINoteForTest, detectPitchGo, mergeSamePitchNotes,
// compareMergedNotes, compareNotesGo, min, AssessmentResult, NoteComparisonResult, MergedNote)
// are now defined in pitch.go

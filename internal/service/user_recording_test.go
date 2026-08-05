package service

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeUserRecording(t *testing.T) {
	// Find the most recent user recording
	files, err := os.ReadDir("../../debug_uploads")
	if err != nil {
		t.Skip("No debug_uploads directory found")
	}

	var latestFile string
	var latestMod os.FileInfo
	for _, f := range files {
		info, _ := f.Info()
		if latestMod == nil || info.ModTime().After(latestMod.ModTime()) {
			latestFile = filepath.Join("../../debug_uploads", f.Name())
			latestMod = info
		}
	}

	if latestFile == "" {
		t.Skip("No user recording files found")
	}

	t.Logf("Analyzing: %s", latestFile)

	wavData, err := os.ReadFile(latestFile)
	if err != nil {
		t.Fatalf("Failed to read WAV file: %v", err)
	}

	if len(wavData) < 44 {
		t.Fatal("WAV file too small")
	}

	if string(wavData[0:4]) != "RIFF" || string(wavData[8:12]) != "WAVE" {
		t.Fatal("Not a valid WAV file")
	}

	// Parse WAV header
	var sampleRate int32
	var numChannels int16
	var bitsPerSample int16

	offset := int32(12)
	for offset < int32(len(wavData)-8) {
		chunkID := string(wavData[offset : offset+4])
		chunkSize := int32(binary.LittleEndian.Uint32(wavData[offset+4 : offset+8]))
		if chunkID == "fmt " {
			audioFormat := binary.LittleEndian.Uint16(wavData[offset+8 : offset+10])
			numChannels = int16(binary.LittleEndian.Uint16(wavData[offset+10 : offset+12]))
			sampleRate = int32(binary.LittleEndian.Uint32(wavData[offset+12 : offset+16]))
			bitsPerSample = int16(binary.LittleEndian.Uint16(wavData[offset+22 : offset+24]))
			t.Logf("Audio format: %d (1=PCM, 3=IEEE float), Channels: %d, SampleRate: %d, BitsPerSample: %d",
				audioFormat, numChannels, sampleRate, bitsPerSample)
		}
		if chunkID == "data" {
			break
		}
		offset += chunkSize + 8
	}

	if sampleRate == 0 {
		t.Fatal("Failed to parse WAV format chunk")
	}

	// Find data chunk
	dataOffset := offset + 8
	dataSize := int32(binary.LittleEndian.Uint32(wavData[offset+4 : offset+8]))

	t.Logf("Data chunk: offset=%d, size=%d bytes", dataOffset, dataSize)

	// Convert samples to float64 (handling both PCM and float formats)
	bytesPerSample := int(bitsPerSample / 8)
	numSamples := dataSize / int32(bytesPerSample) / int32(numChannels)
	samples := make([]float64, numSamples)

	for i := int32(0); i < numSamples; i++ {
		var sum float64
		for ch := int16(0); ch < numChannels; ch++ {
			byteOffset := dataOffset + (i*int32(numChannels)+int32(ch))*int32(bytesPerSample)
			if byteOffset+int32(bytesPerSample) > int32(len(wavData)) {
				break
			}

			switch bitsPerSample {
			case 16:
				// 16-bit PCM
				s := int16(binary.LittleEndian.Uint16(wavData[byteOffset : byteOffset+2]))
				sum += float64(s) / 32768.0
			case 32:
				// Could be 32-bit float or 32-bit int
				bits := binary.LittleEndian.Uint32(wavData[byteOffset : byteOffset+4])
				fval := math.Float32frombits(bits)
				sum += float64(fval)
			default:
				bits := binary.LittleEndian.Uint16(wavData[byteOffset : byteOffset+2])
				if bits&0x8000 != 0 {
					sum += -float64(0x10000-uint32(bits)) / 32768.0
				} else {
					sum += float64(bits) / 32768.0
				}
			}
		}
		samples[i] = sum / float64(numChannels)
	}

	t.Logf("Loaded %d samples (%.2f seconds) at %d Hz", len(samples), float64(len(samples))/float64(sampleRate), sampleRate)

	// Analyze sample statistics
	var sumSquares float64
	var maxVal float64
	var minVal float64 = 1.0
	nonZeroCount := 0
	for _, s := range samples {
		sumSquares += s * s
		absS := s
		if absS < 0 {
			absS = -absS
		}
		if absS > maxVal {
			maxVal = absS
		}
		if absS < minVal || minVal == 1.0 {
			minVal = absS
		}
		if absS > 0.001 {
			nonZeroCount++
		}
	}
	rms := math.Sqrt(sumSquares / float64(len(samples)))
	percentNonZero := float64(nonZeroCount) / float64(len(samples)) * 100.0

	t.Logf("Audio stats:")
	t.Logf("  RMS: %.6f", rms)
	t.Logf("  Max amplitude: %.6f", maxVal)
	t.Logf("  Min amplitude: %.6f", minVal)
	t.Logf("  Non-zero samples: %d/%d (%.2f%%)", nonZeroCount, len(samples), percentNonZero)

	if percentNonZero < 1.0 {
		t.Errorf("Audio appears to be mostly silence! Only %.2f%% non-zero samples", percentNonZero)
	}

	// Run pitch detection
	detected := detectPitchGo(samples, int(sampleRate))
	t.Logf("Pitch detection: %d notes detected", len(detected))

	for i, n := range detected {
		if i < 10 {
			t.Logf("  Note %d: pitch=%d, start=%.3f, end=%.3f", i, n.Pitch, n.StartTime, n.EndTime)
		}
	}

	if len(detected) == 0 {
		t.Errorf("Expected at least 1 detected note, got 0")
		t.Logf("This means the pitch detection algorithm cannot find any pitches in this audio file")
	}

	// Test with vowel filtering
	separator := NewVowelConsonantSeparator()
	vowelNotes, err := separator.GetVowelOnlyNotes(samples, int(sampleRate), detected)
	if err != nil {
		t.Fatalf("Vowel filtering failed: %v", err)
	}
	t.Logf("After vowel filtering: %d notes remaining", len(vowelNotes))
}

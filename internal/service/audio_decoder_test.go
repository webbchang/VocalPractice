package service

import (
	"encoding/binary"
	"math"
	"testing"
)

func makeSineWAV16Bit(freq float64, duration float64, sampleRate int) []byte {
	numSamples := int(duration * float64(sampleRate))
	buf := make([]byte, 44+numSamples*2)

	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+numSamples*2))
	copy(buf[8:12], "WAVE")

	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1)
	binary.LittleEndian.PutUint16(buf[22:24], 1)
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(sampleRate*2))
	binary.LittleEndian.PutUint16(buf[32:34], 2)
	binary.LittleEndian.PutUint16(buf[34:36], 16)

	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(numSamples*2))

	for i := 0; i < numSamples; i++ {
		sec := float64(i) / float64(sampleRate)
		sample := math.Sin(2 * math.Pi * freq * sec)
		val := int16(sample * 32767)
		binary.LittleEndian.PutUint16(buf[44+i*2:], uint16(val))
	}

	return buf
}

func makeSineWAVFloat32(freq float64, duration float64, sampleRate int) []byte {
	numSamples := int(duration * float64(sampleRate))
	buf := make([]byte, 44+numSamples*4)

	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+numSamples*4))
	copy(buf[8:12], "WAVE")

	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 3)
	binary.LittleEndian.PutUint16(buf[22:24], 1)
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(sampleRate*4))
	binary.LittleEndian.PutUint16(buf[32:34], 4)
	binary.LittleEndian.PutUint16(buf[34:36], 32)

	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(numSamples*4))

	for i := 0; i < numSamples; i++ {
		sec := float64(i) / float64(sampleRate)
		sample := float32(math.Sin(2 * math.Pi * freq * sec))
		binary.LittleEndian.PutUint32(buf[44+i*4:], math.Float32bits(sample))
	}

	return buf
}

func makeStereoWAV16Bit(sampleRate int) []byte {
	numSamples := 100
	buf := make([]byte, 44+numSamples*4)

	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+numSamples*4))
	copy(buf[8:12], "WAVE")

	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16)
	binary.LittleEndian.PutUint16(buf[20:22], 1)
	binary.LittleEndian.PutUint16(buf[22:24], 2)
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(sampleRate*4))
	binary.LittleEndian.PutUint16(buf[32:34], 4)
	binary.LittleEndian.PutUint16(buf[34:36], 16)

	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(numSamples*4))

	for i := 0; i < numSamples; i++ {
		leftVal := int16(1000)
		rightVal := int16(-1000)
		binary.LittleEndian.PutUint16(buf[44+i*4:], uint16(leftVal))
		binary.LittleEndian.PutUint16(buf[44+i*4+2:], uint16(rightVal))
	}

	return buf
}

func TestDecodeWAV16BitPCM(t *testing.T) {
	freq := 440.0
	duration := 1.0
	sampleRate := 44100
	wavData := makeSineWAV16Bit(freq, duration, sampleRate)

	samples, sr, err := DecodeAudioData(wavData, "wav")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if sr != sampleRate {
		t.Fatalf("expected sample rate %d, got %d", sampleRate, sr)
	}
	expectedSamples := int(duration * float64(sampleRate))
	if len(samples) != expectedSamples {
		t.Fatalf("expected %d samples, got %d", expectedSamples, len(samples))
	}

	for i, s := range samples {
		sec := float64(i) / float64(sampleRate)
		expected := math.Sin(2 * math.Pi * freq * sec)
		if math.Abs(s-expected) > 0.01 {
			t.Fatalf("sample %d: expected %.4f, got %.4f", i, expected, s)
		}
	}
}

func TestDecodeWAVFloat32(t *testing.T) {
	freq := 440.0
	duration := 1.0
	sampleRate := 44100
	wavData := makeSineWAVFloat32(freq, duration, sampleRate)

	samples, sr, err := DecodeAudioData(wavData, "wav")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if sr != sampleRate {
		t.Fatalf("expected sample rate %d, got %d", sampleRate, sr)
	}
	expectedSamples := int(duration * float64(sampleRate))
	if len(samples) != expectedSamples {
		t.Fatalf("expected %d samples, got %d", expectedSamples, len(samples))
	}

	for i, s := range samples {
		sec := float64(i) / float64(sampleRate)
		expected := math.Sin(2 * math.Pi * freq * sec)
		if math.Abs(s-expected) > 0.001 {
			t.Fatalf("sample %d: expected %.4f, got %.4f", i, expected, s)
		}
	}
}

func TestDecodeWAVMixesStereoToMono(t *testing.T) {
	sampleRate := 44100
	wavData := makeStereoWAV16Bit(sampleRate)

	samples, sr, err := DecodeAudioData(wavData, "wav")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if sr != sampleRate {
		t.Fatalf("expected sample rate %d, got %d", sampleRate, sr)
	}
	if len(samples) != 100 {
		t.Fatalf("expected 100 mono samples, got %d", len(samples))
	}

	left := float64(int16(1000)) / 32768.0
	right := float64(int16(-1000)) / 32768.0
	mixed := (left + right) / 2.0
	for i, s := range samples {
		if math.Abs(s-mixed) > 0.001 {
			t.Fatalf("sample %d: expected %.6f, got %.6f", i, mixed, s)
		}
	}
}

func TestDecodeWAVDefaultFormat(t *testing.T) {
	sampleRate := 22050
	wavData := makeSineWAV16Bit(440.0, 0.5, sampleRate)

	samples, sr, err := DecodeAudioData(wavData, "")
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if sr != sampleRate {
		t.Fatalf("expected sample rate %d, got %d", sampleRate, sr)
	}
	if len(samples) == 0 {
		t.Fatal("expected non-empty samples")
	}
}

func TestDecodeWAVInvalidHeader(t *testing.T) {
	badData := []byte("not a wav file at all")
	_, _, err := DecodeAudioData(badData, "wav")
	if err == nil {
		t.Fatal("expected error for invalid WAV header")
	}
}

func TestDecodeWAVTruncated(t *testing.T) {
	_, _, err := DecodeAudioData([]byte("RIFF"), "wav")
	if err == nil {
		t.Fatal("expected error for truncated WAV")
	}
}

func TestDecodeWAVMissingDataChunk(t *testing.T) {
	wavData := make([]byte, 44)
	copy(wavData[0:4], "RIFF")
	copy(wavData[8:12], "WAVE")

	_, _, err := DecodeAudioData(wavData, "wav")
	if err == nil {
		t.Fatal("expected error for WAV without data chunk")
	}
}

func TestDecodeWebMUnsupported(t *testing.T) {
	_, _, err := DecodeAudioData([]byte("fake-webm"), "webm")
	if err == nil {
		t.Fatal("expected error for webm format")
	}
}

func TestDecodeAudioUnsupportedFormat(t *testing.T) {
	_, _, err := DecodeAudioData([]byte("data"), "mp3")
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

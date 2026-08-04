package service

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

var (
	ErrInvalidWAV   = errors.New("invalid WAV file")
	ErrNoAudioData  = errors.New("no audio data found in WAV file")
	ErrUnsupported  = errors.New("unsupported audio format")
	ErrNoDataChunk  = errors.New("missing data chunk in WAV file")
)

// DecodeAudioData decodes raw audio bytes into normalized float64 samples
// (range [-1, 1]) and the sample rate. The format parameter selects the
// decoder; an empty format defaults to WAV.
func DecodeAudioData(data []byte, format string) ([]float64, int, error) {
	switch format {
	case "", "wav":
		return decodeWAV(data)
	case "webm":
		return decodeWebM(data)
	default:
		return nil, 0, fmt.Errorf("unsupported audio format: %s", format)
	}
}

// decodeWAV parses a WAV (RIFF) file and returns mono float64 samples.
// Supports PCM 16-bit, PCM 8-bit, and IEEE float 32-bit. Stereo tracks
// are mixed down to mono.
func decodeWAV(data []byte) ([]float64, int, error) {
	if len(data) < 44 {
		return nil, 0, ErrInvalidWAV
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, 0, ErrInvalidWAV
	}

	audioFormat := 0
	numChannels := 0
	sampleRate := 0
	bitsPerSample := 0
	var audioData []byte

	offset := 12
	for offset < len(data)-8 {
		chunkID := string(data[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))

		if offset+8+chunkSize > len(data) {
			return nil, 0, ErrInvalidWAV
		}

		switch chunkID {
		case "fmt ":
			if offset+22 > len(data) {
				return nil, 0, ErrInvalidWAV
			}
			audioFormat = int(binary.LittleEndian.Uint16(data[offset+8 : offset+10]))
			numChannels = int(binary.LittleEndian.Uint16(data[offset+10 : offset+12]))
			sampleRate = int(binary.LittleEndian.Uint32(data[offset+12 : offset+16]))
			bitsPerSample = int(binary.LittleEndian.Uint16(data[offset+22 : offset+24]))
		case "data":
			audioData = data[offset+8 : offset+8+chunkSize]
		case "fact":
			// skipped; only relevant for non-PCM formats
		}

		chunkStep := 8 + chunkSize
		if chunkSize == 0 {
			chunkStep = 8
		}
		offset += chunkStep
	}

	if sampleRate == 0 {
		return nil, 0, ErrInvalidWAV
	}
	if len(audioData) == 0 {
		return nil, 0, ErrNoDataChunk
	}

	samples, err := decodeWAVSamples(audioData, audioFormat, numChannels, bitsPerSample)
	if err != nil {
		return nil, 0, err
	}

	if numChannels > 1 {
		samples = mixToMono(samples, numChannels)
	}

	return samples, sampleRate, nil
}

// decodeWAVSamples converts raw audio bytes to []float64 based on the
// WAV audio format code and bit depth.
func decodeWAVSamples(data []byte, audioFormat, numChannels, bitsPerSample int) ([]float64, error) {
	if numChannels < 1 {
		numChannels = 1
	}

	bytesPerSample := bitsPerSample / 8
	if bytesPerSample < 1 {
		bytesPerSample = 1
	}

	frameSize := numChannels * bytesPerSample
	if frameSize == 0 {
		return nil, ErrInvalidWAV
	}

	totalFrames := len(data) / frameSize
	samples := make([]float64, totalFrames*numChannels)

	switch audioFormat {
	case 1: // PCM integer
		switch bitsPerSample {
		case 16:
			for i := 0; i < totalFrames; i++ {
				for ch := 0; ch < numChannels; ch++ {
					off := i*frameSize + ch*2
					val := int16(binary.LittleEndian.Uint16(data[off : off+2]))
					samples[i*numChannels+ch] = float64(val) / 32768.0
				}
			}
		case 8:
			for i := 0; i < totalFrames; i++ {
				for ch := 0; ch < numChannels; ch++ {
					off := i*frameSize + ch
					val := data[off]
					samples[i*numChannels+ch] = (float64(val) - 128.0) / 128.0
				}
			}
		case 32:
			for i := 0; i < totalFrames; i++ {
				for ch := 0; ch < numChannels; ch++ {
					off := i*frameSize + ch*4
					val := int32(binary.LittleEndian.Uint32(data[off : off+4]))
					samples[i*numChannels+ch] = float64(val) / 2147483648.0
				}
			}
		default:
			return nil, fmt.Errorf("unsupported PCM bit depth: %d", bitsPerSample)
		}
	case 3: // IEEE float
		switch bitsPerSample {
		case 32:
			for i := 0; i < totalFrames; i++ {
				for ch := 0; ch < numChannels; ch++ {
					off := i*frameSize + ch*4
					bits := binary.LittleEndian.Uint32(data[off : off+4])
					samples[i*numChannels+ch] = float64(math.Float32frombits(bits))
				}
			}
		case 64:
			for i := 0; i < totalFrames; i++ {
				for ch := 0; ch < numChannels; ch++ {
					off := i*frameSize + ch*8
					bits := binary.LittleEndian.Uint64(data[off : off+8])
					samples[i*numChannels+ch] = math.Float64frombits(bits)
				}
			}
		default:
			return nil, fmt.Errorf("unsupported IEEE float bit depth: %d", bitsPerSample)
		}
	default:
		return nil, fmt.Errorf("unsupported WAV audio format code: %d", audioFormat)
	}

	return samples, nil
}

// mixToMono averages interleaved multi-channel samples into mono.
func mixToMono(interleaved []float64, numChannels int) []float64 {
	if numChannels <= 1 {
		return interleaved
	}
	mono := make([]float64, 0, len(interleaved)/numChannels)
	for i := 0; i < len(interleaved); i += numChannels {
		var sum float64
		end := i + numChannels
		if end > len(interleaved) {
			end = len(interleaved)
		}
		for ch := i; ch < end; ch++ {
			sum += interleaved[ch]
		}
		mono = append(mono, sum/float64(end-i))
	}
	return mono
}

// decodeWebM is a stub for WebM/Opus decoding. Without external C
// libraries (libopus), pure-Go decoding is not practical. Callers should
// prefer WAV as the recording format (see Phase 4 recommendation).
func decodeWebM(data []byte) ([]float64, int, error) {
	return nil, 0, fmt.Errorf("webm/opus decoding not available in pure Go; use WAV format instead")
}

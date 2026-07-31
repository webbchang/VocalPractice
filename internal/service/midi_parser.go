package service

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"sort"

	"vocal-practice-app/internal/domain"
)

var (
	ErrInvalidMIDI       = errors.New("invalid MIDI file")
	ErrUnsupportedFormat = errors.New("unsupported MIDI format")
)

type MIDIParser struct{}

func NewMIDIParser() *MIDIParser {
	return &MIDIParser{}
}

// TempoEntry represents a tempo change event with its tick position and the
// accumulated time in seconds at which this tempo takes effect.
type TempoEntry struct {
	Tick        int     `json:"tick"`
	TimeSec     float64 `json:"time_sec"`
	TempoUSecQN uint32  `json:"tempo_usec_per_qn"`
}

// Parse parses a Standard MIDI File (.mid) binary and returns a Song with parsed tracks.
func (p *MIDIParser) Parse(data []byte, title, artist string) (*domain.Song, error) {
	r := bytes.NewReader(data)

	header := make([]byte, 14)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, ErrInvalidMIDI
	}

	if string(header[0:4]) != "MThd" {
		return nil, ErrInvalidMIDI
	}

	headerLen := binary.BigEndian.Uint32(header[4:8])
	if headerLen < 6 {
		return nil, ErrInvalidMIDI
	}

	format := binary.BigEndian.Uint16(header[8:10])
	numTracks := binary.BigEndian.Uint16(header[10:12])
	ticksPerQuarter := int(binary.BigEndian.Uint16(header[12:14]))

	if format > 2 {
		return nil, ErrUnsupportedFormat
	}

	// Build global tempo map (skip MThd header to read tracks)
	r.Seek(14, io.SeekStart)
	globalTempoMap := buildTempoMap(r, numTracks, ticksPerQuarter)

	// Re-seek for track parsing
	r.Seek(0, io.SeekStart)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, ErrInvalidMIDI
	}
	if string(header[0:4]) != "MThd" {
		return nil, ErrInvalidMIDI
	}

	var tracks []domain.MIDITrack

	for trackIdx := uint16(0); trackIdx < numTracks; trackIdx++ {
		track, err := readTrackWithGlobalTempo(r, ticksPerQuarter, globalTempoMap)
		if err != nil {
			continue
		}
		if track != nil && len(track.Notes) > 0 {
			track.ID = domain.ComputeTrackID(track.Name, track.Channel, track.Notes)
			track.MIDIIndex = int(trackIdx)
			nameLower := toLower(track.Name)
			if contains(nameLower, "vocal") || contains(nameLower, "voice") || contains(nameLower, "lead") || contains(nameLower, "singer") || contains(nameLower, "sop") || contains(nameLower, "alt") || contains(nameLower, "ten") || contains(nameLower, "bas") {
				track.IsVocal = true
			}
			tracks = append(tracks, *track)
		}
	}

	if len(tracks) == 0 {
		return nil, ErrInvalidMIDI
	}

	// Deduplicate track names: append _2, _3, ... for duplicates
	domain.DeduplicateTrackNames(tracks)

	song := domain.NewSong(title, artist, "", tracks)
	song.TicksPerQuarter = ticksPerQuarter
	for _, te := range globalTempoMap {
		song.TempoMap = append(song.TempoMap, domain.TempoMapEntry{
			Tick:        te.Tick,
			TimeSec:     te.TimeSec,
			TempoUSecQN: te.TempoUSecQN,
		})
	}
	return song, nil
}

// ParseExtracted parses a MIDI file and returns the tracks plus the global
// tempo map and ticks-per-quarter needed for time↔tick conversion.
// This can be used by the admin UI to auto-calculate start_tick/end_tick
// from user-entered start_time/end_time.
type ParsedMIDIResult struct {
	Tracks          []domain.MIDITrack
	TempoMap        []TempoEntry
	TicksPerQuarter int
}

func (p *MIDIParser) ParseExtracted(data []byte) (*ParsedMIDIResult, error) {
	r := bytes.NewReader(data)

	header := make([]byte, 14)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, ErrInvalidMIDI
	}
	if string(header[0:4]) != "MThd" {
		return nil, ErrInvalidMIDI
	}

	headerLen := binary.BigEndian.Uint32(header[4:8])
	if headerLen < 6 {
		return nil, ErrInvalidMIDI
	}

	format := binary.BigEndian.Uint16(header[8:10])
	numTracks := binary.BigEndian.Uint16(header[10:12])
	ticksPerQuarter := int(binary.BigEndian.Uint16(header[12:14]))

	if format > 2 {
		return nil, ErrUnsupportedFormat
	}

	// Build the global tempo map from all tracks
	globalTempoMap := buildTempoMap(r, numTracks, ticksPerQuarter)

	// Reset reader and re-parse tracks with the shared global tempo map
	r.Seek(0, io.SeekStart)

	return p.parseWithTempoMap(r, ticksPerQuarter, globalTempoMap)
}

// parseWithTempoMap re-parses using a pre-built global tempo map.
func (p *MIDIParser) parseWithTempoMap(r *bytes.Reader, ticksPerQuarter int, globalTempoMap []TempoEntry) (*ParsedMIDIResult, error) {
	header := make([]byte, 14)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, ErrInvalidMIDI
	}
	if string(header[0:4]) != "MThd" {
		return nil, ErrInvalidMIDI
	}

	numTracks := binary.BigEndian.Uint16(header[10:12])

	var tracks []domain.MIDITrack

	for trackIdx := uint16(0); trackIdx < numTracks; trackIdx++ {
		track, err := readTrackWithGlobalTempo(r, ticksPerQuarter, globalTempoMap)
		if err != nil {
			continue
		}
		if track != nil && len(track.Notes) > 0 {
			track.ID = domain.ComputeTrackID(track.Name, track.Channel, track.Notes)
			track.MIDIIndex = int(trackIdx)
			nameLower := toLower(track.Name)
			if contains(nameLower, "vocal") || contains(nameLower, "voice") || contains(nameLower, "lead") || contains(nameLower, "singer") || contains(nameLower, "sop") || contains(nameLower, "alt") || contains(nameLower, "ten") || contains(nameLower, "bas") {
				track.IsVocal = true
			}
			tracks = append(tracks, *track)
		}
	}

	if len(tracks) == 0 {
		return nil, ErrInvalidMIDI
	}

	// Deduplicate track names: append _2, _3, ... for duplicates
	domain.DeduplicateTrackNames(tracks)

	return &ParsedMIDIResult{
		Tracks:          tracks,
		TempoMap:        globalTempoMap,
		TicksPerQuarter: ticksPerQuarter,
	}, nil
}

// buildTempoMap scans all tracks for tempo events and merges them into a
// single time-ordered tempo map.  The track-level internal tick counting
// is done via a lightweight scan (we ignore note events).
func buildTempoMap(r *bytes.Reader, numTracks uint16, ticksPerQuarter int) []TempoEntry {
	// Collect all raw tempo change entries (tick, tempo) from all tracks
	type rawTempo struct {
		tick        int
		tempoUSecQN uint32
	}
	var raw []rawTempo

	for range numTracks {
		// Read track header
		trackHeader := make([]byte, 8)
		if _, err := io.ReadFull(r, trackHeader); err != nil {
			continue
		}
		if string(trackHeader[0:4]) != "MTrk" {
			continue
		}
		trackLen := binary.BigEndian.Uint32(trackHeader[4:8])
		trackData := make([]byte, trackLen)
		if _, err := io.ReadFull(r, trackData); err != nil {
			continue
		}

		var absTick int
		idx := 0
		var runningStatus byte

		for idx < len(trackData) {
			delta, deltaLen := readVarLen(trackData[idx:])
			idx += deltaLen
			absTick += delta

			if idx >= len(trackData) {
				break
			}

			var status byte
			if trackData[idx] >= 0x80 {
				status = trackData[idx]
				idx++
			} else if runningStatus != 0 {
				status = runningStatus
			} else {
				idx++
				continue
			}

			if status == 0xFF {
				runningStatus = 0
				if idx >= len(trackData) {
					break
				}
				metaType := trackData[idx]
				idx++
				metaLen, metaLenSize := readVarLen(trackData[idx:])
				idx += metaLenSize
				if idx+metaLen > len(trackData) {
					break
				}
				metaData := trackData[idx : idx+metaLen]
				idx += metaLen

				if metaType == 0x51 && metaLen >= 3 {
					tempo := uint32(metaData[0])<<16 | uint32(metaData[1])<<8 | uint32(metaData[2])
					raw = append(raw, rawTempo{tick: absTick, tempoUSecQN: tempo})
				}
				continue
			}

			if status == 0xF0 || status == 0xF7 {
				runningStatus = 0
				sysexLen, sysexLenSize := readVarLen(trackData[idx:])
				idx += sysexLenSize + sysexLen
				continue
			}

			runningStatus = status
			highNibble := status & 0xF0

			switch highNibble {
			case 0x80, 0x90, 0xA0, 0xB0, 0xE0:
				idx += 2
			case 0xC0, 0xD0:
				idx += 1
			default:
			}
		}
	}

	// Sort by tick
	sort.Slice(raw, func(i, j int) bool {
		return raw[i].tick < raw[j].tick
	})

	// De-duplicate: keep first tempo at each tick
	seen := make(map[int]bool)
	var unique []rawTempo
	for _, r := range raw {
		if !seen[r.tick] {
			seen[r.tick] = true
			unique = append(unique, r)
		}
	}

	// Convert to TempoEntry with accumulated time
	ppq := float64(ticksPerQuarter)
	var entries []TempoEntry
	var prevTick int
	var prevTime float64
	var prevTempo uint32 = 500000

	if len(unique) > 0 && unique[0].tick > 0 {
		// Add implicit initial tempo
		entries = append(entries, TempoEntry{
			Tick:        0,
			TimeSec:     0,
			TempoUSecQN: prevTempo,
		})
	}

	for _, rt := range unique {
		deltaTick := rt.tick - prevTick
		secPerTick := float64(prevTempo) / (ppq * 1000000.0)
		deltaSec := float64(deltaTick) * secPerTick
		timeSec := prevTime + deltaSec

		entries = append(entries, TempoEntry{
			Tick:        rt.tick,
			TimeSec:     timeSec,
			TempoUSecQN: rt.tempoUSecQN,
		})

		prevTick = rt.tick
		prevTime = timeSec
		prevTempo = rt.tempoUSecQN
	}

	return entries
}

// SecToTick converts a time in seconds to the nearest tick using the tempo map.
// It interpolates within the appropriate tempo segment.
func SecToTick(timeSec float64, tempoMap []TempoEntry, ticksPerQuarter int) int {
	if len(tempoMap) == 0 {
		// Default: 120 BPM (500000 µs/qn), ppq=480
		ppq := float64(ticksPerQuarter)
		if ppq == 0 {
			ppq = 480
		}
		secPerTick := float64(500000) / (ppq * 1000000.0)
		return int(math.Round(timeSec / secPerTick))
	}

	ppq := float64(ticksPerQuarter)
	if ppq == 0 {
		ppq = 480
	}

	// Find the tempo segment containing timeSec
	i := sort.Search(len(tempoMap), func(i int) bool {
		return tempoMap[i].TimeSec > timeSec
	}) - 1

	if i < 0 {
		i = 0
	}
	if i >= len(tempoMap) {
		i = len(tempoMap) - 1
	}

	entry := tempoMap[i]
	secPerTick := float64(entry.TempoUSecQN) / (ppq * 1000000.0)
	deltaSec := timeSec - entry.TimeSec
	deltaTick := deltaSec / secPerTick
	tick := float64(entry.Tick) + deltaTick
	if tick < 0 {
		tick = 0
	}
	return int(math.Round(tick))
}

// TickToSec converts a tick position to seconds using the global tempo map.
// It finds the appropriate tempo segment and interpolates within it.
func TickToSec(tick int, tempoMap []TempoEntry, ticksPerQuarter int) float64 {
	ppq := float64(ticksPerQuarter)
	if ppq == 0 {
		ppq = 480
	}
	if len(tempoMap) == 0 {
		secPerTick := float64(500000) / (ppq * 1000000.0)
		return float64(tick) * secPerTick
	}

	// Find the tempo segment containing this tick
	i := sort.Search(len(tempoMap), func(i int) bool {
		return tempoMap[i].Tick > tick
	}) - 1

	if i < 0 {
		i = 0
	}
	if i >= len(tempoMap) {
		i = len(tempoMap) - 1
	}

	entry := tempoMap[i]
	secPerTick := float64(entry.TempoUSecQN) / (ppq * 1000000.0)
	deltaTick := float64(tick - entry.Tick)
	timeSec := entry.TimeSec + deltaTick*secPerTick
	return math.Round(timeSec*100) / 100
}

// readTrackWithGlobalTempo reads a single MIDI track, tracking absolute ticks
// and converting them to seconds using the global tempo map.
// This ensures all tracks use consistent tempo timing regardless of which track
// contains the tempo change events.
func readTrackWithGlobalTempo(r io.Reader, ticksPerQuarter int, globalTempoMap []TempoEntry) (*domain.MIDITrack, error) {
	trackHeader := make([]byte, 8)
	if _, err := io.ReadFull(r, trackHeader); err != nil {
		return nil, err
	}

	if string(trackHeader[0:4]) != "MTrk" {
		return nil, errors.New("not a track chunk")
	}

	trackLen := binary.BigEndian.Uint32(trackHeader[4:8])
	trackData := make([]byte, trackLen)
	if _, err := io.ReadFull(r, trackData); err != nil {
		return nil, err
	}

	track := &domain.MIDITrack{
		Name:    "",
		Notes:   []domain.MIDINote{},
		Channel: 0,
	}

	type pendingNote struct {
		pitch    int
		velocity int
		start    float64
	}

	pending := make(map[int]*pendingNote)
	var absTick int
	idx := 0
	hasNote := false

	var runningStatus byte

	for idx < len(trackData) {
		delta, deltaLen := readVarLen(trackData[idx:])
		idx += deltaLen
		absTick += delta

		// Convert absolute tick to seconds using the global tempo map.
		absTimeSec := TickToSec(absTick, globalTempoMap, ticksPerQuarter)

		if idx >= len(trackData) {
			break
		}

		// Determine status byte.
		var status byte
		if trackData[idx] >= 0x80 {
			status = trackData[idx]
			idx++
		} else if runningStatus != 0 {
			status = runningStatus
		} else {
			idx++
			continue
		}

		if status == 0xFF {
			// Meta event: always consumes a full status byte.
			runningStatus = 0 // meta resets running status
			if idx >= len(trackData) {
				break
			}
			metaType := trackData[idx]
			idx++
			metaLen, metaLenSize := readVarLen(trackData[idx:])
			idx += metaLenSize
			if idx+metaLen > len(trackData) {
				break
			}
			metaData := trackData[idx : idx+metaLen]
			idx += metaLen

			switch metaType {
			case 0x03:
				track.Name = string(metaData)
			}
			continue
		}

		if status == 0xF0 || status == 0xF7 {
			// System exclusive — resets running status
			runningStatus = 0
			sysexLen, sysexLenSize := readVarLen(trackData[idx:])
			idx += sysexLenSize + sysexLen
			continue
		}

		// Channel voice message: update running status
		runningStatus = status
		highNibble := status & 0xF0
		channel := int(status & 0x0F)

		if highNibble == 0x90 || highNibble == 0x80 {
			if idx+1 >= len(trackData) {
				break
			}
			pitch := int(trackData[idx])
			velocity := int(trackData[idx+1])
			idx += 2

			if highNibble == 0x90 && velocity > 0 {
				pending[pitch] = &pendingNote{
					pitch:    pitch,
					velocity: velocity,
					start:    absTimeSec,
				}
				track.Channel = channel
				hasNote = true
			} else {
				if p, ok := pending[pitch]; ok {
					note := domain.MIDINote{
						Pitch:     p.pitch,
						Velocity:  p.velocity,
						StartTime: math.Round(p.start*100) / 100,
						EndTime:   math.Round(absTimeSec*100) / 100,
					}
					if note.EndTime > note.StartTime {
						track.Notes = append(track.Notes, note)
					}
					delete(pending, pitch)
				}
			}
			continue
		}

		switch highNibble {
		case 0xA0, 0xB0, 0xE0:
			idx += 2
		case 0xC0, 0xD0:
			idx += 1
		default:
		}
	}

	// Handle pending (unclosed) notes
	finalTimeSec := TickToSec(absTick, globalTempoMap, ticksPerQuarter)
	for _, p := range pending {
		note := domain.MIDINote{
			Pitch:     p.pitch,
			Velocity:  p.velocity,
			StartTime: math.Round(p.start*100) / 100,
			EndTime:   math.Round(finalTimeSec*100) / 100,
		}
		if note.EndTime > note.StartTime {
			track.Notes = append(track.Notes, note)
		}
	}

	if !hasNote {
		return nil, nil
	}

	return track, nil
}

func readVarLen(data []byte) (int, int) {
	var value int
	var size int
	for i := 0; i < len(data); i++ {
		b := data[i]
		value = (value << 7) | int(b&0x7F)
		size++
		if b&0x80 == 0 {
			break
		}
	}
	return value, size
}

func tickToSec(tick float64, usPerQN uint32, ppq float64) float64 {
	secPerTick := float64(usPerQN) / (ppq * 1000000.0)
	return float64(tick) * secPerTick
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}


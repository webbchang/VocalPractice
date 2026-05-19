package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"vocal-practice-app/internal/service"
)

type clientNote struct {
	pitch int
	start float64
	end   float64
	dur   float64
}

func main() {
	midiPath := filepath.Join("test_data", "reference2.MID")

	raw, err := os.ReadFile(midiPath)
	if err != nil {
		panic(err)
	}

	// ========================================================
	// 1) Server-side parser (tempo-aware)
	// ========================================================
	parser := service.NewMIDIParser()
	song, err := parser.Parse(raw, "reference2", "")
	if err != nil {
		panic(fmt.Sprintf("Server parse error: %v", err))
	}

	fmt.Println("=== Server-side MIDIParser (tempo-aware) ===")
	fmt.Printf("TicksPerQuarter: %d\n", song.TicksPerQuarter)
	fmt.Printf("Tempo map (%d entries):\n", len(song.TempoMap))
	for i, te := range song.TempoMap {
		bpm := 60000000.0 / float64(te.TempoUSecQN)
		fmt.Printf("  [%d] tick=%d time_sec=%.4f tempo=%d us/qn (%.1f BPM)\n", i, te.Tick, te.TimeSec, te.TempoUSecQN, bpm)
	}

	for _, t := range song.Tracks {
		fmt.Printf("\nTrack: %q (isVocal=%v, %d notes)\n", t.Name, t.IsVocal, len(t.Notes))
		if len(t.Notes) > 0 {
			first := t.Notes[0]
			last := t.Notes[len(t.Notes)-1]
			fmt.Printf("  Range: %.4f - %.4f (duration %.4f)\n", first.StartTime, last.EndTime, last.EndTime-first.StartTime)

			// Check for notes near target times
			for _, n := range t.Notes {
				s := n.StartTime
				if (s >= 14 && s <= 19) || (s >= 48 && s <= 53) {
					fmt.Printf("  > pitch=%d start=%.4f end=%.4f dur=%.4f\n", n.Pitch, s, n.EndTime, n.EndTime-n.StartTime)
				}
			}
		}
	}

	// ========================================================
	// 2) Client-side style parser (fixed 120 BPM = 500000 us/qn)
	//    This is what user-practice.html's parseMIDINotes() does
	// ========================================================
	fmt.Println("\n\n=== Client-side parser (fixed 120 BPM, NO tempo tracking) ===")
	clientNotes := parseClientStyle(raw)
	fmt.Printf("Parsed %d notes\n", len(clientNotes))
	if len(clientNotes) > 0 {
		fmt.Printf("  Range: %.4f - %.4f (duration %.4f)\n", clientNotes[0].start, clientNotes[len(clientNotes)-1].end, clientNotes[len(clientNotes)-1].end-clientNotes[0].start)
		for _, n := range clientNotes {
			s := n.start
			if (s >= 14 && s <= 19) || (s >= 48 && s <= 53) {
				fmt.Printf("  > pitch=%d start=%.4f end=%.4f dur=%.4f\n", n.pitch, s, n.end, n.dur)
			}
		}
	}

	// ========================================================
	// 3) Compare: find note at tick X under both schemes
	// ========================================================
	fmt.Println("\n\n=== Tick-to-Time comparison ===")
	ppq := float64(song.TicksPerQuarter)
	var tempoMap []service.TempoEntry
	for _, te := range song.TempoMap {
		tempoMap = append(tempoMap, service.TempoEntry{
			Tick:        te.Tick,
			TimeSec:     te.TimeSec,
			TempoUSecQN: te.TempoUSecQN,
		})
	}

	// Use SecToTick from server parser
	testTimes := []float64{0, 5, 10, 16.5, 20, 30, 40, 50.6, 60}
	fmt.Println("\nServer-side SecToTick:")
	for _, sec := range testTimes {
		tick := service.SecToTick(sec, tempoMap, song.TicksPerQuarter)
		fmt.Printf("  %.1f sec -> %d ticks\n", sec, tick)
	}

	// Client-side equivalent (fixed 120 BPM)
	fmt.Println("\nClient-side fixed-120BPM time->tick:")
	const defaultTempo = 500000
	secPerTick := float64(defaultTempo) / (ppq * 1000000.0)
	for _, sec := range testTimes {
		tick := int(math.Round(sec / secPerTick))
		fmt.Printf("  %.1f sec -> %d ticks\n", sec, tick)
	}

	// Show the tick difference as a percentage
	fmt.Println("\nTick ratio (client/server):")
	for _, sec := range testTimes {
		serverTick := service.SecToTick(sec, tempoMap, song.TicksPerQuarter)
		clientTick := int(math.Round(sec / secPerTick))
		if serverTick > 0 {
			ratio := float64(clientTick) / float64(serverTick)
			fmt.Printf("  %.1f sec: server=%d, client=%d, ratio=%.4f\n", sec, serverTick, clientTick, ratio)
		}
	}

	// ========================================================
	// 4) Raw MIDI dump: find tempo events
	// ========================================================
	fmt.Println("\n\n=== Raw MIDI tempo events ===")
	dumpTempoEvents(raw)
}

func parseClientStyle(data []byte) []clientNote {
	head := string(data[0:4])
	if head != "MThd" {
		return nil
	}
	numTracks := binary.BigEndian.Uint16(data[10:12])
	ticksPerQN := binary.BigEndian.Uint16(data[12:14])

	// Fixed tempo: 500000 us/qn = 120 BPM
	const usPerQN = 500000.0
	secPerTick := usPerQN / (float64(ticksPerQN) * 1000000.0)

	type rawNote struct {
		pitch   int
		tick    int
		endTick int
		track   int
	}

	var allNotes []rawNote
	offset := 14

	for t := 0; t < int(numTracks); t++ {
		if offset+8 > len(data) {
			break
		}
		chunkID := string(data[offset : offset+4])
		if chunkID != "MTrk" {
			break
		}
		chunkLen := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))
		offset += 8
		end := offset + chunkLen

		absTick := 0
		runningStatus := 0

		for offset < end {
			delta, deltaSize := readVarLen(data[offset:])
			offset += deltaSize
			absTick += delta

			if offset >= end {
				break
			}

			status := int(data[offset])
			if status >= 0x80 {
				runningStatus = status
				offset++
			} else if runningStatus != 0 {
				status = runningStatus
			} else {
				offset++
				continue
			}

			if status == 0xFF {
				runningStatus = 0
				if offset >= end {
					break
				}
				offset++ // metaType
				metaLen, metaLenSize := readVarLen(data[offset:])
				offset += metaLenSize + metaLen
				continue
			}
			if status == 0xF0 || status == 0xF7 {
				runningStatus = 0
				sysexLen, sysexLenSize := readVarLen(data[offset:])
				offset += sysexLenSize + sysexLen
				continue
			}

			highNibble := status & 0xF0
			if highNibble == 0x90 || highNibble == 0x80 {
				pitch := int(data[offset])
				velocity := int(data[offset+1])
				offset += 2

				if highNibble == 0x90 && velocity > 0 {
					allNotes = append(allNotes, rawNote{pitch: pitch, tick: absTick, track: t})
				} else {
					for i := len(allNotes) - 1; i >= 0; i-- {
						if allNotes[i].pitch == pitch && allNotes[i].endTick == 0 && allNotes[i].track == t {
							allNotes[i].endTick = absTick
							break
						}
					}
				}
				continue
			}

			switch highNibble {
			case 0xA0, 0xB0, 0xE0:
				offset += 2
			case 0xC0, 0xD0:
				offset += 1
			default:
			}
		}
	}

	var result []clientNote
	for _, n := range allNotes {
		if n.endTick > n.tick {
			start := float64(n.tick) * secPerTick
			end := float64(n.endTick) * secPerTick
			dur := end - start
			if dur > 0.02 {
				result = append(result, clientNote{
					pitch: n.pitch,
					start: math.Round(start*100) / 100,
					end:   math.Round(end*100) / 100,
					dur:   dur,
				})
			}
		}
	}

	// Match rounding style of client parser
	sort.Slice(result, func(i, j int) bool {
		return result[i].start < result[j].start
	})
	return result
}

func dumpTempoEvents(data []byte) {
	if len(data) < 14 {
		return
	}
	head := string(data[0:4])
	if head != "MThd" {
		return
	}
	numTracks := binary.BigEndian.Uint16(data[10:12])

	offset := 14

	for t := 0; t < int(numTracks); t++ {
		if offset+8 > len(data) {
			break
		}
		chunkID := string(data[offset : offset+4])
		if chunkID != "MTrk" {
			break
		}
		chunkLen := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))
		offset += 8
		end := offset + chunkLen

		absTick := 0
		tempoCount := 0

		for offset < end {
			delta, deltaSize := readVarLen(data[offset:])
			offset += deltaSize
			absTick += delta

			if offset >= end {
				break
			}

			status := int(data[offset])
			if status >= 0x80 {
				offset++
			} else {
				offset++
				continue
			}

			if status != 0xFF {
				continue
			}

			if offset >= end {
				break
			}
			metaType := int(data[offset])
			offset++
			metaLen, metaLenSize := readVarLen(data[offset:])
			offset += metaLenSize

			if metaType == 0x51 && metaLen >= 3 && offset+metaLen <= len(data) {
				tempo := uint32(data[offset])<<16 | uint32(data[offset+1])<<8 | uint32(data[offset+2])
				bpm := 60000000.0 / float64(tempo)
				fmt.Printf("  Track %d: tick=%d tempo=%d us/qn (%.1f BPM)\n", t, absTick, tempo, bpm)
				tempoCount++
			}
			offset += metaLen
		}

		if tempoCount == 0 {
			fmt.Printf("  Track %d: (no tempo events)\n", t)
		}
	}
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
	if len(s) < len(substr) {
		return false
	}
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

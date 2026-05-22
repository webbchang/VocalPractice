package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"vocal-practice-app/internal/domain"
	"vocal-practice-app/internal/service"
)

type clientNote struct {
	pitch int
	start float64
	end   float64
	dur   float64
}

type rawNote struct {
	pitch   int
	tick    int
	endTick int
	track   int
}

type tempoSeg struct {
	tick    int
	timeSec float64
	usPerQN uint32
}

type clientNoteWithTrack struct {
	pitch int
	start float64
	end   float64
	dur   float64
	track int
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
		fmt.Printf("\nTrack: %q (isVocal=%v, midi_index=%d, %d notes)\n", t.Name, t.IsVocal, t.MIDIIndex, len(t.Notes))
		if len(t.Notes) > 0 {
			first := t.Notes[0]
			last := t.Notes[len(t.Notes)-1]
			fmt.Printf("  Range: %.4f - %.4f (duration %.4f)\n", first.StartTime, last.EndTime, last.EndTime-first.StartTime)

			for _, n := range t.Notes {
				s := n.StartTime
				if (s >= 14 && s <= 19) || (s >= 48 && s <= 53) {
					fmt.Printf("  > pitch=%d start=%.4f end=%.4f dur=%.4f\n", n.Pitch, s, n.EndTime, n.EndTime-n.StartTime)
				}
			}
		}
	}

	// ========================================================
	// 2) Client-side parser (tempo-aware, matching actual midiParser.js)
	//
	//    The actual JS MidiParser.parseMIDINotes() in assets/js/midiParser.js
	//    does a TWO-PASS parse:
	//      Pass 1: scan all tracks for Set Tempo (0xFF 0x51) events to build
	//              a global tempo map (sorted, deduplicated, accumulated time)
	//      Pass 2: parse all note on/off events, converting ticks to seconds
	//              via tickToSec() using the global tempo map
	//
	//    This is functionally IDENTICAL to the Go server-side approach.
	//    The old comment saying "client uses fixed 120 BPM" was WRONG.
	// ========================================================
	fmt.Println("\n\n=== Client-side parser (tempo-aware, matching actual midiParser.js) ===")
	clientNotes := parseClientStyleActual(raw)
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
	// 3) Server vs Client: note-level time comparison
	//
	//    Both parsers use tempo-aware tick-to-second conversion,
	//    so time values should match. We compare per-track.
	// ========================================================
	fmt.Println("\n\n=== Server vs Client: note-level time comparison ===")

	// Group client notes by track index (client parser preserves track index)
	clientByTrack := make(map[int][]clientNote)
	for _, cn := range clientNotes {
		cn := cn
		// We need track index — let's track it via rawNote.
		// Actually the parseClientStyleActual returns clientNote which doesn't
		// have track. Let's use a parallel slice.
		_ = cn
	}
	_ = clientByTrack

	// Re-parse with track info
	clientNotesWithTrack := parseClientStyleActualWithTrack(raw)

	// For each server track, find matching client notes
	for _, t := range song.Tracks {
		serverNotes := t.Notes
		var matchingClient []clientNoteWithTrack
		for _, cn := range clientNotesWithTrack {
			if cn.track == t.MIDIIndex {
				matchingClient = append(matchingClient, cn)
			}
		}

		fmt.Printf("\n  Track %d (%q): server=%d notes, client=%d notes\n", t.MIDIIndex, t.Name, len(serverNotes), len(matchingClient))

		// Sample comparison in 14-19s window
		var serverInWindow []struct {
			pitch int
			start float64
			end   float64
		}
		for _, n := range serverNotes {
			if n.StartTime >= 14 && n.StartTime <= 19 {
				serverInWindow = append(serverInWindow, struct {
					pitch int
					start float64
					end   float64
				}{pitch: n.Pitch, start: n.StartTime, end: n.EndTime})
			}
		}
		var clientInWindow []clientNoteWithTrack
		for _, cn := range matchingClient {
			if cn.start >= 14 && cn.start <= 19 {
				clientInWindow = append(clientInWindow, cn)
			}
		}

		fmt.Printf("    Window [14-19s]: server=%d notes, client=%d notes\n", len(serverInWindow), len(clientInWindow))

		// Show first 5 matching pairs (approximately matched by pitch + time)
		matched := 0
		for _, sn := range serverInWindow {
			for _, cn := range clientInWindow {
				if cn.pitch == sn.pitch && math.Abs(cn.start-sn.start) < 0.1 && math.Abs(cn.end-sn.end) < 0.1 {
					if matched < 5 {
						fmt.Printf("    ✓ pitch=%d server=(%.4f,%.4f) client=(%.4f,%.4f) diff=(%.4f,%.4f)\n",
							sn.pitch, sn.start, sn.end, cn.start, cn.end, cn.start-sn.start, cn.end-sn.end)
					}
					matched++
					break
				}
			}
		}
		fmt.Printf("    Matched notes in window: %d / %d\n", matched, len(serverInWindow))
	}

	// Show tick↔time roundtrip consistency
	fmt.Println("\n  Tick↔Time roundtrip consistency:")
	tempoMap := toServiceTempoMap(song.TempoMap)
	testTicks := []int{0, 480, 960, 1920, 3840, 7680, 15360}
	for _, tick := range testTicks {
		sec := service.TickToSec(tick, tempoMap, song.TicksPerQuarter)
		tickBack := service.SecToTick(sec, tempoMap, song.TicksPerQuarter)
		fmt.Printf("    tick=%d -> sec=%.4f -> tick=%d (diff=%d)\n", tick, sec, tickBack, tickBack-tick)
	}

	// ========================================================
	// 4) Raw MIDI dump: find tempo events
	// ========================================================
	fmt.Println("\n\n=== Raw MIDI tempo events ===")
	dumpTempoEvents(raw)
}

// toServiceTempoMap converts domain TempoMapEntry to service.TempoEntry.
func toServiceTempoMap(entries []domain.TempoMapEntry) []service.TempoEntry {
	var result []service.TempoEntry
	for _, e := range entries {
		result = append(result, service.TempoEntry{
			Tick:        e.Tick,
			TimeSec:     e.TimeSec,
			TempoUSecQN: e.TempoUSecQN,
		})
	}
	return result
}

// parseClientStyleActual replicates the JS MidiParser.parseMIDINotes() algorithm:
//
//	Pass 1: Build global tempo map from all tracks (scan 0xFF 0x51 events)
//	Pass 2: Parse notes, converting ticks to seconds via tickToSec() using tempo map
//
// Returns flattened notes (no track index). For track-indexed output,
// use parseClientStyleActualWithTrack.
func parseClientStyleActual(data []byte) []clientNote {
	notes := parseClientStyleActualWithTrack(data)
	var result []clientNote
	for _, n := range notes {
		result = append(result, clientNote{
			pitch: n.pitch,
			start: n.start,
			end:   n.end,
			dur:   n.dur,
		})
	}
	return result
}

// parseClientStyleActualWithTrack is like parseClientStyleActual but preserves
// the track index from MIDI parsing.
func parseClientStyleActualWithTrack(data []byte) []clientNoteWithTrack {
	if len(data) < 14 {
		return nil
	}
	head := string(data[0:4])
	if head != "MThd" {
		return nil
	}
	numTracks := binary.BigEndian.Uint16(data[10:12])
	ticksPerQN := int(binary.BigEndian.Uint16(data[12:14]))

	// ── Pass 1: Build global tempo map ──
	tempoSegs := buildTempoSegments(data, ticksPerQN)

	// ── Pass 2: Parse notes ──
	offset := 14

	var allNotes []rawNote

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

	// Convert ticks to seconds using the global tempo map (same as JS tickToSec)
	var result []clientNoteWithTrack
	for _, n := range allNotes {
		if n.endTick > n.tick {
			start := tickToSecJS(n.tick, tempoSegs, ticksPerQN)
			end := tickToSecJS(n.endTick, tempoSegs, ticksPerQN)
			dur := end - start
			if dur > 0.02 {
				result = append(result, clientNoteWithTrack{
					pitch: n.pitch,
					start: math.Round(start*100) / 100,
					end:   math.Round(end*100) / 100,
					dur:   dur,
					track: n.track,
				})
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].start < result[j].start
	})
	return result
}

// tickToSecJS replicates the JS tickToSec() function from midiParser.js.
//
//	JS logic:
//	  find i = last tempo segment with tick <= given tick
//	  walk from seg 1..i accumulating time
//	  then add delta from seg i's tick to given tick
func tickToSecJS(tick int, segs []tempoSeg, ticksPerQN int) float64 {
	ppq := float64(ticksPerQN)
	if len(segs) == 0 {
		// Default 120 BPM
		secPerTick := 500000.0 / (ppq * 1000000.0)
		return float64(tick) * secPerTick
	}

	// Find the last segment with tick <= given tick (same as JS loop)
	i := 0
	for j := len(segs) - 1; j >= 0; j-- {
		if segs[j].tick <= tick {
			i = j
			break
		}
	}

	// Walk from seg 1 to i accumulating time (same as JS)
	timeSec := 0.0
	prevTick := 0
	prevTempo := segs[0].usPerQN // JS: tempoMap.length > 0 ? tempoMap[0].usPerQN : 500000
	for k := 1; k <= i; k++ {
		dt := segs[k].tick - prevTick
		timeSec += float64(dt) * float64(prevTempo) / (ppq * 1000000.0)
		prevTick = segs[k].tick
		prevTempo = segs[k].usPerQN
	}

	// Add remaining delta from seg i's tick to target tick
	dt := tick - prevTick
	timeSec += float64(dt) * float64(prevTempo) / (ppq * 1000000.0)
	return timeSec
}

// buildTempoSegments replicates the JS MidiParser's tempo map building logic.
// First pass: scan all tracks for 0xFF 0x51 (Set Tempo) events.
// Returns sorted, deduplicated tempo segments with precomputed accumulated time.
func buildTempoSegments(data []byte, ticksPerQN int) []tempoSeg {
	if len(data) < 14 {
		return nil
	}
	if string(data[0:4]) != "MThd" {
		return nil
	}
	numTracks := binary.BigEndian.Uint16(data[10:12])

	offset := 14
	type rawTempo struct {
		tick    int
		usPerQN uint32
	}
	var rawTempos []rawTempo

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
				metaType := int(data[offset])
				offset++
				metaLen, metaLenSize := readVarLen(data[offset:])
				offset += metaLenSize
				if metaType == 0x51 && metaLen >= 3 && offset+metaLen <= len(data) {
					tempo := uint32(data[offset])<<16 | uint32(data[offset+1])<<8 | uint32(data[offset+2])
					rawTempos = append(rawTempos, rawTempo{tick: absTick, usPerQN: tempo})
				}
				offset += metaLen
				continue
			}
			if status == 0xF0 || status == 0xF7 {
				runningStatus = 0
				sysexLen, sysexLenSize := readVarLen(data[offset:])
				offset += sysexLenSize + sysexLen
				continue
			}

			highNibble := status & 0xF0
			switch highNibble {
			case 0xA0, 0xB0, 0xE0:
				offset += 2
			case 0xC0, 0xD0:
				offset += 1
			default:
			}
		}
	}

	// Sort by tick (same as JS: rawTempos.sort((a,b) => a.tick - b.tick))
	sort.Slice(rawTempos, func(i, j int) bool {
		return rawTempos[i].tick < rawTempos[j].tick
	})

	// Deduplicate: keep first at each tick (same as JS: if last.tick !== t.tick)
	seen := make(map[int]bool)
	var unique []rawTempo
	for _, rt := range rawTempos {
		if !seen[rt.tick] {
			seen[rt.tick] = true
			unique = append(unique, rt)
		}
	}

	// Build tempoSegments — same logic as the Go server's buildTempoMap
	ppq := float64(ticksPerQN)
	var segs []tempoSeg
	var prevTick int
	var prevTime float64
	var prevTempo uint32 = 500000 // Default 120 BPM

	if len(unique) > 0 && unique[0].tick > 0 {
		// Add implicit initial tempo at tick 0 (same as Go server)
		segs = append(segs, tempoSeg{
			tick:    0,
			timeSec: 0,
			usPerQN: prevTempo,
		})
	}

	for _, rt := range unique {
		deltaTick := rt.tick - prevTick
		secPerTick := float64(prevTempo) / (ppq * 1000000.0)
		deltaSec := float64(deltaTick) * secPerTick
		prevTime += deltaSec

		segs = append(segs, tempoSeg{
			tick:    rt.tick,
			timeSec: prevTime,
			usPerQN: rt.usPerQN,
		})

		prevTick = rt.tick
		prevTempo = rt.usPerQN
	}

	return segs
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

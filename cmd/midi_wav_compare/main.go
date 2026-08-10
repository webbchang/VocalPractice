package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"vocal-practice-app/internal/domain"
	"vocal-practice-app/internal/service"
)

// ---- 複製自 internal/service/pitch.go 的 detectPitchGo（unexported，無法外部呼叫）----

type DetectedNote struct {
	Pitch          int
	PitchFloat     float64
	StartPitchFloat float64
	StartTime      float64
	EndTime        float64
}

func detectPitch(samples []float64, sampleRate int) []DetectedNote {
	windowSize := 2048
	hopSize := int(float64(sampleRate) * 0.05) // 50ms
	silenceThreshold := 0.01
	correlationThreshold := 0.1
	minFreq := 50.0
	maxFreq := 2000.0

	var notes []DetectedNote
	var currentNote *DetectedNote

	for start := 0; start+windowSize < len(samples); start += hopSize {
		end := start + windowSize
		if end > len(samples) {
			end = len(samples)
		}
		window := samples[start:end]

		var rms float64
		for _, s := range window {
			rms += s * s
		}
		rms = math.Sqrt(rms / float64(len(window)))

		if rms < float64(silenceThreshold) {
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
			continue
		}

		time := float64(start) / float64(sampleRate)

		// 診斷：輸出 4.5~6.7s 區間內每個 window 的 pitchFloat
		if time >= 4.5 && time <= 6.7 {
			fmt.Printf("  [window] t=%.2f pitchFloat=%.2f rounded=%d\n", time, midiNote, roundedPitch)
		}

		if currentNote == nil {
			currentNote = &DetectedNote{
				Pitch:          roundedPitch,
				PitchFloat:     midiNote,
				StartPitchFloat: midiNote,
				StartTime:      time,
				EndTime:        time + float64(hopSize)/float64(sampleRate),
			}
		} else {
			// 合併門檻：距離音符起始 pitchFloat 的累積漂移 < 0.99 才合併
			// (避免相鄰 window 微小漂移導致誤合併跨音符不分的情況)
			if math.Abs(currentNote.StartPitchFloat-midiNote) >= 0.99 {
				currentNote.EndTime = time
				notes = append(notes, *currentNote)
				currentNote = &DetectedNote{
					Pitch:          roundedPitch,
					PitchFloat:     midiNote,
					StartPitchFloat: midiNote,
					StartTime:      time,
					EndTime:        time + float64(hopSize)/float64(sampleRate),
				}
			} else {
				currentNote.EndTime = time + float64(hopSize)/float64(sampleRate)
				currentNote.PitchFloat = (currentNote.PitchFloat + midiNote) / 2
				// 合併後用 PitchFloat 重新計算 Pitch
				currentNote.Pitch = int(math.Round(currentNote.PitchFloat))
			}
		}
	}

	if currentNote != nil {
		notes = append(notes, *currentNote)
	}

	// Filter short notes
	var filtered []DetectedNote
	for _, n := range notes {
		if n.EndTime-n.StartTime > 0.1 {
			filtered = append(filtered, n)
		}
	}

	return filtered
}

// ---- 比對邏輯（複製 compareMergedNotesForAssessment 核心）----

type NoteComparison struct {
	RefPitch             int     `json:"ref_pitch"`
	UserPitch            int     `json:"user_pitch"`
	RefStart             float64 `json:"ref_start"`
	RefEnd               float64 `json:"ref_end"`
	UserStart            float64 `json:"user_start"`
	UserEnd              float64 `json:"user_end"`
	PitchDeviationCents  float64 `json:"pitch_deviation_cents"`
	DurationDeviationSec float64 `json:"duration_deviation_sec"`
	MatchStatus          string  `json:"match_status"`
	MissReason           string  `json:"miss_reason,omitempty"`
	DetectedInRange      []DetectedInRange `json:"detected_in_range,omitempty"`
}

type DetectedInRange struct {
	Pitch      int     `json:"pitch"`
	PitchFloat float64 `json:"pitch_float"`
	StartTime  float64 `json:"start_time"`
	EndTime    float64 `json:"end_time"`
}

type MergedRef struct {
	Pitch      int
	PitchFloat float64
	StartTime  float64
	EndTime    float64
	EventIdx   []int
}

func mergeRefs(refs []domain.MIDINote) []MergedRef {
	if len(refs) == 0 {
		return nil
	}
	sorted := make([]domain.MIDINote, len(refs))
	copy(sorted, refs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartTime < sorted[j].StartTime
	})

	sortedToOrig := make([]int, len(sorted))
	for i, s := range sorted {
		for j, o := range refs {
			if s.StartTime == o.StartTime && s.EndTime == o.EndTime && s.Pitch == o.Pitch {
				sortedToOrig[i] = j
				break
			}
		}
	}

	var merged []MergedRef
	current := MergedRef{
		Pitch:      sorted[0].Pitch,
		PitchFloat: float64(sorted[0].Pitch),
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
			totalWeight := float64(len(current.EventIdx))
			current.PitchFloat = (current.PitchFloat*totalWeight + float64(n.Pitch)) / (totalWeight + 1)
			current.EventIdx = append(current.EventIdx, sortedToOrig[i])
		} else {
			merged = append(merged, current)
			current = MergedRef{
				Pitch:      n.Pitch,
				PitchFloat: float64(n.Pitch),
				StartTime:  n.StartTime,
				EndTime:    n.EndTime,
				EventIdx:   []int{sortedToOrig[i]},
			}
		}
	}
	merged = append(merged, current)
	return merged
}

// compareNotes 比對參考與偵測音符，回傳每個參考音符的比對結果
// overlapThreshold: 重疊門檻（0~1）
// pitchTolerance: 音高容差（semitone）
func compareNotes(refs []domain.MIDINote, detected []DetectedNote, offset float64, overlapThreshold float64, pitchTolerance int) []NoteComparison {
	// 套用 offset 到偵測音符
	detShifted := make([]DetectedNote, len(detected))
	for i, d := range detected {
		detShifted[i] = DetectedNote{
			Pitch:      d.Pitch,
			PitchFloat: d.PitchFloat,
			StartTime:  d.StartTime + offset,
			EndTime:    d.EndTime + offset,
		}
	}

	detSorted := make([]DetectedNote, len(detShifted))
	copy(detSorted, detShifted)
	sort.Slice(detSorted, func(i, j int) bool {
		return detSorted[i].StartTime < detSorted[j].StartTime
	})

	merged := mergeRefs(refs)

	matched := make([]NoteComparison, len(refs))
	for i := range matched {
		matched[i] = NoteComparison{
			RefPitch:    refs[i].Pitch,
			RefStart:    refs[i].StartTime,
			RefEnd:      refs[i].EndTime,
			MatchStatus: "missed",
			MissReason:  "未偵測到",
		}
	}

	// 移除 usedDetected：允許一個偵測音符對應多個 ref（多對一匹配）
	// (因 detectPitch 可能把連續滑音合併成一個長音符，覆蓋多個 ref audio)
	matchedCount := 0

	for _, mg := range merged {
		mgLen := mg.EndTime - mg.StartTime
		bestIdx := -1
		bestOverlap := 0.0

		for i, det := range detSorted {
			// 音高容差
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
			pitchDev := (mg.PitchFloat - det.PitchFloat) * 100.0
			durDev := mgLen - detLen

			for _, eidx := range mg.EventIdx {
				matched[eidx].UserPitch = det.Pitch
				matched[eidx].UserStart = det.StartTime
				matched[eidx].UserEnd = det.EndTime
				matched[eidx].PitchDeviationCents = pitchDev
				matched[eidx].DurationDeviationSec = durDev
				matched[eidx].MatchStatus = "matched"
				matched[eidx].MissReason = ""
			}
			matchedCount += len(mg.EventIdx)
		}
	}

	// 對未匹配的參考音符，分析原因
	for i := range matched {
		if matched[i].MatchStatus == "matched" {
			continue
		}
		ref := refs[i]
		// 找該時間區間內最近的偵測音符
		bestDet := -1
		bestDist := math.Inf(1)
		for j, det := range detSorted {
			dist := math.Abs(det.StartTime - ref.StartTime)
			if dist < bestDist {
				bestDist = dist
				bestDet = j
			}
		}

		if bestDet >= 0 {
			det := detSorted[bestDet]
			// 檢查時間是否接近（< 1s）
			if bestDist < 1.0 {
				if det.Pitch != ref.Pitch {
					matched[i].MissReason = fmt.Sprintf("音高不符: 參考=%d, 偵測=%d (偏差=%d semitone)", ref.Pitch, det.Pitch, det.Pitch-ref.Pitch)
					matched[i].UserPitch = det.Pitch
					matched[i].UserStart = det.StartTime
					matched[i].UserEnd = det.EndTime
				} else {
					// 音高相同但重疊不足
					overlapStart := math.Max(ref.StartTime, det.StartTime)
					overlapEnd := math.Min(ref.EndTime, det.EndTime)
					overlapLen := math.Max(0, overlapEnd-overlapStart)
					refLen := ref.EndTime - ref.StartTime
					detLen := det.EndTime - det.StartTime
					shorterLen := refLen
					if detLen < shorterLen {
						shorterLen = detLen
					}
					ratio := 0.0
					if shorterLen > 0 {
						ratio = overlapLen / shorterLen
					}
					matched[i].MissReason = fmt.Sprintf("重疊不足: 重疊率=%.2f (門檻=%.2f), 偵測時間=[%.2f, %.2f]", ratio, overlapThreshold, det.StartTime, det.EndTime)
					matched[i].UserPitch = det.Pitch
					matched[i].UserStart = det.StartTime
					matched[i].UserEnd = det.EndTime
				}
			} else {
				matched[i].MissReason = fmt.Sprintf("未偵測到: 最近偵測音符在 %.2fs 外 (時間=[%.2f, %.2f])", bestDist, det.StartTime, det.EndTime)
			}
		} else {
			matched[i].MissReason = "未偵測到: 無任何偵測音符"
		}
	}

	// 為每個參考音符填入其 start-end 範圍內的所有偵測音符（套用 offset 後）
	for i := range matched {
		ref := refs[i]
		var inRange []DetectedInRange
		for _, det := range detSorted {
			// 偵測音符與參考音符時間重疊
			if det.EndTime > ref.StartTime && det.StartTime < ref.EndTime {
				inRange = append(inRange, DetectedInRange{
					Pitch:      det.Pitch,
					PitchFloat: det.PitchFloat,
					StartTime:  det.StartTime,
					EndTime:    det.EndTime,
				})
			}
		}
		matched[i].DetectedInRange = inRange
	}

	return matched
}

// ---- 主程式 ----

type TrackInfo struct {
	Name     string `json:"name"`
	IsVocal  bool   `json:"is_vocal"`
	NumNotes int    `json:"num_notes"`
	Start    float64 `json:"start"`
	End      float64 `json:"end"`
}

type Report struct {
	MIDIFile       string        `json:"midi_file"`
	WAVFile        string        `json:"wav_file"`
	Tracks         []TrackInfo   `json:"tracks"`
	Tenor1Track    string        `json:"tenor1_track"`
	RefNotes       int           `json:"ref_notes"`
	RefStart       float64       `json:"ref_start"`
	RefEnd         float64       `json:"ref_end"`
	RefPitchMin    int           `json:"ref_pitch_min"`
	RefPitchMax    int           `json:"ref_pitch_max"`
	DetectedNotes  int           `json:"detected_notes"`
	DetectedStart  float64       `json:"detected_start"`
	DetectedEnd    float64       `json:"detected_end"`
	DetectedPitchMin int         `json:"detected_pitch_min"`
	DetectedPitchMax int         `json:"detected_pitch_max"`
	SampleRate     int           `json:"sample_rate"`
	WAVDuration    float64       `json:"wav_duration"`
	BestOffset     float64       `json:"best_offset"`
	MatchRateNoOffset float64    `json:"match_rate_no_offset"`
	MatchRateWithOffset float64  `json:"match_rate_with_offset"`
	MatchRateImproved float64    `json:"match_rate_improved"`
	ImprovedMatched int          `json:"improved_matched"`
	NoOffset       []NoteComparison `json:"no_offset_comparison"`
	WithOffset     []NoteComparison `json:"with_offset_comparison"`
	Improved       []NoteComparison `json:"improved_comparison"`
	Offset041      []NoteComparison `json:"offset_041_comparison"`
}

func main() {
	midiPath := "uploads_test/1c766971-eb04-4363-9284-ab2885f0406d.mid"
	wavPath := "debug_uploads/20260810_155328_user_recording.wav"

	// 1. 讀取 MIDI
	midiData, err := os.ReadFile(midiPath)
	if err != nil {
		fmt.Printf("ERROR reading MIDI: %v\n", err)
		os.Exit(1)
	}

	parser := service.NewMIDIParser()
	parsed, err := parser.ParseExtracted(midiData)
	if err != nil {
		fmt.Printf("ERROR parsing MIDI: %v\n", err)
		os.Exit(1)
	}

	// 2. 列出所有 track
	report := Report{
		MIDIFile: midiPath,
		WAVFile:  wavPath,
	}
	for _, t := range parsed.Tracks {
		start, end := 0.0, 0.0
		if len(t.Notes) > 0 {
			start = t.Notes[0].StartTime
			end = t.Notes[len(t.Notes)-1].EndTime
		}
		report.Tracks = append(report.Tracks, TrackInfo{
			Name:     t.Name,
			IsVocal:  t.IsVocal,
			NumNotes: len(t.Notes),
			Start:    start,
			End:      end,
		})
	}

	// 3. 找出 tenor1 track（大小寫不敏感）
	var tenor1 *domain.MIDITrack
	for i := range parsed.Tracks {
		if strings.EqualFold(parsed.Tracks[i].Name, "tenor1") {
			tenor1 = &parsed.Tracks[i]
			break
		}
	}
	if tenor1 == nil {
		fmt.Println("ERROR: tenor1 track not found. Available tracks:")
		for _, t := range report.Tracks {
			fmt.Printf("  - %s (vocal=%v, notes=%d)\n", t.Name, t.IsVocal, t.NumNotes)
		}
		os.Exit(1)
	}
	report.Tenor1Track = tenor1.Name

	// 4. 過濾 tenor1 notes 到 0~27s
	var refNotes []domain.MIDINote
	for _, n := range tenor1.Notes {
		if n.StartTime >= 0 && n.StartTime <= 27.0 {
			refNotes = append(refNotes, n)
		}
	}
	report.RefNotes = len(refNotes)
	if len(refNotes) > 0 {
		report.RefStart = refNotes[0].StartTime
		report.RefEnd = refNotes[len(refNotes)-1].EndTime
		report.RefPitchMin = 127
		report.RefPitchMax = 0
		for _, n := range refNotes {
			if n.Pitch < report.RefPitchMin {
				report.RefPitchMin = n.Pitch
			}
			if n.Pitch > report.RefPitchMax {
				report.RefPitchMax = n.Pitch
			}
		}
	}

	// 5. 讀取 WAV
	wavData, err := os.ReadFile(wavPath)
	if err != nil {
		fmt.Printf("ERROR reading WAV: %v\n", err)
		os.Exit(1)
	}

	samples, sampleRate, err := service.DecodeAudioData(wavData, "wav")
	if err != nil {
		fmt.Printf("ERROR decoding WAV: %v\n", err)
		os.Exit(1)
	}
	report.SampleRate = sampleRate
	report.WAVDuration = float64(len(samples)) / float64(sampleRate)

	// 6. 偵測音符
	detected := detectPitch(samples, sampleRate)
	// 診斷：輸出前 10 個原始偵測音符
	fmt.Printf("\n--- 原始偵測音符（前 10 個，未套用 offset）---\n")
	for i, d := range detected {
		if i >= 10 {
			break
		}
		fmt.Printf("  [%d] pitch=%d pitchFloat=%.2f time=[%.2f, %.2f]\n", i, d.Pitch, d.PitchFloat, d.StartTime, d.EndTime)
	}
	report.DetectedNotes = len(detected)
	if len(detected) > 0 {
		report.DetectedStart = detected[0].StartTime
		report.DetectedEnd = detected[len(detected)-1].EndTime
		report.DetectedPitchMin = 127
		report.DetectedPitchMax = 0
		for _, d := range detected {
			if d.Pitch < report.DetectedPitchMin {
				report.DetectedPitchMin = d.Pitch
			}
			if d.Pitch > report.DetectedPitchMax {
				report.DetectedPitchMax = d.Pitch
			}
		}
	}

	// 7. 測試多個 offset 值
	//    - 0.56：project 原本機制（selectedStructure.start + firstNoteTime）
	//    - 0.25：使用者指定
	//    - 0.15：WAV 第一個偵測音符
	offsets := []float64{0.0, 0.15, 0.25, 0.41, 0.56}
	report.BestOffset = 0.41 // WAV 0.15 + 0.41 = MIDI 0.56

	// 8. 比對（無 offset、有 offset、改善後）
	report.NoOffset = compareNotes(refNotes, detected, 0, 0.8, 0)
	report.WithOffset = compareNotes(refNotes, detected, report.BestOffset, 0.8, 0)
	// 改善：音高容差 ±1 semitone、重疊門檻 0.5
	improved := compareNotes(refNotes, detected, report.BestOffset, 0.5, 1)

	// 計算各 offset 的 match rate（改善後參數）
	fmt.Printf("\n--- Offset 比較（改善後參數：音高容差 ±1、重疊門檻 0.5）---\n")
	for _, off := range offsets {
		res := compareNotes(refNotes, detected, off, 0.5, 1)
		cnt := 0
		for _, c := range res {
			if c.MatchStatus == "matched" {
				cnt++
			}
		}
		fmt.Printf("  offset=%.2f: %.1f%% (%d/%d)\n", off, float64(cnt)/float64(len(refNotes))*100, cnt, len(refNotes))
	}

	noOffsetMatched := 0
	withOffsetMatched := 0
	improvedMatched := 0
	for _, c := range report.NoOffset {
		if c.MatchStatus == "matched" {
			noOffsetMatched++
		}
	}
	for _, c := range report.WithOffset {
		if c.MatchStatus == "matched" {
			withOffsetMatched++
		}
	}
	for _, c := range improved {
		if c.MatchStatus == "matched" {
			improvedMatched++
		}
	}
	if len(refNotes) > 0 {
		report.MatchRateNoOffset = float64(noOffsetMatched) / float64(len(refNotes)) * 100
		report.MatchRateWithOffset = float64(withOffsetMatched) / float64(len(refNotes)) * 100
	}
	report.MatchRateImproved = float64(improvedMatched) / float64(len(refNotes)) * 100
	report.Improved = improved
	report.ImprovedMatched = improvedMatched
	// offset=0.41 詳細比較資料
	report.Offset041 = compareNotes(refNotes, detected, 0.41, 0.5, 1)

	// 診斷：測試 overlap=0.8（pitchTolerance=1）
	fmt.Printf("\n--- Overlap 0.8 vs 0.5 (offset=0.41, pitchTolerance=1) ---\n")
	for _, ov := range []float64{0.5, 0.7, 0.8} {
		res := compareNotes(refNotes, detected, 0.41, ov, 1)
		cnt := 0
		var missed []string
		for i, c := range res {
			if c.MatchStatus == "matched" {
				cnt++
			} else {
				missed = append(missed, fmt.Sprintf("[%d]pitch=%d time=[%.2f,%.2f]", i, c.RefPitch, c.RefStart, c.RefEnd))
			}
		}
		fmt.Printf("  overlap=%.1f: %.1f%% (%d/%d) missed=%v\n", ov, float64(cnt)/float64(len(refNotes))*100, cnt, len(refNotes), missed)
	}

	// 9. 輸出
	jsonData, _ := json.Marshal(report)
	os.MkdirAll("midi_vs_wav_report_output", 0755)
	os.WriteFile("midi_vs_wav_report_output/report.json", jsonData, 0644)

	// 可讀輸出
	fmt.Printf("=== MIDI/WAV 比較診斷 ===\n")
	fmt.Printf("MIDI: %s\n", midiPath)
	fmt.Printf("WAV: %s\n", wavPath)
	fmt.Printf("\n--- Tracks ---\n")
	for _, t := range report.Tracks {
		fmt.Printf("  %s (vocal=%v, notes=%d, time=[%.2f, %.2f])\n", t.Name, t.IsVocal, t.NumNotes, t.Start, t.End)
	}
	fmt.Printf("\n--- Tenor1 Track (0~27s) ---\n")
	fmt.Printf("  Notes: %d, Time: [%.2f, %.2f], Pitch: [%d, %d]\n", report.RefNotes, report.RefStart, report.RefEnd, report.RefPitchMin, report.RefPitchMax)
	fmt.Printf("\n--- WAV ---\n")
	fmt.Printf("  SampleRate: %d, Duration: %.2fs\n", report.SampleRate, report.WAVDuration)
	fmt.Printf("  Detected Notes: %d, Time: [%.2f, %.2f], Pitch: [%d, %d]\n", report.DetectedNotes, report.DetectedStart, report.DetectedEnd, report.DetectedPitchMin, report.DetectedPitchMax)
	fmt.Printf("\n--- Alignment ---\n")
	fmt.Printf("  Best Offset: %.2fs\n", report.BestOffset)
	fmt.Printf("  Match Rate (no offset): %.1f%% (%d/%d)\n", report.MatchRateNoOffset, noOffsetMatched, len(refNotes))
	fmt.Printf("  Match Rate (with offset): %.1f%% (%d/%d)\n", report.MatchRateWithOffset, withOffsetMatched, len(refNotes))
	fmt.Printf("  Match Rate (improved): %.1f%% (%d/%d)\n", report.MatchRateImproved, improvedMatched, len(refNotes))

	fmt.Printf("\n--- Missing Notes (with offset) ---\n")
	for i, c := range report.WithOffset {
		if c.MatchStatus == "missed" {
			fmt.Printf("  [%d] Ref pitch=%d time=[%.2f, %.2f] -> %s\n", i, c.RefPitch, c.RefStart, c.RefEnd, c.MissReason)
		}
	}

	fmt.Printf("\n--- Missing Notes (improved) ---\n")
	for i, c := range report.Improved {
		if c.MatchStatus == "missed" {
			fmt.Printf("  [%d] Ref pitch=%d time=[%.2f, %.2f] -> %s\n", i, c.RefPitch, c.RefStart, c.RefEnd, c.MissReason)
		}
	}

	fmt.Printf("\n報告已儲存到 midi_vs_wav_report_output/report.json\n")
}

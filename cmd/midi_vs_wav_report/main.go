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

type MIDINoteForTest struct {
	Pitch     int
	StartTime float64
	EndTime   float64
}

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

// MergedNote groups consecutive same-pitch MIDI events into one logical note.
type MergedNote struct {
	Pitch     int
	StartTime float64
	EndTime   float64
	EventIdx  []int // indices into the original referenceNotes slice
}

func main() {
	midiPath := filepath.Join("test_data", "reference.mid")
	wavPath := filepath.Join("test_data", "test_recording_alto.wav")

	// 1. Parse MIDI
	midiData, err := os.ReadFile(midiPath)
	if err != nil {
		panic(err)
	}
	parser := service.NewMIDIParser()
	song, err := parser.Parse(midiData, "reference", "")
	if err != nil {
		panic(err)
	}

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
	fmt.Printf("Parsed %d reference notes\n", len(referenceNotes))

	// 2. Parse WAV
	wavData, err := os.ReadFile(wavPath)
	if err != nil {
		panic(err)
	}
	if len(wavData) < 44 {
		panic("WAV too small")
	}

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
				panic(fmt.Sprintf("expected IEEE float (3), got %d", audioFormat))
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
	dataOffset := offset + 8
	dataSize := int32(binary.LittleEndian.Uint32(wavData[offset+4 : offset+8]))

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
	fmt.Printf("Loaded %d samples (%.2f seconds, %d Hz)\n", len(samples), float64(len(samples))/float64(sampleRate), sampleRate)

	// 3a. Volume normalization
	var origMax, origRMS float64
	var nonZeroCount int64
	for _, s := range samples {
		abs := math.Abs(s)
		if abs > origMax {
			origMax = abs
		}
		origRMS += s * s
		if abs > 1e-10 {
			nonZeroCount++
		}
	}
	origRMS = math.Sqrt(origRMS / float64(len(samples)))
	fmt.Printf("Volume stats: max=%.4f, RMS=%.6f, nonZeroSamples=%d\n", origMax, origRMS, nonZeroCount)

	// Normalize to [-1, 1] range
	if origMax > 0 {
		for i := range samples {
			samples[i] /= origMax
		}
		var afterRMS float64
		for _, s := range samples {
			afterRMS += s * s
		}
		afterRMS = math.Sqrt(afterRMS / float64(len(samples)))
		fmt.Printf("Normalized: max=1.0, RMS=%.6f\n", afterRMS)
	}

	// 3b. Pitch detection
	detectedNotes := detectPitchGo(samples, int(sampleRate))
	fmt.Printf("Detected %d notes\n", len(detectedNotes))

	// 4. Merge same-pitch reference notes, then compare merged vs detected
	mergedRef := mergeSamePitchNotes(referenceNotes)
	fmt.Printf("Merged into %d note groups\n", len(mergedRef))

	result := compareMergedNotes(referenceNotes, mergedRef, detectedNotes)
	fmt.Printf("Result: score=%d, total=%d, matched=%d, avgPitchDev=%.1fc, avgDurDev=%.3fs\n",
		result.Score, result.TotalNotes, result.MatchedNotes, result.AveragePitchDeviation, result.AverageDurationDeviation)

	// 5. Generate HTML
	generateHTML(referenceNotes, detectedNotes, mergedRef, result)
}

// mergeSamePitchNotes groups consecutive same-pitch notes whose gap < 50ms.
func mergeSamePitchNotes(notes []MIDINoteForTest) []MergedNote {
	if len(notes) == 0 {
		return nil
	}

	sorted := make([]MIDINoteForTest, len(notes))
	copy(sorted, notes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartTime < sorted[j].StartTime
	})

	// Build original index mapping
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
		Pitch:     sorted[0].Pitch,
		StartTime: sorted[0].StartTime,
		EndTime:   sorted[0].EndTime,
		EventIdx:  []int{sortedToOrig[0]},
	}

	gapThreshold := 0.05 // 50ms

	for i := 1; i < len(sorted); i++ {
		n := sorted[i]
		gap := n.StartTime - current.EndTime
		if n.Pitch == current.Pitch && gap >= 0 && gap <= gapThreshold {
			// Same pitch, close enough → merge
			if n.EndTime > current.EndTime {
				current.EndTime = n.EndTime
			}
			current.EventIdx = append(current.EventIdx, sortedToOrig[i])
		} else {
			// Pitch change or too big gap → finalize current group
			merged = append(merged, current)
			current = MergedNote{
				Pitch:     n.Pitch,
				StartTime: n.StartTime,
				EndTime:   n.EndTime,
				EventIdx:  []int{sortedToOrig[i]},
			}
		}
	}
	merged = append(merged, current)

	return merged
}

// compareMergedNotes compares merged reference note groups against detected notes.
// If a merged group has ≥80% overlap with a same-pitch detected note, all events in
// that group are marked "matched" with the group-level pitch/duration deviation.
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

		// Find the best matching detected note by overlap
		bestIdx := -1
		bestOverlap := 0.0

		for i, det := range detSorted {
			if usedDetected[i] {
				continue
			}
			if det.Pitch != mg.Pitch {
				continue
			}

			// Compute overlap interval
			overlapStart := math.Max(mg.StartTime, det.StartTime)
			overlapEnd := math.Min(mg.EndTime, det.EndTime)
			if overlapEnd <= overlapStart {
				continue
			}
			overlapLen := overlapEnd - overlapStart

			// Overlap ratio relative to the shorter of the two
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

			// Group-level deviations
			detLen := det.EndTime - det.StartTime
			pitchDev := float64(mg.Pitch-det.Pitch) * 100.0 // cents
			durDev := mgLen - detLen

			// Apply to every event in the merged group
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

// --- Everything below this line is unchanged from previous version ---

func detectPitchGo(samples []float64, sampleRate int) []MIDINoteForTest {
	windowSize := 2048
	hopSize := int(float64(sampleRate) * 0.05)
	silenceThreshold := 0.01
	correlationThreshold := 0.3
	minFreq := 50.0
	maxFreq := 2000.0

	var notes []MIDINoteForTest
	var currentNote *MIDINoteForTest

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

		if rms < silenceThreshold {
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

		var energy float64
		for _, s := range window {
			energy += s * s
		}
		if energy == 0 {
			continue
		}

		bestPeriod := 0
		maxCorr := 0.0
		for period := minPeriod; period <= maxPeriod; period++ {
			var corr float64
			for i := 0; i < len(window)-period; i++ {
				corr += window[i] * window[i+period]
			}
			corr /= energy
			if corr > maxCorr {
				maxCorr = corr
				bestPeriod = period
			}
		}

		if maxCorr < correlationThreshold || bestPeriod == 0 {
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

		if currentNote == nil {
			currentNote = &MIDINoteForTest{
				Pitch:     roundedPitch,
				StartTime: time,
				EndTime:   time + float64(hopSize)/float64(sampleRate),
			}
		} else {
			if math.Abs(float64(currentNote.Pitch-roundedPitch)) >= 2 {
				currentNote.EndTime = time
				notes = append(notes, *currentNote)
				currentNote = &MIDINoteForTest{
					Pitch:     roundedPitch,
					StartTime: time,
					EndTime:   time + float64(hopSize)/float64(sampleRate),
				}
			} else {
				currentNote.EndTime = time + float64(hopSize)/float64(sampleRate)
			}
		}
	}

	if currentNote != nil {
		notes = append(notes, *currentNote)
	}

	var filtered []MIDINoteForTest
	for _, n := range notes {
		if n.EndTime-n.StartTime > 0.1 {
			filtered = append(filtered, n)
		}
	}
	return filtered
}

func generateHTML(refNotes, detNotes []MIDINoteForTest, merged []MergedNote, result AssessmentResult) {
	htmlContent := `<!DOCTYPE html>
<html lang="zh-TW">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>MIDI vs WAV 比對報告 (Merged)</title>
<style>
  body { font-family: 'Segoe UI', sans-serif; max-width: 1200px; margin: 0 auto; padding: 20px; background: #f5f5f5; }
  h1 { color: #333; }
  .score-card { background: white; border-radius: 12px; padding: 20px; margin: 16px 0; box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
  .score-value { font-size: 48px; font-weight: bold; }
  .stats { display: flex; gap: 24px; flex-wrap: wrap; }
  .stat { background: white; border-radius: 8px; padding: 12px 20px; box-shadow: 0 1px 4px rgba(0,0,0,0.1); }
  .stat-label { font-size: 12px; color: #888; }
  .stat-value { font-size: 24px; font-weight: bold; }
  canvas { width: 100%; max-width: 800px; height: 300px; background: white; border-radius: 8px; box-shadow: 0 1px 4px rgba(0,0,0,0.1); }
  table { width: 100%; border-collapse: collapse; background: white; border-radius: 8px; overflow: hidden; box-shadow: 0 1px 4px rgba(0,0,0,0.1); }
  th { background: #4a90d9; color: white; padding: 10px; text-align: left; }
  td { padding: 6px 10px; border-bottom: 1px solid #eee; }
  td.missed { color: #e74c3c; }
  td.matched { color: #2ecc71; }
  .scroll-table { max-height: 400px; overflow-y: auto; }
  .green { color: #2ecc71; }
  .yellow { color: #f1c40f; }
  .red { color: #e74c3c; }
  .merged-info { background: #eef; border-radius: 8px; padding: 12px 20px; margin: 8px 0; font-size: 14px; }
</style>
</head>
<body>
<h1>🎵 MIDI vs WAV 比對報告 (Merged)</h1>
<div class="score-card">
  <div class="score-value ` + scoreClass(result.Score) + `">` + fmt.Sprintf("%d", result.Score) + `</div>
  <div style="font-size:18px;color:#555;">總分</div>
</div>
<div class="stats">
  <div class="stat"><div class="stat-label">原始音符數</div><div class="stat-value">` + fmt.Sprintf("%d", result.TotalNotes) + `</div></div>
  <div class="stat"><div class="stat-label">合併群組數</div><div class="stat-value">` + fmt.Sprintf("%d", len(merged)) + `</div></div>
  <div class="stat"><div class="stat-label">匹配音符數</div><div class="stat-value green">` + fmt.Sprintf("%d", result.MatchedNotes) + `</div></div>
  <div class="stat"><div class="stat-label">未匹配音符數</div><div class="stat-value red">` + fmt.Sprintf("%d", result.TotalNotes-result.MatchedNotes) + `</div></div>
  <div class="stat"><div class="stat-label">平均音高偏差</div><div class="stat-value">` + fmt.Sprintf("%.1f", result.AveragePitchDeviation) + ` cents</div></div>
  <div class="stat"><div class="stat-label">平均時長偏差</div><div class="stat-value">` + fmt.Sprintf("%.3f", result.AverageDurationDeviation) + ` s</div></div>
  <div class="stat"><div class="stat-label">檢測音符數</div><div class="stat-value">` + fmt.Sprintf("%d", len(detNotes)) + `</div></div>
</div>

<h2>📊 音高輪廓</h2>
<canvas id="pitchCanvas"></canvas>

<h2>📋 合併群組列表</h2>
<div class="scroll-table">
<table>
<tr><th>群組</th><th>音高</th><th>開始</th><th>結束</th><th>長度</th><th>包含事件數</th></tr>
`

	for i, mg := range merged {
		htmlContent += fmt.Sprintf(`<tr><td>%d</td><td>%d</td><td>%.2f</td><td>%.2f</td><td>%.2f</td><td>%d</td></tr>
`, i+1, mg.Pitch, mg.StartTime, mg.EndTime, mg.EndTime-mg.StartTime, len(mg.EventIdx))
	}

	htmlContent += `</table></div>

<h2>📋 音符比對表 (原始事件)</h2>
<div class="scroll-table">
<table>
<tr><th>#</th><th>狀態</th><th>參考音高</th><th>使用者音高</th><th>參考開始</th><th>參考結束</th><th>使用者開始</th><th>使用者結束</th><th>音高偏差</th><th>時長偏差</th></tr>
`

	for i, nc := range result.NoteComparison {
		if i >= 100 {
			break
		}
		statusText := "✅"
		statusClass := "matched"
		if nc.MatchStatus == "missed" {
			statusText = "❌"
			statusClass = "missed"
		}

		userPitchStr := fmt.Sprintf("%d", nc.UserPitch)
		userStartStr := fmt.Sprintf("%.2f", nc.UserStart)
		userEndStr := fmt.Sprintf("%.2f", nc.UserEnd)
		pitchDevStr := fmt.Sprintf("%.0f", nc.PitchDeviationCents)
		durDevStr := fmt.Sprintf("%.3f", nc.DurationDeviationSec)
		if nc.MatchStatus == "missed" {
			userPitchStr = "-"
			userStartStr = "-"
			userEndStr = "-"
			pitchDevStr = "-"
			durDevStr = "-"
		}

		htmlContent += fmt.Sprintf(`<tr><td class="%s">%d</td><td class="%s">%s</td><td>%d</td><td>%s</td><td>%.2f</td><td>%.2f</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>
`, statusClass, i+1, statusClass, statusText, nc.RefPitch, userPitchStr, nc.RefStart, nc.RefEnd, userStartStr, userEndStr, pitchDevStr, durDevStr)
	}

	htmlContent += `</table></div>

<script>
const refNotes = ` + notesJSON(refNotes) + `;
const detNotes = ` + notesJSON(detNotes) + `;
const comparisons = ` + comparisonsJSON(result.NoteComparison) + `;
const minTime = Math.min(...refNotes.map(n=>n.s));
const maxTime = Math.max(...refNotes.map(n=>n.e));
const allPitches = [...refNotes.map(n=>n.p), ...detNotes.filter(d=>d.p>0).map(d=>d.p)];
const minPitch = Math.min(...allPitches) - 2;
const maxPitch = Math.max(...allPitches) + 2;
const timeRange = Math.max(maxTime - minTime, 1);
const canvas = document.getElementById('pitchCanvas');
const ctx = canvas.getContext('2d');
const dpr = window.devicePixelRatio || 1;
const rect = canvas.getBoundingClientRect();
canvas.width = rect.width * dpr;
canvas.height = rect.height * dpr;
ctx.scale(dpr, dpr);
const W = rect.width;
const H = rect.height;
const margin = {top:20,bottom:25,left:40,right:20};
const pw = W - margin.left - margin.right;
const ph = H - margin.top - margin.bottom;
const xs = t => margin.left + ((t - minTime)/timeRange)*pw;
const ys = p => margin.top + ph - ((p - minPitch)/(maxPitch-minPitch))*ph;

// grid
ctx.strokeStyle = '#eee';
ctx.lineWidth = 1;
for(let p = minPitch; p <= maxPitch; p+=2){
  ctx.beginPath(); ctx.moveTo(margin.left, ys(p)); ctx.lineTo(W-margin.right, ys(p)); ctx.stroke();
  ctx.fillStyle = '#999'; ctx.font = '10px sans-serif'; ctx.textAlign = 'right';
  ctx.fillText(p, margin.left-5, ys(p)+4);
}

// ref notes (blue)
ctx.strokeStyle = '#3498db';
ctx.lineWidth = 2;
ctx.beginPath();
refNotes.forEach((n,i) => {
  const x = xs(n.s), y = ys(n.p);
  if(i===0) ctx.moveTo(x,y); else ctx.lineTo(x,y);
  ctx.lineTo(xs(n.e), y);
});
ctx.stroke();

// detected notes (orange dashed)
ctx.strokeStyle = '#e67e22';
ctx.lineWidth = 2;
ctx.setLineDash([4,4]);
ctx.beginPath();
let drawing = false;
detNotes.forEach((n,i) => {
  if(n.p > 0){
    const x = xs(n.s), y = ys(n.p);
    if(!drawing){ ctx.moveTo(x,y); drawing=true; }
    else ctx.lineTo(x,y);
    ctx.lineTo(xs(n.e), y);
  } else { drawing=false; }
});
ctx.stroke();
ctx.setLineDash([]);

// highlight deviations
comparisons.forEach(c => {
  if(c.ms === 'matched' && Math.abs(c.pd) > 100){
    const x = xs(c.rs), y = ys(c.rp), uy = ys(c.up);
    ctx.fillStyle = 'rgba(231,76,60,0.3)';
    ctx.fillRect(x, Math.min(y,uy), xs(c.re)-x, Math.abs(y-uy));
  }
});

ctx.fillStyle = '#3498db'; ctx.font = '11px sans-serif'; ctx.textAlign = 'center';
ctx.fillText('參考 MIDI', W/2 - 60, 14);
ctx.fillStyle = '#e67e22';
ctx.fillText('WAV 檢測', W/2 + 60, 14);
</script>
</body>
</html>`

	outPath := filepath.Join("output", "comparison_report.html")
	os.MkdirAll("output", 0755)
	if err := os.WriteFile(outPath, []byte(htmlContent), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Report saved to %s\n", outPath)
}

func scoreClass(score int) string {
	if score >= 80 {
		return "green"
	}
	if score >= 50 {
		return "yellow"
	}
	return "red"
}

func notesJSON(notes []MIDINoteForTest) string {
	s := "["
	for i, n := range notes {
		if i > 0 {
			s += ","
		}
		s += fmt.Sprintf(`{"s":%.2f,"e":%.2f,"p":%d}`, n.StartTime, n.EndTime, n.Pitch)
	}
	return s + "]"
}

func comparisonsJSON(comparisons []NoteComparisonResult) string {
	s := "["
	for i, c := range comparisons {
		if i > 0 {
			s += ","
		}
		s += fmt.Sprintf(`{"rs":%.2f,"re":%.2f,"rp":%d,"us":%.2f,"ue":%.2f,"up":%d,"pd":%.0f,"dd":%.3f,"ms":"%s"}`,
			c.RefStart, c.RefEnd, c.RefPitch, c.UserStart, c.UserEnd, c.UserPitch, c.PitchDeviationCents, c.DurationDeviationSec, c.MatchStatus)
	}
	return s + "]"
}

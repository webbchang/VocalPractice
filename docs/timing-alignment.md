# Recording-Reference Note Timing Alignment

## Problem

When a user practices singing, the client records their voice and sends it to the
backend for pitch assessment. The backend compares the user's detected notes
against reference notes extracted from the MIDI file.

The two note sets live on **different time scales**:

| Source         | Time scale                                    | Example            |
|----------------|-----------------------------------------------|--------------------|
| Reference      | Absolute MIDI time (seconds from song start) | 8.89, 10.55        |
| Detected       | Relative to recording start (0-based)         | 0.00, 5.15, ...    |

Without alignment, `compareMergedNotesForAssessment` computes
`overlapStart = max(ref.Start, det.Start)` and `overlapEnd = min(ref.End, det.End)`.
Because `det.Start` (~0-5s) and `ref.Start` (~8.89s) never overlap, every
reference note is marked `"missed"` regardless of whether the user sang the
right pitch.

## Solution

Align the detected notes to the reference notes' absolute timeline. The client:

1. **Trims the count-in** (3-beat lead-in) from the recording so detected note
   times start at the first accompaniment note, not at `Beat 1`.
2. **Calculates the offset** `recordingStartTime = sectionStart + firstNoteTime`,
   where `firstNoteTime` is the offset of the first note from the section start.
3. **Sends the offset** to the backend in the `recording_start_time` field.

The backend:

1. **Detects pitches** and **filters vowel portions** using the (trimmed)
   recording — note start/end times are still relative to the recording.
2. **Shifts** all detected/vowel note times by `recordingStartTime` to convert
   them to absolute MIDI time.
3. **Compares** with reference notes (already absolute) — now both are on the
   same timeline.

### Timing diagram

```
MIDI timeline (absolute):   0s ........... [sectionStart] ....... [first ref note @ 8.89] ...........

Recording timeline (before trim):
  [Beat 1] [Beat 2] [Beat 3] [first accompaniment note] [user sings...] [end]
  0          beatInt  2×beatInt  2×beatInt+0.25 = recordingTrimOffset

Recording timeline (after trim):
  [first accompaniment note] [user sings...] [end]
  0

After backend shift (+ recordingStartTime):
  [first ref note @ 8.89] [user sings...] [end]
```

## Pseudo Code

```javascript
// ── Client side (practice-business.js) ──

function startPractice() {
    const { firstNoteTime, beatInterval } = getTimingInfo(sectionStart, sectionEnd);
    const recordingTrimOffset = 2 * beatInterval + 0.25;  // 3-beat count-in

    // Store for use in stopMediaRecorder and analyzeAndSubmitRecording
    state.recordingStartTime = sectionStart + firstNoteTime;   // absolute MIDI time of recording t=0 (after trim)
    state.recordingTrimOffset = recordingTrimOffset;

    // ... schedule accompaniment playback, count-in beats, and start recording
    // Recording starts at Beat 1 (before the first note)
}

function stopMediaRecorder() {
    // Trim the count-in from the audio buffer before encoding
    const trimSamples = floor(state.recordingTrimOffset * sampleRate);
    const trimmedBuffer = audioBuffer.slice(trimSamples);
    recordedWavBlob = encodeWAV(new Float32Array(trimmedBuffer), sampleRate);
}

async function analyzeAndSubmitRecording() {
    const audioBase64 = await getRecordedAudioBase64();

    // Send the offset so the backend can align detected notes with reference notes
    const result = await api("/assessments/analyze", {
        method: "POST",
        body: JSON.stringify({
            audio_data: audioBase64,
            audio_format: "wav",
            reference_notes: referenceNotes,       // absolute MIDI times (unchanged)
            recording_start_time: state.recordingStartTime   // the offset
        })
    });
}
```

```go
// ── Backend side (internal/service/assessment.go) ──

func AssessRecordingWithVowelFiltering(
    samples []float64,
    sampleRate int,
    referenceNotes []domain.MIDINote,   // absolute MIDI times
    recordingStartTime float64,          // offset from client
) *AssessmentResultForAssessment {

    // Step 1: Detect all pitches (times are relative to recording start)
    detectedNotes := detectPitchGo(samples, sampleRate)

    // Step 2: Filter to vowel-only portions
    //    (uses relative times → sample indices within recording bounds)
    vowelNotes := separator.GetVowelOnlyNotes(samples, sampleRate, detectedNotes)

    // Step 3: Shift detected notes to absolute MIDI time
    for note in vowelNotes {
        note.StartTime += recordingStartTime
        note.EndTime   += recordingStartTime
    }

    // Step 4: Compare — both reference and detected are now absolute
    result := compareMergedNotesForAssessment(refNotes, mergedRef, vowelNotes)
    return result
}
```

## Files Modified

| File                          | Change                                                      |
|-------------------------------|-------------------------------------------------------------|
| `assets/js/state.js`          | Added `recordingStartTime` and `recordingTrimOffset` fields   |
| `assets/js/practice-business.js` | Store timing info, trim audio, send offset in API request |
| `assets/js/songDataExtractor.js` | Fixed syntax error (missing `)` → caused `SongDataExtractor` to be `undefined`) |
| `internal/handler/assessments.go` | Added `RecordingStartTime` to request struct            |
| `internal/service/assessment.go`  | Accept offset, shift detected notes after vowel filtering |

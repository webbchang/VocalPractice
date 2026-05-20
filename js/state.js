// === State Module ===
export const API_BASE = '/api/v1';

export const state = {
    token: null,
    user: null,
    songs: [],
    selectedSong: null,
    selectedVocalTrack: null,
    selectedAccompanimentTracks: [],
    selectedStructure: null,
    structures: [],
    mediaRecorder: null,
    audioChunks: [],
    isRecording: false,
    isCountIn: false,
    referenceNotes: [],
    accompanimentNotes: [],
    detectedNotes: [],
    assessmentResult: null,
    // Range selection
    rangeStartId: null,
    rangeEndId: null,
    selectedRangeStart: null,
    selectedRangeEnd: null,
    // Accompaniment
    audioCtx: null,
    accompanimentOscillators: [],
    accompanimentGain: null,
    currentScheduledNotes: [],
    playbackStartTime: 0,
    // Headset
    preferredMicId: null,
    headsetState: 'unknown', // 'wired' | 'bluetooth' | 'none' | 'unknown'
    // Lyrics
    lyricsLines: [],        // [{ structureId, lyrics, startTime, endTime }]
    lyricsAnimFrame: null,
    // Progress
    progressInterval: null,
    recordingTimer: null
};
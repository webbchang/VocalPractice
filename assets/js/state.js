// state.js — 集中式狀態管理
// 所有模組透過此物件讀寫共用狀態

const state = {
    currentSongId: null,
    currentSongFull: null,
    structures: [],
    selectedTrackId: null,
    selectedAccompanimentTrackIds: [],

    // 選取結果（由 selection.js 計算）
    selectedStructure: null, // { id, start, end, title } — section 或合併範圍
    selectedPhraseIds: [],   // 被選取的 phrase id（含整段選取所涵蓋的 phrase）
    selectedPhrasesData: [], // 對應的 phrase 原始資料（含 lyrics）
    selectedSectionIds: [],  // 被「整段選取」的 section id（整選，非逐句）

    midiData: null,
    parsedNotes: null,
    referenceNotes: null,
    allTracksMeta: [],
    parsedTrackIndex: null,
    parsedMIDITracks: [],
    isRecording: false,      // 是否正在錄音
    practicePhase: 'idle',   // 'idle' | 'playing_accompaniment' | 'playing_beats' | 'recording'

    // Recording timing metadata for note alignment
    recordingStartTime: 0,     // absolute MIDI time corresponding to recording time 0 (after count-in trim)
    recordingTrimOffset: 0,    // count-in duration in seconds to trim from the recording
};

export default state;

// state.js — 集中式狀態管理
// 所有模組透過此物件讀寫共用狀態

const state = {
    currentSongId: null,
    currentSongFull: null,
    structures: [],
    selectedTrackId: null,
    selectedAccompanimentTrackIds: [],
    selectedStructure: null, // { id, start, end, title } — section 或合併範圍
    selectedPhraseIds: [],   // 多選的 phrase ID 陣列（連續句子）
    selectedPhrasesData: [], // 對應的 phrase 原始資料（含 lyrics）
    midiData: null,
    parsedNotes: null,
    referenceNotes: null,
    allTracksMeta: [],
    parsedTrackIndex: null,
    parsedMIDITracks: [],
    isRecording: false,      // 是否正在錄音
    practicePhase: 'idle',   // 'idle' | 'playing_accompaniment' | 'playing_beats' | 'recording'
};

export default state;

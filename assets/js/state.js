// state.js — 集中式狀態管理
// 所有模組透過此物件讀寫共用狀態

const state = {
    currentSongId: null,
    currentSongFull: null,
    structures: [],
    selectedTrackId: null,
    selectedAccompanimentTrackIds: [],
    selectedStructure: null, // { id, start, end, title }
    midiData: null,
    parsedNotes: null,
    referenceNotes: null,
    allTracksMeta: [],
    parsedTrackIndex: null,
    parsedMIDITracks: [],
};

export default state;
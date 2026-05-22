import { describe, it, expect, vi, beforeEach } from 'vitest';
import state from '../state.js';

// Setup DOM
function setupDOM() {
    document.body.innerHTML = `
        <div id="vocal-tracks"></div>
        <div id="accompaniment-tracks"></div>
        <div id="structure-list"></div>
        <div id="range-info"></div>
        <div id="range-actions" style="display:none;"></div>
        <div id="current-range-label">未選擇</div>
        <div id="song-title">請選擇歌曲</div>
        <div id="song-artist"></div>
        <div id="status-text">準備就緒</div>
        <div id="chart-area"><div>🎵</div></div>
        <select id="song-select"><option value="">請選擇歌曲...</option></select>
        <button id="record-btn"><svg></svg></button>
    `;
}

// Mock dependent modules with vi.fn() references
const mockPlayRange = vi.fn();
const mockStopPlayback = vi.fn();

vi.mock('../api.js', () => ({
    api: vi.fn(),
    loadMIDI: vi.fn(),
    loadStructures: vi.fn(),
}));

vi.mock('../audio.js', () => ({
    playRange: mockPlayRange,
    stopPlayback: mockStopPlayback,
}));

// Mock practice-ui dynamic imports
vi.mock('../practice-ui.js', () => ({
    renderVocalTracks: vi.fn(),
    renderAccompanimentTracks: vi.fn(),
    renderStructureList: vi.fn(),
    updateSelectionUI: vi.fn(),
}));

// Setup window.SongDataExtractor mock
beforeEach(() => {
    window.SongDataExtractor = {
        extractReferenceNotes: vi.fn((notes, trackIdx, start, end) => {
            return notes.filter(n => n.track === trackIdx && n.start >= start && n.start < end);
        }),
    };
});

describe('practice-business.js', () => {
    let businessModule;

    beforeEach(async () => {
        setupDOM();
        vi.clearAllMocks();
        // Reset state
        state.currentSongId = null;
        state.currentSongFull = null;
        state.structures = [];
        state.selectedTrackId = null;
        state.selectedAccompanimentTrackIds = [];
        state.selectedStructure = null;
        state.parsedNotes = null;
        state.referenceNotes = null;
        state.parsedTrackIndex = null;
        state.midiData = null;

        businessModule = await import('../practice-business.js');
    });

    describe('toggleAccompanimentTrack', () => {
        it('should add trackId to selectedAccompanimentTrackIds', () => {
            businessModule.toggleAccompanimentTrack('track-1');
            expect(state.selectedAccompanimentTrackIds).toContain('track-1');
        });

        it('should remove trackId if already selected', () => {
            state.selectedAccompanimentTrackIds = ['track-1', 'track-2'];
            businessModule.toggleAccompanimentTrack('track-1');
            expect(state.selectedAccompanimentTrackIds).toEqual(['track-2']);
        });

        it('should toggle same ID back and forth', () => {
            businessModule.toggleAccompanimentTrack('track-a');
            expect(state.selectedAccompanimentTrackIds).toEqual(['track-a']);
            businessModule.toggleAccompanimentTrack('track-a');
            expect(state.selectedAccompanimentTrackIds).toEqual([]);
        });
    });

    describe('selectStructure', () => {
        beforeEach(() => {
            state.structures = [
                { id: 's1', title: 'Verse', start_time: 0, end_time: 10 },
                {
                    id: 's2', title: 'Chorus', start_time: 10, end_time: 20,
                    phrases: [
                        { id: 'p1', title: 'Line 1', start_time: 10, end_time: 15 },
                        { id: 'p2', title: 'Line 2', start_time: 15, end_time: 20 },
                    ],
                },
            ];
            state.parsedNotes = [{ pitch: 60, start: 1, dur: 0.5, track: 0 }];
            state.parsedTrackIndex = 0;
        });

        it('should select a top-level structure', () => {
            businessModule.selectStructure('s1');
            expect(state.selectedStructure).toBeDefined();
            expect(state.selectedStructure.id).toBe('s1');
            expect(state.selectedStructure.title).toBe('Verse');
            expect(state.selectedStructure.start).toBe(0);
            expect(state.selectedStructure.end).toBe(10);
        });

        it('should select a phrase within a structure', () => {
            businessModule.selectStructure('p1');
            expect(state.selectedStructure).toBeDefined();
            expect(state.selectedStructure.id).toBe('p1');
            expect(state.selectedStructure.title).toBe('Line 1');
        });

        it('should generate referenceNotes on selection', () => {
            businessModule.selectStructure('s1');
            expect(state.referenceNotes).toBeDefined();
        });

        it('should set referenceNotes to null if no parsedNotes', () => {
            state.parsedNotes = null;
            businessModule.selectStructure('s1');
            expect(state.referenceNotes).toBeNull();
        });
    });

    describe('startPractice', () => {
        it('should alert if no structure selected', () => {
            state.selectedStructure = null;
            const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {});
            businessModule.startPractice();
            expect(alertSpy).toHaveBeenCalledWith('請先選擇一個段落或句子');
            alertSpy.mockRestore();
        });

        it('should alert if no referenceNotes', () => {
            state.selectedStructure = { id: 's1', start: 0, end: 10, title: 'Verse' };
            state.referenceNotes = null;
            const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {});
            businessModule.startPractice();
            expect(alertSpy).toHaveBeenCalledWith('此段落尚無比對資料，請確認已選取正確的聲部');
            alertSpy.mockRestore();
        });

        it('should update DOM and call playRange when valid', () => {
            state.selectedStructure = { id: 's1', start: 0, end: 10, title: 'Verse' };
            state.referenceNotes = [{ pitch: 60, start: 1, dur: 0.5 }];

            businessModule.startPractice();

            expect(document.getElementById('status-text').textContent).toContain('Verse');
            expect(document.getElementById('chart-area').innerHTML).toContain('比對基準');
            expect(document.getElementById('chart-area').innerHTML).toContain('1 個音符');
            expect(mockPlayRange).toHaveBeenCalledWith(0, 10);
        });
    });
});
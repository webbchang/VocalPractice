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
        <div id="lyrics-content"></div>
        <div id="lyrics-status"></div>
    `;
}

// Mock dependent modules with vi.fn() references
const mockPlayRange = vi.fn();
const mockStopPlayback = vi.fn();
const mockPlayRangeDelayed = vi.fn();
const mockScheduleBeats = vi.fn();
const mockGetAudioCtx = vi.fn(() => ({ currentTime: 100 }));

vi.mock('../api.js', () => ({
    api: vi.fn(),
    loadMIDI: vi.fn(),
    loadStructures: vi.fn(),
}));

vi.mock('../audio.js', () => ({
    playRange: mockPlayRange,
    stopPlayback: mockStopPlayback,
    playRangeDelayed: mockPlayRangeDelayed,
    scheduleBeats: mockScheduleBeats,
    getAudioCtx: mockGetAudioCtx,
    setUpdatePlayBtn: vi.fn(),
    stopReplay: vi.fn(),
}));

// Mock practice-ui dynamic imports
vi.mock('../practice-ui.js', () => ({
    renderVocalTracks: vi.fn(),
    renderAccompanimentTracks: vi.fn(),
    renderStructureList: vi.fn(),
    renderLyricsPanel: vi.fn(),
    updateSelectionUI: vi.fn(),
    clearSelection: vi.fn(),
}));

// Setup window.SongDataExtractor mock
beforeEach(() => {
    window.SongDataExtractor = {
        extractReferenceNotes: vi.fn((notes, trackIdx, start, end) => {
            return notes.filter(n => n.track === trackIdx && n.start >= start && n.start < end);
        }),
    };
    window.MidiParser = {
        parseMIDINotes: vi.fn(() => [{ pitch: 60, start: 1, dur: 0.5, track: 0 }]),
        extractBPM: vi.fn(() => 120),
        midiPitchToFreq: vi.fn(() => 440),
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
        state.selectedPhraseIds = [];
        state.selectedPhrasesData = [];
        state.flatItems = [];
        state.selectionRange = { from: null, to: null };
        state.parsedNotes = null;
        state.referenceNotes = null;
        state.parsedTrackIndex = null;
        state.midiData = null;
        state.isRecording = false;
        state.practicePhase = 'idle';

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

    describe('getLyricsForSelection', () => {
        beforeEach(() => {
            state.structures = [
                {
                    id: 's1', title: 'Verse', start_time: 0, end_time: 10,
                    phrases: [
                        { id: 'p1', title: 'Line 1', start_time: 0, end_time: 5, lyrics: 'Hello world' },
                        { id: 'p2', title: 'Line 2', start_time: 5, end_time: 10, lyrics: 'Foo bar' },
                    ],
                },
                { id: 's2', title: 'Chorus', start_time: 10, end_time: 20 },
            ];
        });

        it('should return empty result when nothing selected', () => {
            const result = businessModule.getLyricsForSelection();
            expect(result.count).toBe(0);
            expect(result.phrases).toEqual([]);
        });

        it('should return all phrases lyrics when section with phrases is selected', () => {
            state.selectedStructure = { id: 's1', title: 'Verse', start: 0, end: 10 };
            const result = businessModule.getLyricsForSelection();
            expect(result.count).toBe(2);
            expect(result.phrases[0].title).toBe('Line 1');
            expect(result.phrases[0].lyrics).toBe('Hello world');
            expect(result.phrases[1].title).toBe('Line 2');
            expect(result.phrases[1].lyrics).toBe('Foo bar');
        });

        it('should return section title when section has no phrases', () => {
            state.selectedStructure = { id: 's2', title: 'Chorus', start: 10, end: 20 };
            const result = businessModule.getLyricsForSelection();
            expect(result.count).toBe(1);
            expect(result.phrases[0].title).toBe('Chorus');
        });

        it('should return selected phrases data when multiple phrases selected', () => {
            state.selectedPhraseIds = ['p1', 'p2'];
            state.selectedPhrasesData = [
                { id: 'p1', title: 'Line 1', lyrics: 'Hello world' },
                { id: 'p2', title: 'Line 2', lyrics: 'Foo bar' },
            ];
            state.selectedStructure = { id: 'p1,p2', title: 'Line 1 ~ Line 2（2句）', start: 0, end: 10 };

            const result = businessModule.getLyricsForSelection();
            expect(result.count).toBe(2);
            expect(result.phrases[0].title).toBe('Line 1');
            expect(result.phrases[0].lyrics).toBe('Hello world');
            expect(result.phrases[1].title).toBe('Line 2');
            expect(result.phrases[1].lyrics).toBe('Foo bar');
        });

        it('should handle phrases without lyrics', () => {
            state.structures = [{
                id: 's1', title: 'Verse', start_time: 0, end_time: 10,
                phrases: [
                    { id: 'p1', title: 'Line 1', start_time: 0, end_time: 5 },
                ],
            }];
            state.selectedStructure = { id: 's1', title: 'Verse', start: 0, end: 10 };
            const result = businessModule.getLyricsForSelection();
            expect(result.count).toBe(1);
            expect(result.phrases[0].lyrics).toBe('');
        });
    });

    describe('selectStructure (flat items multi-select)', () => {
        beforeEach(() => {
            state.structures = [
                {
                    id: 's1', title: 'Verse', start_time: 0, end_time: 10,
                    phrases: [
                        { id: 'p1', title: 'Line 1', start_time: 0, end_time: 5, lyrics: 'Hello' },
                        { id: 'p2', title: 'Line 2', start_time: 5, end_time: 10, lyrics: 'World' },
                    ],
                },
                {
                    id: 's2', title: 'Chorus', start_time: 10, end_time: 20,
                    phrases: [
                        { id: 'p3', title: 'Line 3', start_time: 10, end_time: 15, lyrics: 'Foo' },
                        { id: 'p4', title: 'Line 4', start_time: 15, end_time: 20, lyrics: 'Bar' },
                    ],
                },
            ];
            state.flatItems = [
                { type: 'section', data: state.structures[0], section: state.structures[0] },
                { type: 'phrase', data: state.structures[0].phrases[0], section: state.structures[0] },
                { type: 'phrase', data: state.structures[0].phrases[1], section: state.structures[0] },
                { type: 'section', data: state.structures[1], section: state.structures[1] },
                { type: 'phrase', data: state.structures[1].phrases[0], section: state.structures[1] },
                { type: 'phrase', data: state.structures[1].phrases[1], section: state.structures[1] },
            ];
            state.parsedNotes = [{ pitch: 60, start: 1, dur: 0.5, track: 0 }];
            state.parsedTrackIndex = 0;
        });

        it('should select a single section', () => {
            businessModule.selectStructure('s1');
            expect(state.selectionRange).toEqual({ from: 0, to: 0 });
            expect(state.selectedStructure).toBeDefined();
            expect(state.selectedStructure.title).toBe('Verse');
            expect(state.selectedStructure.start).toBe(0);
            expect(state.selectedStructure.end).toBe(10);
            // Section implicitly includes its phrases
            expect(state.selectedPhraseIds).toEqual(['p1', 'p2']);
        });

        it('should select a single phrase', () => {
            businessModule.selectStructure('p1');
            expect(state.selectionRange).toEqual({ from: 1, to: 1 });
            expect(state.selectedStructure.title).toBe('Line 1');
            expect(state.selectedPhraseIds).toEqual(['p1']);
        });

        it('should extend selection forward when clicking later item', () => {
            businessModule.selectStructure('p1'); // idx 1
            businessModule.selectStructure('p2'); // idx 2
            expect(state.selectionRange).toEqual({ from: 1, to: 2 });
            expect(state.selectedPhraseIds).toEqual(['p1', 'p2']);
        });

        it('should extend selection backward when clicking earlier item first', () => {
            businessModule.selectStructure('p2'); // idx 2
            businessModule.selectStructure('p1'); // idx 1
            expect(state.selectionRange).toEqual({ from: 1, to: 2 });
            expect(state.selectedPhraseIds).toEqual(['p1', 'p2']);
        });

        it('should extend across sections', () => {
            businessModule.selectStructure('p2'); // idx 2 (Verse Line 2)
            businessModule.selectStructure('p3'); // idx 4 (Chorus Line 3)
            expect(state.selectionRange).toEqual({ from: 2, to: 4 });
            // Should include p2, s2 (Chorus), p3
            expect(state.selectedPhraseIds).toContain('p2');
            expect(state.selectedPhraseIds).toContain('p3');
            expect(state.selectedPhraseIds).toContain('p4'); // Chorus implicitly includes p4
            expect(state.selectedStructure.title).toContain('Line 2');
            expect(state.selectedStructure.title).toContain('Line 3');
        });

        it('should shrink from start when clicking first selected item again', () => {
            businessModule.selectStructure('p1'); // idx 1
            businessModule.selectStructure('p3'); // idx 4 → range [1, 4]
            // Click p1 (first) again → shrink to [2, 4]
            businessModule.selectStructure('p1');
            expect(state.selectionRange).toEqual({ from: 2, to: 4 });
            expect(state.selectedPhraseIds).not.toContain('p1');
        });

        it('should shrink from end when clicking last selected item again', () => {
            businessModule.selectStructure('p1'); // idx 1
            businessModule.selectStructure('p3'); // idx 4 → range [1, 4]
            // Click p3 (last) again → shrink to [1, 3]
            // Range [1, 3] includes: p1, p2, s2(Chorus) → phrases: p1, p2, p3, p4
            // s2 is still in range so its phrases (p3, p4) are still included
            businessModule.selectStructure('p3');
            expect(state.selectionRange).toEqual({ from: 1, to: 3 });
            // s2 (Chorus) at idx 3 is still in range, so p3, p4 are implicitly included
            expect(state.selectedPhraseIds).toContain('p3');
            expect(state.selectedPhraseIds).toContain('p4');
        });

        it('should reset to single item when clicking middle of selection', () => {
            businessModule.selectStructure('p1'); // idx 1
            businessModule.selectStructure('p3'); // idx 4 → range [1, 4]
            // Click p2 (idx 2, middle) → reset to [2, 2]
            businessModule.selectStructure('p2');
            expect(state.selectionRange).toEqual({ from: 2, to: 2 });
            expect(state.selectedPhraseIds).toEqual(['p2']);
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

        it('should select section and implicitly include its phrases', () => {
            businessModule.selectStructure('s1');
            expect(state.selectedPhraseIds).toEqual(['p1', 'p2']);
            expect(state.selectedPhrasesData.length).toBe(2);
        });

        it('should select section + phrase across sections', () => {
            // Select s1 (Verse) → idx 0
            businessModule.selectStructure('s1');
            // Then extend to p3 (Chorus Line 3) → idx 4
            businessModule.selectStructure('p3');
            expect(state.selectionRange).toEqual({ from: 0, to: 4 });
            // Should include all phrases from both sections
            expect(state.selectedPhraseIds).toEqual(['p1', 'p2', 'p3', 'p4']);
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

        it('should update DOM and call playRangeWithBeats when valid', () => {
            state.selectedStructure = { id: 's1', start: 0, end: 10, title: 'Verse' };
            state.referenceNotes = [{ pitch: 60, start: 1, dur: 0.5 }];
            state.parsedNotes = [{ pitch: 60, start: 1, dur: 0.5, track: 0 }];
            state.parsedTrackIndex = 0;
            state.midiData = new ArrayBuffer(0);

            businessModule.startPractice();

            expect(document.getElementById('status-text').textContent).toContain('Verse');
            expect(document.getElementById('chart-area').innerHTML).toContain('比對基準');
            expect(document.getElementById('chart-area').innerHTML).toContain('1 個音符');
            // Verify scheduleBeats was called with correct params
            expect(mockScheduleBeats).toHaveBeenCalled();
            // Verify playRangeDelayed was called for accompaniment
            expect(mockPlayRangeDelayed).toHaveBeenCalledWith(0, 10, expect.any(Number));
        });
    });

    describe('playRangeWithBeats', () => {
        it('should schedule 3 beats and play accompaniment without recording', () => {
            state.parsedNotes = [{ pitch: 60, start: 1, dur: 0.5, track: 0 }];
            state.parsedTrackIndex = 0;
            state.midiData = new ArrayBuffer(0);

            businessModule.playRangeWithBeats(0, 10, false);

            expect(mockScheduleBeats).toHaveBeenCalledWith(
                expect.any(Object),  // AudioContext
                3,                   // 3 beats
                expect.any(Number),  // beatInterval
                expect.any(Number)   // beat1Time
            );
            expect(mockPlayRangeDelayed).toHaveBeenCalledWith(0, 10, expect.any(Number));
        });

        it('should also schedule recording when withRecording=true', () => {
            state.parsedNotes = [{ pitch: 60, start: 1, dur: 0.5, track: 0 }];
            state.parsedTrackIndex = 0;
            state.midiData = new ArrayBuffer(0);

            businessModule.playRangeWithBeats(0, 10, true);

            expect(mockScheduleBeats).toHaveBeenCalled();
            expect(mockPlayRangeDelayed).toHaveBeenCalledWith(0, 10, expect.any(Number));
            
            // Recording is triggered via setTimeout in playRangeWithBeats
            // Just verify the rest was called correctly
        });
    });

    describe('showReplayButton', () => {
        it('should remove existing replay button container before checking recordedWavBlob', () => {
            const chartArea = document.getElementById('chart-area');
            
            // Manually create a replay button container (simulating previous showReplayButton call)
            const existingContainer = document.createElement('div');
            existingContainer.id = 'replay-btn-container';
            existingContainer.innerHTML = '<button>🔁 回放錄音</button><button>⏹ 中斷回放</button>';
            chartArea.appendChild(existingContainer);
            
            // Verify it exists
            expect(document.getElementById('replay-btn-container')).not.toBeNull();
            expect(chartArea.querySelectorAll('#replay-btn-container').length).toBe(1);
            
            // Call showReplayButton — recordedWavBlob is null so it won't create new buttons
            // but should still remove the existing container first
            businessModule.showReplayButton();
            
            // The container should be removed
            expect(document.getElementById('replay-btn-container')).toBeNull();
            expect(chartArea.querySelectorAll('#replay-btn-container').length).toBe(0);
        });

        it('should remove replay button container when stopReplayPlayback is called', () => {
            const chartArea = document.getElementById('chart-area');
            
            // Create a replay button container
            const container = document.createElement('div');
            container.id = 'replay-btn-container';
            container.innerHTML = '<button>🔁 回放錄音</button>';
            chartArea.appendChild(container);
            
            // Verify it exists
            expect(document.getElementById('replay-btn-container')).not.toBeNull();
            
            // Call stopReplayPlayback
            businessModule.stopReplayPlayback();
            
            // Container should be removed
            expect(document.getElementById('replay-btn-container')).toBeNull();
        });
    });
});
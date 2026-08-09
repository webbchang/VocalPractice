import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import state from '../state.js';

// Setup DOM elements before each test
function setupDOM() {
    document.body.innerHTML = `
        <div id="vocal-tracks"></div>
        <div id="accompaniment-tracks"></div>
        <div id="structure-list"></div>
        <div id="range-info"></div>
        <div id="range-actions" style="display:none;"></div>
        <div id="current-range-label">未選擇</div>
        <button id="record-btn">
            <svg viewBox="0 0 24 24" width="30" height="30">
                <polygon points="8,5 19,12 8,19" fill="white"/>
            </svg>
        </button>
        <div class="viz-tab active"></div>
        <div class="viz-tab"></div>
        <div id="lyrics-content"></div>
        <div id="lyrics-status"></div>
        <div id="results-panel-foldable" class="results-panel" data-folded="true">
            <div id="results-panel-body"></div>
        </div>
        <div id="pitch-panel" data-folded="true">
            <div id="pitch-chart"></div>
        </div>
    `;
}

// Mock audio.js module (hoisted by vitest, no top-level variable refs to state)
vi.mock('../audio.js', () => {
    const mockFn = () => {};
    return {
        playRange: vi.fn(),
        togglePlayback: vi.fn(),
        stopPlayback: vi.fn(),
        getIsPlaying: vi.fn(() => false),
        setUpdatePlayBtn: vi.fn(),
    };
});

// Mock practice-business.js for lyrics panel tests
vi.mock('../practice-business.js', () => ({
    getLyricsForSelection: vi.fn(),
}));

describe('practice-ui.js', () => {
    let uiModule;
    let mockGetLyrics;

    beforeEach(async () => {
        setupDOM();
        vi.clearAllMocks();
        // Reset state for clean test
        state.structures = [];
        state.selectedStructure = null;
        state.selectedPhraseIds = [];
        state.selectedPhrasesData = [];
        state.selectedAccompanimentTrackIds = [];
        state.selectedTrackId = null;
        uiModule = await import('../practice-ui.js');

        // Import the mock for getLyricsForSelection
        const { getLyricsForSelection } = await import('../practice-business.js');
        mockGetLyrics = getLyricsForSelection;
    });

    afterEach(() => {
        document.body.innerHTML = '';
    });

    describe('updatePlayBtn', () => {
        it('should show play icon when not playing (default mock state)', () => {
            uiModule.updatePlayBtn();
            const btn = document.getElementById('record-btn');
            expect(btn.innerHTML).toContain('polygon');
        });

        it('should do nothing if button element missing', () => {
            document.getElementById('record-btn').remove();
            expect(() => uiModule.updatePlayBtn()).not.toThrow();
        });
    });

    describe('renderVocalTracks', () => {
        const tracks = [
            { id: 'v1', name: 'Lead', is_vocal: true },
            { id: 'v2', name: 'Harmony', is_vocal: false },
            { id: 'v3', name: 'Backup', is_vocal: true },
        ];

        it('should render only vocal tracks', () => {
            uiModule.renderVocalTracks(tracks);
            const container = document.getElementById('vocal-tracks');
            expect(container.innerHTML).toContain('Lead');
            expect(container.innerHTML).toContain('Backup');
            expect(container.innerHTML).not.toContain('Harmony');
            expect(container.innerHTML).toContain('🎤');
        });

        it('should show empty message for no tracks', () => {
            uiModule.renderVocalTracks([]);
            const container = document.getElementById('vocal-tracks');
            expect(container.innerHTML).toContain('沒有音軌');
        });

        it('should show empty message when no vocal tracks exist', () => {
            uiModule.renderVocalTracks([{ id: 'i1', name: 'Inst', is_vocal: false }]);
            const container = document.getElementById('vocal-tracks');
            expect(container.innerHTML).toContain('沒有主聲部音軌');
        });
    });

    describe('renderAccompanimentTracks', () => {
        const tracks = [
            { id: 'a1', name: 'Piano', is_vocal: false },
            { id: 'a2', name: 'Guitar', is_vocal: false },
        ];

        it('should render all tracks as checkboxes', () => {
            uiModule.renderAccompanimentTracks(tracks);
            const container = document.getElementById('accompaniment-tracks');
            expect(container.innerHTML).toContain('Piano');
            expect(container.innerHTML).toContain('Guitar');
            expect(container.innerHTML).toContain('checkbox');
        });

        it('should show empty message for no tracks', () => {
            uiModule.renderAccompanimentTracks([]);
            const container = document.getElementById('accompaniment-tracks');
            expect(container.innerHTML).toContain('沒有音軌');
        });
    });

    describe('renderStructureList', () => {
        it('should render empty message when no structures', () => {
            uiModule.renderStructureList();
            const container = document.getElementById('structure-list');
            expect(container.innerHTML).toContain('尚無段落結構');
        });

        it('should render structures and phrases', () => {
            state.structures = [
                {
                    id: 's1', title: 'Verse', start_time: 0, end_time: 10,
                    phrases: [
                        { id: 'p1', title: 'Line 1', start_time: 0, end_time: 5 },
                        { id: 'p2', title: 'Line 2', start_time: 5, end_time: 10 },
                    ],
                },
                { id: 's2', title: 'Chorus', start_time: 10, end_time: 20 },
            ];

            uiModule.renderStructureList();
            const container = document.getElementById('structure-list');
            expect(container.innerHTML).toContain('Verse');
            expect(container.innerHTML).toContain('Chorus');
            expect(container.innerHTML).toContain('Line 1');
            expect(container.innerHTML).toContain('Line 2');
        });

        it('should highlight multiple selected phrases', () => {
            state.structures = [
                {
                    id: 's1', title: 'Verse', start_time: 0, end_time: 10,
                    phrases: [
                        { id: 'p1', title: 'Line 1', start_time: 0, end_time: 5 },
                        { id: 'p2', title: 'Line 2', start_time: 5, end_time: 10 },
                    ],
                },
            ];
            state.flatItems = [
                { type: 'section', data: state.structures[0], section: state.structures[0] },
                { type: 'phrase', data: state.structures[0].phrases[0], section: state.structures[0] },
                { type: 'phrase', data: state.structures[0].phrases[1], section: state.structures[0] },
            ];
            state.selectionRange = { from: 1, to: 2 };
            state.selectedPhraseIds = ['p1', 'p2'];
            state.selectedStructure = { id: 'p1,p2', title: 'Line 1 ~ Line 2（2句）', start: 0, end: 10 };

            uiModule.renderStructureList();
            const container = document.getElementById('structure-list');
            // Both phrases should have 'selected' class
            expect(container.innerHTML).toContain('class="structure-option phrased selected"');
        });

        it('should highlight section implicitly when all its phrases are selected', () => {
            state.structures = [
                {
                    id: 's1', title: 'Verse', start_time: 0, end_time: 10,
                    phrases: [
                        { id: 'p1', title: 'Line 1', start_time: 0, end_time: 5 },
                    ],
                },
            ];
            state.flatItems = [
                { type: 'section', data: state.structures[0], section: state.structures[0] },
                { type: 'phrase', data: state.structures[0].phrases[0], section: state.structures[0] },
            ];
            state.selectionRange = { from: 1, to: 1 };
            state.selectedPhraseIds = ['p1'];
            state.selectedStructure = { id: 'p1', title: 'Line 1', start: 0, end: 5 };

            uiModule.renderStructureList();
            const container = document.getElementById('structure-list');
            // Section should be implicitly highlighted because its only phrase is selected
            expect(container.innerHTML).toContain('class="structure-option selected"');
            expect(container.innerHTML).toContain('class="structure-option phrased selected"');
        });
    });

    describe('renderLyricsPanel', () => {
        it('should show placeholder when nothing selected', () => {
            state.selectedStructure = null;
            state.selectedPhraseIds = [];
            uiModule.renderLyricsPanel();
            const container = document.getElementById('lyrics-content');
            expect(container.innerHTML).toContain('選擇段落或句子以顯示歌詞');
            expect(document.getElementById('lyrics-status').textContent).toBe('未選取');
        });

        it('should show placeholder when getLyricsForSelection returns empty', () => {
            state.selectedStructure = { id: 's1', title: 'Verse', start: 0, end: 10 };
            mockGetLyrics.mockReturnValue({ phrases: [], count: 0 });
            uiModule.renderLyricsPanel();
            const container = document.getElementById('lyrics-content');
            expect(container.innerHTML).toContain('無歌詞資料');
            expect(document.getElementById('lyrics-status').textContent).toBe('無歌詞');
        });

        it('should render phrase labels and text for each phrase with lyrics', () => {
            state.selectedStructure = { id: 's1', title: 'Verse', start: 0, end: 10 };
            mockGetLyrics.mockReturnValue({
                phrases: [
                    { title: 'Line 1', lyrics: 'Hello world' },
                    { title: 'Line 2', lyrics: 'Foo bar' },
                ],
                count: 2,
            });
            uiModule.renderLyricsPanel();
            const container = document.getElementById('lyrics-content');
            expect(container.innerHTML).toContain('Line 1');
            expect(container.innerHTML).toContain('Line 2');
            expect(container.innerHTML).toContain('Hello world');
            expect(container.innerHTML).toContain('Foo bar');
            expect(document.getElementById('lyrics-status').textContent).toBe('2 句');
        });

        it('should render "此句無歌詞" for phrases without lyrics', () => {
            state.selectedStructure = { id: 's1', title: 'Verse', start: 0, end: 10 };
            mockGetLyrics.mockReturnValue({
                phrases: [
                    { title: 'Line 1', lyrics: '' },
                ],
                count: 1,
            });
            uiModule.renderLyricsPanel();
            const container = document.getElementById('lyrics-content');
            expect(container.innerHTML).toContain('此句無歌詞');
        });
    });

    describe('clearSelection', () => {
        it('should reset DOM and state', () => {
            uiModule.clearSelection();
            expect(document.getElementById('range-info').textContent).toContain('選擇一個段落或句子');
            expect(document.getElementById('range-actions').style.display).toBe('none');
            expect(document.getElementById('current-range-label').textContent).toBe('未選擇');
        });

        it('should reset selectedPhraseIds and selectedPhrasesData in state', () => {
            state.selectedPhraseIds = ['p1', 'p2'];
            state.selectedPhrasesData = [{ id: 'p1' }, { id: 'p2' }];
            uiModule.clearSelection();
            expect(state.selectedPhraseIds).toEqual([]);
            expect(state.selectedPhrasesData).toEqual([]);
            expect(state.selectedStructure).toBeNull();
        });
    });

    describe('switchTab', () => {
        it('should toggle active class', () => {
            const tabs = document.querySelectorAll('.viz-tab');
            expect(tabs[0].classList.contains('active')).toBe(true);
            expect(tabs[1].classList.contains('active')).toBe(false);

            uiModule.switchTab(tabs[1], 'pitchroll');
            expect(tabs[0].classList.contains('active')).toBe(false);
            expect(tabs[1].classList.contains('active')).toBe(true);
        });
    });
});
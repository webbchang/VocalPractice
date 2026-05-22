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

describe('practice-ui.js', () => {
    let uiModule;

    beforeEach(async () => {
        setupDOM();
        vi.clearAllMocks();
        // Reset state for clean test
        state.structures = [];
        state.selectedStructure = null;
        state.selectedAccompanimentTrackIds = [];
        state.selectedTrackId = null;
        uiModule = await import('../practice-ui.js');
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
    });

    describe('clearSelection', () => {
        it('should reset DOM and state', () => {
            uiModule.clearSelection();
            expect(document.getElementById('range-info').textContent).toContain('選擇一個段落或句子');
            expect(document.getElementById('range-actions').style.display).toBe('none');
            expect(document.getElementById('current-range-label').textContent).toBe('未選擇');
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
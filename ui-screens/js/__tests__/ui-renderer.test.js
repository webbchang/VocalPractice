import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { StructureRenderer, UIStatus } from '../adapters/ui-renderer.js';

function setupDOM() {
    document.body.innerHTML = `
        <div id="timeline"></div>
        <div id="waveform-placeholder">📊 請先選擇歌曲</div>
        <div id="structure-tree"><div>請先選擇歌曲</div></div>
        <div id="stat-sections">0</div>
        <div id="stat-phrases">0</div>
        <div id="stat-duration">0:00</div>
        <select id="song-select"><option value="">請選擇歌曲...</option></select>
        <select id="lyrics-track-select"><option value="">請先載入歌曲...</option></select>
        <div id="edit-panel" style="display:none;">
            <input id="edit-title" value="">
            <select id="edit-type"><option value="SECTION">SECTION</option><option value="PHRASE">PHRASE</option></select>
            <input id="edit-start" value="0">
            <input id="edit-end" value="0">
            <input id="edit-start-tick" value="0">
            <input id="edit-end-tick" value="0">
            <input id="edit-order" value="1">
            <select id="edit-parent"><option value="">無</option></select>
            <textarea id="edit-lyrics"></textarea>
            <div id="edit-lyrics-row" style="display:none;"></div>
            <div id="section-lyrics-hint" style="display:none;"></div>
        </div>
        <div id="import-result"></div>
        <button id="copy-structures-btn" style="display:none;">📋 複製</button>
    `;
}

afterEach(() => {
    document.body.innerHTML = '';
});

describe('StructureRenderer', () => {
    beforeEach(() => {
        setupDOM();
    });

    describe('renderTimeline', () => {
        it('should show empty message when no structures', () => {
            StructureRenderer.renderTimeline([]);
            expect(document.getElementById('timeline').innerHTML).toContain('尚無段落結構');
            expect(document.getElementById('waveform-placeholder').textContent).toContain('尚無段落結構');
        });

        it('should render section blocks', () => {
            const structures = [
                { id: 's1', title: 'Verse', start_time: 0, end_time: 10, phrases: [] }
            ];
            StructureRenderer.renderTimeline(structures);
            const timeline = document.getElementById('timeline');
            expect(timeline.innerHTML).toContain('Verse');
            expect(timeline.querySelectorAll('.section-block').length).toBe(1);
        });

        it('should render section and phrase blocks', () => {
            const structures = [
                {
                    id: 's1', title: 'Verse', start_time: 0, end_time: 10,
                    phrases: [
                        { id: 'p1', title: 'Line 1', start_time: 0, end_time: 5 },
                        { id: 'p2', title: 'Line 2', start_time: 5, end_time: 10 }
                    ]
                }
            ];
            StructureRenderer.renderTimeline(structures);
            const blocks = document.getElementById('timeline').querySelectorAll('.section-block');
            // 1 section + 2 phrases = 3 blocks
            expect(blocks.length).toBe(3);
            expect(blocks[0].textContent).toBe('Verse');
            expect(blocks[1].textContent).toBe('Line 1');
            expect(blocks[2].textContent).toBe('Line 2');
        });

        it('should set data attributes for event delegation', () => {
            const structures = [
                {
                    id: 's1', title: 'Verse', start_time: 0, end_time: 10,
                    phrases: [{ id: 'p1', title: 'Phrase', start_time: 0, end_time: 5 }]
                }
            ];
            StructureRenderer.renderTimeline(structures);
            const blocks = document.getElementById('timeline').querySelectorAll('.section-block');
            expect(blocks[0].dataset.structureId).toBe('s1');
            expect(blocks[0].dataset.structureType).toBe('SECTION');
            expect(blocks[1].dataset.structureId).toBe('p1');
            expect(blocks[1].dataset.structureType).toBe('PHRASE');
        });
    });

    describe('renderSongsDropdown', () => {
        it('should populate song select options', () => {
            const songs = [
                { id: '1', title: 'Song A', artist: 'Artist A' },
                { id: '2', title: 'Song B', artist: null }
            ];
            StructureRenderer.renderSongsDropdown(songs);
            const select = document.getElementById('song-select');
            expect(select.options.length).toBe(3); // default + 2 songs
            expect(select.options[1].textContent).toContain('Song A');
            expect(select.options[2].textContent).toContain('Song B');
        });

        it('should handle empty songs list', () => {
            StructureRenderer.renderSongsDropdown([]);
            const select = document.getElementById('song-select');
            expect(select.options.length).toBe(1);
            expect(select.options[0].value).toBe('');
        });
    });

    describe('renderTrackSelector', () => {
        it('should populate track select options', () => {
            const tracks = [
                { id: 't1', name: 'Lead', is_vocal: true, note_count: 100 },
                { id: 't2', name: 'Piano', is_vocal: false, note_count: 200 }
            ];
            StructureRenderer.renderTrackSelector(tracks);
            const select = document.getElementById('lyrics-track-select');
            expect(select.options.length).toBe(3); // default + 2 tracks
            expect(select.options[1].textContent).toContain('Lead');
            expect(select.options[1].textContent).toContain('🎤');
            expect(select.options[2].textContent).toContain('Piano');
        });
    });

    describe('renderTree', () => {
        it('should show empty message when no structures', () => {
            StructureRenderer.renderTree([], 'track-1');
            expect(document.getElementById('structure-tree').innerHTML).toContain('尚無段落結構');
        });

        it('should render sections with play/edit/delete buttons', () => {
            const structures = [
                { id: 's1', title: 'Verse', start_time: 0, end_time: 10, order_index: 1, phrases: [] }
            ];
            StructureRenderer.renderTree(structures, null);
            const html = document.getElementById('structure-tree').innerHTML;
            expect(html).toContain('Verse');
            expect(html).toContain('data-action="play"');
            expect(html).toContain('data-action="edit"');
            expect(html).toContain('data-action="delete"');
            expect(html).toContain('data-action="add-phrase"');
        });

        it('should render phrases with lyrics text, edit/delete buttons, and edit hint', () => {
            const structures = [
                {
                    id: 's1', title: 'Verse', start_time: 0, end_time: 10, order_index: 1,
                    phrases: [
                        { id: 'p1', title: 'Phrase 1', start_time: 0, end_time: 5, lyrics: 'Hello' }
                    ]
                }
            ];
            StructureRenderer.renderTree(structures, null);
            const html = document.getElementById('structure-tree').innerHTML;
            expect(html).toContain('Phrase 1');
            expect(html).toContain('class="lyrics-text"');
            expect(html).toContain('Hello');
            expect(html).toContain('data-action="edit"');
            expect(html).toContain('data-action="delete"');
            expect(html).toContain('data-type="PHRASE"');
            expect(html).toContain('歌詞在編輯時修改');
        });
    });

    describe('hideEditPanel', () => {
        it('should hide edit panel', () => {
            document.getElementById('edit-panel').style.display = 'block';
            StructureRenderer.hideEditPanel();
            expect(document.getElementById('edit-panel').style.display).toBe('none');
        });
    });
});

describe('UIStatus', () => {
    beforeEach(() => {
        setupDOM();
    });

    describe('updateWaveformStatus', () => {
        it('should update placeholder text', () => {
            UIStatus.updateWaveformStatus('MIDI loaded', false);
            expect(document.getElementById('waveform-placeholder').textContent).toBe('MIDI loaded');
        });

        it('should set error color', () => {
            UIStatus.updateWaveformStatus('Error', true);
            expect(document.getElementById('waveform-placeholder').style.color).toBe('rgb(231, 76, 60)');
        });
    });

    describe('setPlayButton', () => {
        it('should set playing state', () => {
            const btn = document.createElement('button');
            UIStatus.setPlayButton(btn, true);
            expect(btn.textContent).toBe('⏹');
            expect(btn.dataset.playing).toBe('true');
        });

        it('should set stopped state', () => {
            const btn = document.createElement('button');
            UIStatus.setPlayButton(btn, false);
            expect(btn.textContent).toBe('▶️');
            expect(btn.dataset.playing).toBe('false');
        });
    });

    describe('updateStatCounters', () => {
        it('should update all counters', () => {
            UIStatus.updateStatCounters({ sections: 3, phrases: 10, duration: '2:30' });
            expect(document.getElementById('stat-sections').textContent).toBe('3');
            expect(document.getElementById('stat-phrases').textContent).toBe('10');
            expect(document.getElementById('stat-duration').textContent).toBe('2:30');
        });

        it('should handle partial updates', () => {
            UIStatus.updateStatCounters({ sections: 5 });
            expect(document.getElementById('stat-sections').textContent).toBe('5');
            expect(document.getElementById('stat-phrases').textContent).toBe('0'); // unchanged from setup
        });
    });

    describe('resetDashboard', () => {
        it('should reset all dashboard elements', () => {
            UIStatus.resetDashboard();
            expect(document.getElementById('structure-tree').innerHTML).toContain('請先選擇歌曲');
            expect(document.getElementById('waveform-placeholder').textContent).toBe('📊 請先選擇歌曲');
            expect(document.getElementById('stat-sections').textContent).toBe('0');
            expect(document.getElementById('stat-phrases').textContent).toBe('0');
            expect(document.getElementById('stat-duration').textContent).toBe('0:00');
        });
    });

    describe('showLoading', () => {
        it('should show loading message', () => {
            UIStatus.showLoading('Loading songs...');
            expect(document.getElementById('waveform-placeholder').textContent).toBe('⏳ Loading songs...');
        });
    });

    describe('setImportResult', () => {
        it('should set success message', () => {
            UIStatus.setImportResult('Success', 'success');
            expect(document.getElementById('import-result').textContent).toBe('Success');
            expect(document.getElementById('import-result').style.color).toBe('rgb(39, 174, 96)');
        });

        it('should set error message', () => {
            UIStatus.setImportResult('Failed', 'error');
            expect(document.getElementById('import-result').style.color).toBe('rgb(231, 76, 60)');
        });
    });

    describe('getEditPanelData', () => {
        it('should collect data from edit panel', () => {
            document.getElementById('edit-title').value = 'Test';
            document.getElementById('edit-type').value = 'SECTION';
            document.getElementById('edit-start').value = '1.5';
            document.getElementById('edit-end').value = '5.0';
            document.getElementById('edit-start-tick').value = '100';
            document.getElementById('edit-end-tick').value = '400';
            document.getElementById('edit-order').value = '2';
            document.getElementById('edit-lyrics').value = 'Some lyrics';

            const data = UIStatus.getEditPanelData();
            expect(data.title).toBe('Test');
            expect(data.type).toBe('SECTION');
            expect(data.startTime).toBe(1.5);
            expect(data.endTime).toBe(5.0);
            expect(data.startTick).toBe(100);
            expect(data.endTick).toBe(400);
            expect(data.orderIdx).toBe(2);
            expect(data.lyrics).toBe('Some lyrics');
        });
    });
});
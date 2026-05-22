// practice-business.js — 純業務邏輯
// 依賴：state.js, api.js, audio.js, songDataExtractor.js (global)
// 不直接操作 DOM（UI 操作委託給 practice-ui.js）

import state from './state.js';
import { api, loadMIDI, loadStructures } from './api.js';
import { playRange, stopPlayback } from './audio.js';

function determineParsedTrackIndex() {
    const { currentSongFull, selectedTrackId } = state;
    if (!currentSongFull || !currentSongFull.tracks) return -1;
    for (const t of currentSongFull.tracks) {
        if (t.id === selectedTrackId) {
            return t.midi_index;
        }
    }
    return -1;
}

function generateReferenceData() {
    const { parsedNotes, selectedStructure, parsedTrackIndex } = state;
    if (!parsedNotes || !selectedStructure || parsedTrackIndex < 0) {
        state.referenceNotes = null;
        return;
    }
    const start = selectedStructure.start;
    const end = selectedStructure.end;
    state.referenceNotes = window.SongDataExtractor.extractReferenceNotes(
        parsedNotes,
        parsedTrackIndex,
        start,
        end
    );
    console.log(`Reference data: ${state.referenceNotes.length} notes from track ${parsedTrackIndex} [${start.toFixed(2)}-${end.toFixed(2)}]`);
}

export async function selectTrack(trackId) {
    state.selectedTrackId = trackId;
    state.referenceNotes = null;
    state.parsedTrackIndex = determineParsedTrackIndex();
    // Re-render both vocal and accompaniment track lists
    const { renderVocalTracks, renderAccompanimentTracks } = await import('./practice-ui.js');
    if (state.currentSongFull && state.currentSongFull.tracks) {
        renderVocalTracks(state.currentSongFull.tracks);
        renderAccompanimentTracks(state.currentSongFull.tracks);
    }
    const structures = await loadStructures(state.currentSongId, trackId);
    state.structures = structures;
    const { renderStructureList } = await import('./practice-ui.js');
    renderStructureList();
}

export function selectStructure(id) {
    const { structures } = state;
    for (const s of structures) {
        if (s.id === id) {
            state.selectedStructure = { id: s.id, title: s.title, start: s.start_time, end: s.end_time };
            generateReferenceData();
            updateSelectionUI();
            return;
        }
        if (s.phrases) {
            for (const p of s.phrases) {
                if (p.id === id) {
                    state.selectedStructure = { id: p.id, title: p.title, start: p.start_time, end: p.end_time };
                    generateReferenceData();
                    updateSelectionUI();
                    return;
                }
            }
        }
    }
}

function updateSelectionUI() {
    const { selectedStructure } = state;
    if (selectedStructure) {
        const formatTime = (sec) => {
            if (sec == null) return '0:00';
            const m = Math.floor(sec / 60);
            const s = Math.floor(sec % 60);
            return m + ':' + s.toString().padStart(2, '0');
        };
        document.getElementById('range-info').textContent =
            `🎯 ${selectedStructure.title}：${formatTime(selectedStructure.start)} - ${formatTime(selectedStructure.end)}`;
        document.getElementById('range-actions').style.display = 'flex';
        document.getElementById('current-range-label').textContent =
            `${selectedStructure.title} (${formatTime(selectedStructure.start)})`;
        // re-render structure list to show selection
        import('./practice-ui.js').then(m => m.renderStructureList());
    }
}

export async function onSongChange() {
    const select = document.getElementById('song-select');
    state.currentSongId = select.value || null;
    state.selectedStructure = null;
    document.getElementById('range-actions').style.display = 'none';
    document.getElementById('range-info').textContent = '選擇一個段落或句子開始練習';

    if (!state.currentSongId) {
        document.getElementById('song-title').textContent = '請選擇歌曲';
        document.getElementById('song-artist').textContent = '';
        document.getElementById('vocal-tracks').innerHTML = '<div class="no-structures">請先選擇歌曲</div>';
        document.getElementById('structure-list').innerHTML = '<div class="no-structures">請先選擇歌曲</div>';
        return;
    }

    try {
        const song = await api('/songs/' + state.currentSongId);
        state.currentSongFull = song;
        document.getElementById('song-title').textContent = song.title;
        document.getElementById('song-artist').textContent = song.artist;

        // Load MIDI for playback
        const midiData = await loadMIDI(state.currentSongId);
        state.midiData = midiData;
        state.parsedNotes = midiData ? window.MidiParser.parseMIDINotes(midiData) : null;

        // Render vocal tracks and accompaniment tracks
        const { renderVocalTracks, renderAccompanimentTracks } = await import('./practice-ui.js');
        renderVocalTracks(song.tracks);
        renderAccompanimentTracks(song.tracks);

        // Load structures for first track if available
        if (song.tracks.length > 0) {
            state.selectedTrackId = song.tracks[0].id;
            await selectTrack(song.tracks[0].id);
        }
    } catch (err) {
        console.error('Failed to load song:', err);
    }
}

export function toggleAccompanimentTrack(trackId) {
    const idx = state.selectedAccompanimentTrackIds.indexOf(trackId);
    if (idx === -1) {
        state.selectedAccompanimentTrackIds.push(trackId);
    } else {
        state.selectedAccompanimentTrackIds.splice(idx, 1);
    }
    // Re-render
    if (state.currentSongFull && state.currentSongFull.tracks) {
        import('./practice-ui.js').then(m => m.renderAccompanimentTracks(state.currentSongFull.tracks));
    }
}

export function startPractice() {
    const { selectedStructure, referenceNotes } = state;
    if (!selectedStructure) {
        alert('請先選擇一個段落或句子');
        return;
    }
    if (!referenceNotes || referenceNotes.length === 0) {
        alert('此段落尚無比對資料，請確認已選取正確的聲部');
        return;
    }
    const formatTime = (sec) => {
        if (sec == null) return '0:00';
        const m = Math.floor(sec / 60);
        const s = Math.floor(sec % 60);
        return m + ':' + s.toString().padStart(2, '0');
    };
    document.getElementById('status-text').textContent = `練習：${selectedStructure.title}`;
    document.getElementById('chart-area').innerHTML = `
        <div>
            <div style="font-size:48px;margin-bottom:12px;">🎯</div>
            <div>準備練習「${selectedStructure.title}」</div>
            <div style="color:#888;font-size:13px;margin-top:8px;">
                ${formatTime(selectedStructure.start)} → ${formatTime(selectedStructure.end)}
                (${(selectedStructure.end - selectedStructure.start).toFixed(1)}秒)
            </div>
            <div style="color:#27ae60;font-size:12px;margin-top:6px;">
                比對基準：${referenceNotes.length} 個音符（用戶端 MIDI 即時產生）
            </div>
        </div>
    `;
    console.log('Reference notes for comparison:', referenceNotes);
    // Auto-play the range
    playRange(selectedStructure.start, selectedStructure.end);
}
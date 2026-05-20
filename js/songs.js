// === Songs Module ===
import { state } from './state.js';
import { api } from './api.js';
import { escapeHtml, sortStructures } from './utils.js';
import { showScreen } from './navigation.js';
import { renderTrackSelection } from './track-selection.js';

export async function loadSongs() {
    try {
        state.songs = await api('/songs');
        renderSongList();
        showScreen('songs-screen');
    } catch (err) {
        alert('載入歌曲失敗: ' + err.message);
    }
}

function renderSongList() {
    const container = document.getElementById('song-list');
    container.innerHTML = state.songs.map(song => `
        <div class="list-item" data-action="selectSong" data-id="${song.id}">
            <div class="name">${escapeHtml(song.title)}</div>
            <div class="meta">${escapeHtml(song.artist)} · ${song.tracks.length} 個聲部</div>
        </div>
    `).join('');
}

export async function selectSong(songId) {
    state.selectedSong = state.songs.find(s => s.id === songId);
    if (!state.selectedSong) return;

    // Load structures
    try {
        const result = await api(`/songs/${songId}/structures`);
        state.structures = result.structures || [];
        // Sort structures by order_index
        sortStructures(state.structures);
    } catch {
        state.structures = [];
    }

    renderTrackSelection();
    showScreen('tracks-screen');
}

export function goBackToSongs() {
    showScreen('songs-screen');
}
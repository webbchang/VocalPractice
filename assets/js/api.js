// api.js — API 通訊層
// 處理所有後端請求，回傳資料

const API_BASE = '/api/v1';

export async function api(path, options = {}) {
    const token = localStorage.getItem('token');
    const { headers: optHeaders, ...rest } = options;
    const headers = {
        'Content-Type': 'application/json',
        ...optHeaders,
    };
    if (token) headers['Authorization'] = `Bearer ${token}`;
    const res = await fetch(API_BASE + path, {
        credentials: 'same-origin',
        headers,
        ...rest,
    });
    if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        throw new Error(err.error || `HTTP ${res.status}`);
    }
    return res.json();
}

export function formatTime(sec) {
    if (sec == null) return '0:00';
    const m = Math.floor(sec / 60);
    const s = Math.floor(sec % 60);
    return m + ':' + s.toString().padStart(2, '0');
}

export async function loadSongs() {
    try {
        const songs = await api('/songs');
        const select = document.getElementById('song-select');
        select.innerHTML = '<option value="">請選擇歌曲...</option>';
        for (const s of songs) {
            const opt = document.createElement('option');
            opt.value = s.id;
            opt.textContent = s.title + ' - ' + (s.artist || '未知');
            select.appendChild(opt);
        }
    } catch (err) {
        console.error('Failed to load songs:', err);
    }
}

export async function loadMIDI(currentSongId) {
    if (!currentSongId) return null;
    try {
        const token = localStorage.getItem('token');
        const headers = {};
        if (token) headers['Authorization'] = `Bearer ${token}`;
        const res = await fetch(API_BASE + '/songs/' + currentSongId + '/midi', {
            credentials: 'same-origin',
            headers,
        });
        if (!res.ok) throw new Error('Failed to load MIDI');
        return await res.arrayBuffer();
    } catch (err) {
        console.error('MIDI load error:', err);
        return null;
    }
}

export async function loadStructures(currentSongId, trackId) {
    if (!currentSongId) return [];
    const container = document.getElementById('structure-list');
    container.innerHTML = '<div class="loading-spinner">載入段落結構...</div>';

    try {
        let path = '/songs/' + currentSongId + '/structures';
        if (trackId) path += '?track_id=' + encodeURIComponent(trackId);
        const data = await api(path);
        return data.structures || [];
    } catch (err) {
        console.error('Failed to load structures:', err);
        container.innerHTML = '<div class="no-structures">無法載入段落結構</div>';
        return [];
    }
}
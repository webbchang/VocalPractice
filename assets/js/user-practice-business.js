const API_BASE = '/api/v1';

// --- State ---
let currentSongId = null;
let currentSongFull = null; // full song with tempo_map etc
let structures = [];
let selectedTrackId = null;
let selectedAccompanimentTrackIds = []; // multi-select accompaniment
let selectedStructure = null; // { id, start, end, title }
let midiData = null;
let parsedNotes = null;       // all parsed notes from MIDI, each has .track index
let referenceNotes = null;    // filtered notes for the selected track+section
let allTracksMeta = [];       // meta info from GET /songs/:id to map track index to UUID
let parsedTrackIndex = null;  // which parsedNotes track index corresponds to selectedTrackId
let parsedMIDITracks = [];    // track info from MIDI parsing (name, track index)

// --- Audio playback ---
let audioCtx = null;
let playbackNodes = [];
let playbackTimer = null;
let isPlaying = false;

function getAudioCtx() {
    if (!audioCtx) audioCtx = new (window.AudioContext || window.webkitAudioContext)();
    return audioCtx;
}

function parseMIDINotes(midiArrayBuf) {
    return window.MidiParser.parseMIDINotes(midiArrayBuf);
}

function midiPitchToFreq(pitch) {
    return window.MidiParser.midiPitchToFreq(pitch);
}

async function loadMIDI() {
    if (!currentSongId) return;
    try {
        const headers = {};
        const token = localStorage.getItem('token');
        if (token) {
            headers['Authorization'] = `Bearer ${token}`;
        }
        const res = await fetch(API_BASE + '/songs/' + currentSongId + '/midi', {
            credentials: 'same-origin',
            headers
        });
        if (!res.ok) throw new Error('Failed to load MIDI');
        midiData = await res.arrayBuffer();
        parsedNotes = parseMIDINotes(midiData);
    } catch (err) {
        console.error('MIDI load error:', err);
        midiData = null; parsedNotes = null;
    }
}

function playRange(startTime, endTime) {
    if (!parsedNotes || parsedNotes.length === 0) return;
    stopPlayback();
    const ctx = getAudioCtx();
    const gainNode = ctx.createGain();
    gainNode.gain.value = 0.3;
    gainNode.connect(ctx.destination);
    const dur = endTime - startTime;
    const now = ctx.currentTime;

    // 只播放選取的音軌：主聲部 + 伴唱聲部
    const accompanimentTrackIndices = selectedAccompanimentTrackIds
        .map(id => {
            if (!currentSongFull || !currentSongFull.tracks) return -1;
            for (const t of currentSongFull.tracks) {
                if (t.id === id) return t.midi_index;
            }
            return -1;
        })
        .filter(idx => idx >= 0);
    const playableTrackIndices = [parsedTrackIndex, ...accompanimentTrackIndices].filter(idx => idx >= 0);

    const rangeNotes = parsedNotes.filter(n =>
        n.start >= startTime && n.start < endTime &&
        playableTrackIndices.includes(n.track)
    );

    for (const n of rangeNotes) {
        const osc = ctx.createOscillator();
        const ng = ctx.createGain();
        osc.type = 'triangle';
        osc.frequency.value = midiPitchToFreq(n.pitch);
        const ls = n.start - startTime;
        const nd = Math.min(n.dur, dur - ls);
        if (nd <= 0.01) continue;
        ng.gain.setValueAtTime(0, now + ls);
        ng.gain.linearRampToValueAtTime(0.3, now + ls + 0.005);
        ng.gain.setValueAtTime(0.3, now + ls + nd - 0.01);
        ng.gain.linearRampToValueAtTime(0, now + ls + nd);
        osc.connect(ng); ng.connect(gainNode);
        osc.start(now + ls); osc.stop(now + ls + nd + 0.01);
        playbackNodes.push(osc, ng);
    }
    playbackTimer = setTimeout(() => { isPlaying = false; updatePlayBtn(); stopPlayback(); }, dur * 1000 + 300);
    isPlaying = true;
    updatePlayBtn();
}

function stopPlayback() {
    if (playbackTimer) { clearTimeout(playbackTimer); playbackTimer = null; }
    for (const n of playbackNodes) { try { n.disconnect(); } catch(e) {} }
    playbackNodes = [];
    isPlaying = false;
    updatePlayBtn();
}

async function api(path, options = {}) {
    const headers = { 'Content-Type': 'application/json', ...options.headers };
    const token = localStorage.getItem('token');
    if (token) {
        headers['Authorization'] = `Bearer ${token}`;
    }
    const res = await fetch(API_BASE + path, {
        credentials: 'same-origin',
        headers,
        ...options,
    });
    if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        throw new Error(err.error || `HTTP ${res.status}`);
    }
    return res.json();
}

function formatTime(sec) {
    if (sec == null) return '0:00';
    const m = Math.floor(sec / 60);
    const s = Math.floor(sec % 60);
    return m + ':' + s.toString().padStart(2, '0');
}

async function loadSongs() {
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

async function onSongChange() {
    const select = document.getElementById('song-select');
    currentSongId = select.value || null;
    selectedStructure = null;
    document.getElementById('range-actions').style.display = 'none';
    document.getElementById('range-info').textContent = '選擇一個段落或句子開始練習';

    if (!currentSongId) {
        document.getElementById('song-title').textContent = '請選擇歌曲';
        document.getElementById('song-artist').textContent = '';
        document.getElementById('vocal-tracks').innerHTML = '<div class="no-structures">請先選擇歌曲</div>';
        document.getElementById('structure-list').innerHTML = '<div class="no-structures">請先選擇歌曲</div>';
        return;
    }

    try {
        const song = await api('/songs/' + currentSongId);
        currentSongFull = song;
        document.getElementById('song-title').textContent = song.title;
        document.getElementById('song-artist').textContent = song.artist;

        // Load MIDI for playback
        await loadMIDI();

        // Render vocal tracks (radio) and accompaniment tracks (checkbox)
        renderVocalTracks(song.tracks);
        renderAccompanimentTracks(song.tracks);

        // Load structures for first track if available
        if (song.tracks.length > 0) {
            selectedTrackId = song.tracks[0].id;
            selectTrack(song.tracks[0].id);
        }
    } catch (err) {
        console.error('Failed to load song:', err);
    }
}

function toggleAccompanimentTrack(trackId) {
    const idx = selectedAccompanimentTrackIds.indexOf(trackId);
    if (idx === -1) {
        selectedAccompanimentTrackIds.push(trackId);
    } else {
        selectedAccompanimentTrackIds.splice(idx, 1);
    }
    // Re-render
    if (currentSongFull && currentSongFull.tracks) {
        renderAccompanimentTracks(currentSongFull.tracks);
    }
}

function determineParsedTrackIndex() {
    // Use the MIDI track index stored in the backend's midi_index
    // This matches the original MIDI track index used by parsedNotes
    if (!currentSongFull || !currentSongFull.tracks) return -1;
    for (const t of currentSongFull.tracks) {
        if (t.id === selectedTrackId) {
            return t.midi_index;
        }
    }
    return -1;
}

function generateReferenceData() {
    // Build reference notes from parsed MIDI: filter by track + time range.
    if (!parsedNotes || !selectedStructure || parsedTrackIndex < 0) {
        referenceNotes = null;
        return;
    }
    const start = selectedStructure.start;
    const end = selectedStructure.end;
    referenceNotes = window.SongDataExtractor.extractReferenceNotes(
        parsedNotes,
        parsedTrackIndex,
        start,
        end
    );
    console.log(`Reference data: ${referenceNotes.length} notes from track ${parsedTrackIndex} [${start.toFixed(2)}-${end.toFixed(2)}]`);
}

async function selectTrack(trackId) {
    selectedTrackId = trackId;
    referenceNotes = null;
    // Determine which MIDI track index corresponds to the selected UUID
    parsedTrackIndex = determineParsedTrackIndex();
    // Re-render both vocal and accompaniment track lists
    if (currentSongFull && currentSongFull.tracks) {
        renderVocalTracks(currentSongFull.tracks);
        renderAccompanimentTracks(currentSongFull.tracks);
    }
    await loadStructures(trackId);
}

async function loadStructures(trackId) {
    if (!currentSongId) return;
    const container = document.getElementById('structure-list');
    container.innerHTML = '<div class="loading-spinner">載入段落結構...</div>';

    try {
        let path = '/songs/' + currentSongId + '/structures';
        if (trackId) path += '?track_id=' + encodeURIComponent(trackId);
        const data = await api(path);
        structures = data.structures || [];
        renderStructureList();
    } catch (err) {
        console.error('Failed to load structures:', err);
        container.innerHTML = '<div class="no-structures">無法載入段落結構</div>';
    }
}

function selectStructure(id) {
    // Find structure by id
    for (const s of structures) {
        if (s.id === id) {
            selectedStructure = { id: s.id, title: s.title, start: s.start_time, end: s.end_time };
            generateReferenceData();
            updateSelectionUI();
            return;
        }
        if (s.phrases) {
            for (const p of s.phrases) {
                if (p.id === id) {
                    selectedStructure = { id: p.id, title: p.title, start: p.start_time, end: p.end_time };
                    generateReferenceData();
                    updateSelectionUI();
                    return;
                }
            }
        }
    }
}

function startPractice() {
    if (!selectedStructure) {
        alert('請先選擇一個段落或句子');
        return;
    }
    if (!referenceNotes || referenceNotes.length === 0) {
        alert('此段落尚無比對資料，請確認已選取正確的聲部');
        return;
    }
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

// --- Init ---
loadSongs();

// Expose functions to the global scope if needed by UI
window.togglePlayback = togglePlayback;
window.onSongChange = onSongChange;
window.selectTrack = selectTrack;
window.toggleAccompanimentTrack = toggleAccompanimentTrack;
window.selectStructure = selectStructure;
window.clearSelection = clearSelection;
window.startPractice = startPractice;
window.logout = logout; // Assuming logout might be business logic
window.switchTab = switchTab; // Assuming switchTab might be UI related, but needs access to some state. If pure UI, it moves to UI file.
window.renderVocalTracks = renderVocalTracks;
window.renderAccompanimentTracks = renderAccompanimentTracks;
window.renderStructureList = renderStructureList;
window.updateSelectionUI = updateSelectionUI;
window.updatePlayBtn = updatePlayBtn; 

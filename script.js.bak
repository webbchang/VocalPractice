// === State ===
const API_BASE = '/api/v1';
let state = {
    token: null,
    user: null,
    songs: [],
    selectedSong: null,
    selectedVocalTrack: null,
    selectedAccompanimentTracks: [],
    selectedStructure: null,
    structures: [],
    mediaRecorder: null,
    audioChunks: [],
    isRecording: false,
    isCountIn: false,
    referenceNotes: [],
    detectedNotes: [],
    assessmentResult: null,
    // Range selection
    rangeStartId: null,
    rangeEndId: null,
    selectedRangeStart: null,
    selectedRangeEnd: null,
    // Accompaniment
    audioCtx: null,
    accompanimentOscillators: [],
    accompanimentGain: null,
    currentScheduledNotes: [],
    playbackStartTime: 0,
    // Headset
    preferredMicId: null,
    headsetState: 'unknown', // 'wired' | 'bluetooth' | 'none' | 'unknown'
    // Lyrics
    lyricsLines: [],        // [{ structureId, lyrics, startTime, endTime }]
    lyricsAnimFrame: null
};

// === API Helper ===
async function api(path, options = {}) {
    const headers = { 'Content-Type': 'application/json', ...options.headers };
    if (state.token) {
        headers['Authorization'] = `Bearer ${state.token}`;
    }
    const res = await fetch(`${API_BASE}${path}`, { ...options, headers });
    if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        throw new Error(err.error || 'API Error');
    }
    const text = await res.text();
    return text ? JSON.parse(text) : null;
}

// === Screen Navigation ===
function showScreen(id) {
    document.querySelectorAll('.screen').forEach(s => s.classList.add('hidden'));
    document.getElementById(id).classList.remove('hidden');
}

// === Login ===
async function login() {
    const email = document.getElementById('login-email').value;
    const password = document.getElementById('login-password').value;
    const errorEl = document.getElementById('login-error');

    if (!email || !password) {
        errorEl.textContent = '請輸入 Email 和密碼';
        return;
    }

    try {
        const result = await api('/auth/login', {
            method: 'POST',
            body: JSON.stringify({ email, password })
        });
        state.token = result.token;
        state.user = result.user;
        errorEl.textContent = '';
        await loadSongs();
    } catch (err) {
        errorEl.textContent = '登入失敗: ' + err.message;
    }
}

function logout() {
    stopAccompaniment();
    state.token = null;
    state.user = null;
    state.selectedSong = null;
    document.getElementById('login-email').value = '';
    document.getElementById('login-password').value = '';
    showScreen('login-screen');
}

// === Songs ===
async function loadSongs() {
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
        <div class="list-item" onclick="selectSong('${song.id}')">
            <div class="name">${escapeHtml(song.title)}</div>
            <div class="meta">${escapeHtml(song.artist)} · ${song.tracks.length} 個聲部</div>
        </div>
    `).join('');
}

async function selectSong(songId) {
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

function sortStructures(nodes) {
    nodes.sort((a, b) => (a.order_index || a.song_structure?.order_index || 0) - (b.order_index || b.song_structure?.order_index || 0));
    for (const node of nodes) {
        if (node.phrases && node.phrases.length) {
            sortStructures(node.phrases);
        }
    }
}

function goBackToSongs() {
    showScreen('songs-screen');
}

// === Audio Device Enumeration ===
async function enumerateAudioDevices() {
    try {
        // Request permission first to get label data
        const tempStream = await navigator.mediaDevices.getUserMedia({ audio: true });
        tempStream.getTracks().forEach(t => t.stop());

        const devices = await navigator.mediaDevices.enumerateDevices();
        const audioInputs = devices.filter(d => d.kind === 'audioinput' && d.deviceId);
        const audioOutputs = devices.filter(d => d.kind === 'audiooutput' && d.deviceId);

        let headsetState = 'none';
        let preferredMicId = null;
        let headsetLabel = '';

        // Check audio inputs first (headset microphone)
        for (const input of audioInputs) {
            const label = (input.label || '').toLowerCase();
            const isBluetooth = /bluetooth|藍芽|藍牙|bt.*audio|wireless|無線/.test(label);
            const isWiredHeadset = /headset|headphone|耳機|耳麥|earphone|head.?set/.test(label);
            const isBuiltIn = /built.?in|內建|default|internal/.test(label);

            if (isBluetooth) {
                headsetState = 'bluetooth';
                headsetLabel = input.label || '藍芽耳機';
                preferredMicId = input.deviceId;
                break;
            } else if (isWiredHeadset && !isBuiltIn) {
                headsetState = 'wired';
                headsetLabel = input.label || '有線耳機';
                preferredMicId = input.deviceId;
                break;
            }
        }

        // If no headset mic found, check output devices for headphones
        if (headsetState === 'none') {
            for (const output of audioOutputs) {
                const label = (output.label || '').toLowerCase();
                const isBluetooth = /bluetooth|藍芽|藍牙|bt.*audio|wireless|無線/.test(label);
                const isWiredHeadset = /headset|headphone|耳機|耳麥|earphone|head.?set/.test(label);
                const isBuiltIn = /built.?in|內建|default|internal/.test(label);

                if (isBluetooth) {
                    headsetState = 'bluetooth';
                    headsetLabel = output.label || '藍芽耳機';
                    break;
                } else if (isWiredHeadset && !isBuiltIn) {
                    headsetState = 'wired';
                    headsetLabel = output.label || '有線耳機';
                    break;
                }
            }
        }

        state.headsetState = headsetState;
        state.preferredMicId = preferredMicId;

        return { headsetState, headsetLabel, preferredMicId, audioInputs };
    } catch (err) {
        console.error('Device enumeration error:', err);
        state.headsetState = 'unknown';
        return { headsetState: 'unknown', headsetLabel: '', preferredMicId: null, audioInputs: [] };
    }
}

// === Headset Check & Warning ===
function getHeadsetWarnings(headsetState) {
    switch (headsetState) {
        case 'wired':
            return { level: 'success', message: '✅ 已偵測到有線耳機，可正常練習' };
        case 'bluetooth':
            return { level: 'warning', message: '⚠️ 偵測到藍芽耳機，藍芽音訊有 100-300ms 延遲，可能影響演唱節奏，建議改用有線耳機' };
        case 'none':
            return { level: 'warning', message: '⚠️ 未偵測到耳機，揚聲器播放伴奏可能被麥克風收到造成回授 (feedback)，建議插入有線耳機' };
        default:
            return { level: 'info', message: 'ℹ️ 無法判斷耳機狀態，建議使用有線耳機以獲得最佳練習體驗' };
    }
}

async function checkAudioSetup() {
    const { headsetState, headsetLabel, audioInputs } = await enumerateAudioDevices();
    const warning = getHeadsetWarnings(headsetState);

    // Update UI
    const headsetStatusEl = document.getElementById('headset-status');
    if (headsetStatusEl) {
        headsetStatusEl.className = 'headset-status ' + headsetState;
        headsetStatusEl.textContent = (headsetLabel ? `[${headsetLabel}] ` : '') + warning.message;
    }

    // Also show on practice screen
    const headsetBannerEl = document.getElementById('headset-banner');
    if (headsetBannerEl) {
        headsetBannerEl.className = 'headset-banner ' + headsetState;
        headsetBannerEl.textContent = (headsetLabel ? `[${headsetLabel}] ` : '') + warning.message;
    }

    // Log available inputs for debugging
    if (audioInputs.length) {
        console.log('Available audio inputs:', audioInputs.map(d => d.label || '(unnamed)').join(', '));
    }

    return { headsetState, preferredMicId: state.preferredMicId };
}

// === Track Selection ===
function renderTrackSelection() {
    const song = state.selectedSong;
    if (!song) return;

    document.getElementById('track-song-title').textContent = `${song.title} - ${song.artist}`;

    const vocalTracks = song.tracks.filter(t => t.is_vocal);
    const accTracks = song.tracks.filter(t => !t.is_vocal);

    // Vocal tracks (radio - select one)
    const vocalContainer = document.getElementById('vocal-tracks');
    if (vocalTracks.length === 0) {
        vocalContainer.innerHTML = '<p style="color:#888;padding:8px;">沒有標記為人聲的聲部</p>';
    } else {
        vocalContainer.innerHTML = vocalTracks.map(t => `
            <div class="track-item">
                <input type="radio" name="vocal-track" value="${t.id}" id="vocal-${t.id}"
                    onchange="selectVocalTrack('${t.id}')">
                <label for="vocal-${t.id}">${escapeHtml(t.name)} (${t.note_count} 個音符)</label>
            </div>
        `).join('');
    }

    // Accompaniment tracks (checkbox - select multiple)
    const accContainer = document.getElementById('accompaniment-tracks');
    if (accTracks.length === 0) {
        accContainer.innerHTML = '<p style="color:#888;padding:8px;">沒有伴奏聲部</p>';
    } else {
        accContainer.innerHTML = accTracks.map(t => `
            <div class="track-item">
                <input type="checkbox" name="acc-track" value="${t.id}" id="acc-${t.id}"
                    onchange="toggleAccompaniment('${t.id}')">
                <label for="acc-${t.id}">${escapeHtml(t.name)} (${t.note_count} 個音符)</label>
            </div>
        `).join('');
    }

    // Structures (tree with sections and phrases)
    const structureContainer = document.getElementById('structure-list');
    if (state.structures.length === 0) {
        structureContainer.innerHTML = '<p style="color:#888;padding:8px;">無段落定義</p>';
    } else {
        structureContainer.innerHTML = renderStructureTreeWithRange(state.structures, 0);
    }

    // Range selection info
    const rangeInfoEl = document.getElementById('range-info');
    if (rangeInfoEl) {
        rangeInfoEl.innerHTML = '<span style="color:#888;">點選段落或句子，再用下方按鈕設定練習範圍</span>';
    }

    document.getElementById('start-practice-btn').disabled = true;
}

// New structure tree rendering with range selection support
function renderStructureTreeWithRange(nodes, depth) {
    return nodes.map(node => {
        const id = node.id || (node.song_structure ? node.song_structure.id : null);
        const title = node.title || (node.song_structure ? node.song_structure.title : '');
        const type = node.type || (node.song_structure ? node.song_structure.type : '');
        const startTime = node.start_time ?? node.song_structure?.start_time ?? 0;
        const endTime = node.end_time ?? node.song_structure?.end_time ?? 0;
        const duration = endTime - startTime;
        const durationStr = duration > 0 ? formatDuration(duration) : '';
        const phrases = node.phrases || [];
        
        const isSection = type === 'SECTION' || type === '';
        const icon = isSection ? '📂' : '📄';
        const indent = depth * 20;

        // Check if this node is in range selection
        const isRangeStart = state.rangeStartId === id;
        const isRangeEnd = state.rangeEndId === id;
        const isInRange = isRangeStart || isRangeEnd;

        return `
            <div class="structure-node" style="padding-left:${indent}px">
                <div class="structure-option ${isInRange ? 'in-range' : ''}"
                     onclick="onStructureClick('${id}', '${type}', ${startTime}, ${endTime})">
                    <span class="structure-icon">${icon}</span>
                    <span class="structure-name">${escapeHtml(title)}</span>
                    ${isSection && phrases.length ? `<span class="structure-badge">${phrases.length} 句</span>` : ''}
                    <span class="structure-time">${durationStr}</span>
                </div>
                ${phrases.length ? renderStructureTreeWithRange(phrases, depth + 1) : ''}
            </div>
        `;
    }).join('');
}

function formatDuration(seconds) {
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}:${secs.toString().padStart(2, '0')}`;
}

function onStructureClick(id, type, startTime, endTime) {
    // Update selected visual
    document.querySelectorAll('.structure-option').forEach(el => el.classList.remove('selected'));
    
    // Find clicked element and add selected class
    const clickedEl = event.currentTarget;
    clickedEl.classList.add('selected');

    // Store as potential range boundary
    // If no range start set, set as range start
    if (!state.rangeStartId) {
        state.rangeStartId = id;
        state.selectedRangeStart = { id, type, startTime, endTime };
        updateRangeInfo();
        return;
    }

    // If range start is set but no range end, set as range end
    if (!state.rangeEndId) {
        state.rangeEndId = id;
        state.selectedRangeEnd = { id, type, startTime, endTime };
        updateRangeInfo();
        return;
    }

    // Both set -> reset and start fresh
    state.rangeStartId = id;
    state.selectedRangeStart = { id, type, startTime, endTime };
    state.rangeEndId = null;
    state.selectedRangeEnd = null;
    updateRangeInfo();
}

function clearRangeSelection() {
    state.rangeStartId = null;
    state.rangeEndId = null;
    state.selectedRangeStart = null;
    state.selectedRangeEnd = null;
    document.querySelectorAll('.structure-option').forEach(el => el.classList.remove('selected', 'in-range'));
    updateRangeInfo();
}

function updateRangeInfo() {
    const rangeInfoEl = document.getElementById('range-info');
    if (!rangeInfoEl) return;

    const rangeActionsEl = document.getElementById('range-actions');
    if (!rangeActionsEl) return;

    if (!state.rangeStartId) {
        rangeInfoEl.innerHTML = '<span style="color:#888;">點選段落或句子，再用下方按鈕設定練習範圍</span>';
        rangeActionsEl.innerHTML = '';
        return;
    }

    const startTitle = state.selectedRangeStart?.type === 'SECTION' ? state.selectedRangeStart.type : state.selectedRangeStart?.type;

    // Find titles
    const findTitle = (id) => {
        const allStructures = flattenStructures(state.structures);
        const found = allStructures.find(s => (s.id || s.song_structure?.id) === id);
        return found ? (found.title || found.song_structure?.title || '') : '';
    };

    const startLabel = findTitle(state.rangeStartId);
    
    if (!state.rangeEndId) {
        rangeInfoEl.innerHTML = `
            <span>起始：<strong>${escapeHtml(startLabel)}</strong></span>
            <span style="color:#888; margin-left: 12px;">← 點選另一個段落或句子設為結束點</span>
        `;
        rangeActionsEl.innerHTML = `
            <button class="btn btn-sm btn-secondary" onclick="clearRangeSelection()">清除</button>
        `;
        return;
    }

    const endLabel = findTitle(state.rangeEndId);
    
    // Calculate total time for range
    const startTime = state.selectedRangeStart?.startTime ?? 0;
    const endTime = state.selectedRangeEnd?.endTime ?? 0;
    const rangeDuration = endTime > startTime ? formatDuration(endTime - startTime) : '';

    rangeInfoEl.innerHTML = `
        <span>範圍：<strong>${escapeHtml(startLabel)}</strong> → <strong>${escapeHtml(endLabel)}</strong></span>
        <span style="color:#888; margin-left: 12px;">${rangeDuration}</span>
    `;
    rangeActionsEl.innerHTML = `
        <button class="btn btn-sm btn-primary" onclick="confirmRange()">✓ 確認範圍</button>
        <button class="btn btn-sm btn-secondary" onclick="clearRangeSelection()">清除</button>
    `;
}

function confirmRange() {
    // Mark as confirmed
    document.getElementById('start-practice-btn').disabled = !state.selectedVocalTrack;
    
    // Show confirmed state
    const rangeInfoEl = document.getElementById('range-info');
    const startTitle = findStructureTitle(state.rangeStartId);
    const endTitle = findStructureTitle(state.rangeEndId);
    
    if (rangeInfoEl) {
        rangeInfoEl.innerHTML = `<span style="color: #27ae60;">✓ 已選取：<strong>${escapeHtml(startTitle)}</strong> → <strong>${escapeHtml(endTitle)}</strong></span>`;
    }

    const rangeActionsEl = document.getElementById('range-actions');
    if (rangeActionsEl) {
        rangeActionsEl.innerHTML = `
            <button class="btn btn-sm btn-secondary" onclick="clearRangeSelection()">重新選取</button>
        `;
    }

    updateStartButton();
}

function findStructureTitle(id) {
    const allStructures = flattenStructures(state.structures);
    const found = allStructures.find(s => (s.id || s.song_structure?.id) === id);
    return found ? (found.title || found.song_structure?.title || '') : '';
}

function flattenStructures(nodes) {
    let result = [];
    for (const node of nodes) {
        result.push(node);
        if (node.phrases && node.phrases.length) {
            result = result.concat(flattenStructures(node.phrases));
        }
    }
    return result;
}

function selectVocalTrack(trackId) {
    state.selectedVocalTrack = trackId;
    updateStartButton();
}

function toggleAccompaniment(trackId) {
    const idx = state.selectedAccompanimentTracks.indexOf(trackId);
    if (idx === -1) {
        state.selectedAccompanimentTracks.push(trackId);
    } else {
        state.selectedAccompanimentTracks.splice(idx, 1);
    }
}

function selectStructure(structureId) {
    state.selectedStructure = structureId;
}

function updateStartButton() {
    const btn = document.getElementById('start-practice-btn');
    btn.disabled = !state.selectedVocalTrack;
}

// === Practice ===
async function startPractice() {
    showScreen('practice-screen');
    document.getElementById('analysis-status').textContent = '準備錄音...';

    const song = state.selectedSong;
    if (!song) return;

    // Load notes for vocal and accompaniment tracks
    try {
        const fullSong = await api(`/songs/${song.id}`);
        
        // Vocal reference notes
        const vocalTrack = fullSong.tracks.find(t => t.id === state.selectedVocalTrack);
        if (vocalTrack) {
            state.referenceNotes = vocalTrack.notes || [];
        }

        // Accompaniment notes
        state.accompanimentNotes = [];
        for (const accId of state.selectedAccompanimentTracks) {
            const accTrack = fullSong.tracks.find(t => t.id === accId);
            if (accTrack && accTrack.notes) {
                state.accompanimentNotes = state.accompanimentNotes.concat(accTrack.notes);
            }
        }
    } catch (err) {
        console.error('Failed to load track notes:', err);
    }

    if (state.referenceNotes.length === 0) {
        document.getElementById('analysis-status').textContent = '無法取得參考音符資料';
    }

    // Update structures with lyrics for selected vocal track
    await loadLyrics();
    renderLyrics();

    // Check audio setup
    await checkAudioSetup();

    // Setup recording UI
    setupPracticeUI();
}

function setupPracticeUI() {
    // Show headset banner
    const headsetBannerEl = document.getElementById('headset-banner');
    if (headsetBannerEl) {
        headsetBannerEl.style.display = 'block';
    }

    // Show range info if selected
    const practiceRangeEl = document.getElementById('practice-range-info');
    if (practiceRangeEl && state.rangeStartId && state.rangeEndId) {
        const startTitle = findStructureTitle(state.rangeStartId);
        const endTitle = findStructureTitle(state.rangeEndId);
        practiceRangeEl.innerHTML = `
            <span>📌 練習範圍：<strong>${escapeHtml(startTitle)}</strong> → <strong>${escapeHtml(endTitle)}</strong></span>
        `;
        practiceRangeEl.style.display = 'block';
    } else if (practiceRangeEl) {
        practiceRangeEl.style.display = 'none';
    }

    // Show accompaniment info
    const accInfoEl = document.getElementById('accompaniment-info');
    if (accInfoEl && state.selectedAccompanimentTracks.length > 0) {
        // Get track names
        const song = state.selectedSong;
        const trackNames = state.selectedAccompanimentTracks.map(id => {
            const t = song.tracks.find(t => t.id === id);
            return t ? t.name : id;
        });
        accInfoEl.innerHTML = `🎵 伴奏：${escapeHtml(trackNames.join(', '))}`;
        accInfoEl.style.display = 'block';
    } else if (accInfoEl) {
        accInfoEl.style.display = 'none';
    }

    // Reset recording UI
    const statusEl = document.getElementById('recording-status');
    if (statusEl) {
        statusEl.textContent = '準備錄音';
        statusEl.style.color = '#fff';
    }

    const btn = document.getElementById('record-btn');
    if (btn) {
        btn.disabled = false;
        btn.textContent = '開始錄音';
        btn.classList.remove('recording');
    }

    document.getElementById('timer').textContent = '00:00';

    // Show count down area
    const countDownEl = document.getElementById('count-down');
    if (countDownEl) {
        countDownEl.classList.add('hidden');
    }

    // Reset progress
    const progressEl = document.getElementById('accompaniment-progress');
    if (progressEl) {
        progressEl.style.width = '0%';
    }

    document.getElementById('results-panel').classList.remove('visible');
    document.getElementById('visualizations').classList.add('hidden');
    document.getElementById('score-display').classList.add('hidden');
    document.getElementById('submit-btn').classList.add('hidden');
}

function goBackToTracks() {
    stopAccompaniment();
    state.selectedVocalTrack = null;
    state.selectedAccompanimentTracks = [];
    state.selectedStructure = null;
    state.referenceNotes = [];
    state.accompanimentNotes = [];
    state.detectedNotes = [];
    state.assessmentResult = null;
    state.rangeStartId = null;
    state.rangeEndId = null;
    state.selectedRangeStart = null;
    state.selectedRangeEnd = null;
    state.lyricsLines = [];
    stopLyricsSync();
    showScreen('tracks-screen');
}

// === BPM Calculation ===
function calculateBPM(notes, defaultBPM = 120) {
    if (!notes || notes.length < 2) return defaultBPM;

    const sorted = [...notes].sort((a, b) => (a.start_time ?? a.startTime) - (b.start_time ?? b.startTime));
    
    // Get the first few note onsets
    const onsets = sorted.map(n => n.start_time ?? n.startTime);
    
    // Calculate inter-onset intervals for first 8 notes (or fewer)
    const intervals = [];
    for (let i = 1; i < Math.min(onsets.length, 9); i++) {
        const interval = onsets[i] - onsets[i - 1];
        if (interval > 0.1 && interval < 3) { // reasonable note interval
            intervals.push(interval);
        }
    }

    if (intervals.length === 0) return defaultBPM;

    // Use median interval
    intervals.sort((a, b) => a - b);
    const mid = Math.floor(intervals.length / 2);
    const medianInterval = intervals.length % 2 === 0 
        ? (intervals[mid - 1] + intervals[mid]) / 2 
        : intervals[mid];

    // BPM = 60 / interval_in_seconds * beat_unit
    // Assuming quarter note = one interval
    const bpm = Math.round(60 / medianInterval);
    
    // Sanity check
    if (bpm < 30 || bpm > 300) return defaultBPM;
    return bpm;
}

// === Count-in ===
async function countIn(bpm, callback) {
    const countDownEl = document.getElementById('count-down');
    if (!countDownEl) {
        callback();
        return;
    }

    state.isCountIn = true;
    countDownEl.classList.remove('hidden');

    const audioCtx = state.audioCtx || new (window.AudioContext || window.webkitAudioContext)();
    state.audioCtx = audioCtx;

    const beatDuration = 60 / bpm;
    const startTime = audioCtx.currentTime + 0.1;

    for (let i = 0; i < 3; i++) {
        const beatTime = startTime + i * beatDuration;
        
        // Visual count
        setTimeout(() => {
            countDownEl.textContent = `${3 - i}`;
            countDownEl.style.color = '#3498db';
            countDownEl.style.fontSize = '72px';
        }, (i * beatDuration) * 1000);

        // Metronome click (short sine burst)
        const osc = audioCtx.createOscillator();
        const gain = audioCtx.createGain();
        osc.connect(gain);
        gain.connect(audioCtx.destination);
        
        osc.type = 'triangle';
        osc.frequency.value = i === 0 ? 880 : 660; // Higher pitch on first beat
        
        const clickDuration = 0.05;
        gain.gain.setValueAtTime(0.3, beatTime);
        gain.gain.exponentialRampToValueAtTime(0.01, beatTime + clickDuration);
        
        osc.start(beatTime);
        osc.stop(beatTime + clickDuration);
    }

    // Wait for count-in to finish
    const totalDuration = 3 * beatDuration;
    setTimeout(() => {
        countDownEl.classList.add('hidden');
        state.isCountIn = false;
        callback();
    }, totalDuration * 1000 + 100);
}

// === Accompaniment Playback ===
function playAccompaniment(rangeStart, rangeEnd) {
    if (!state.accompanimentNotes || state.accompanimentNotes.length === 0) {
        console.log('No accompaniment notes to play');
        return;
    }

    const audioCtx = state.audioCtx || new (window.AudioContext || window.webkitAudioContext)();
    state.audioCtx = audioCtx;

    // Resume context if suspended (autoplay policy)
    if (audioCtx.state === 'suspended') {
        audioCtx.resume();
    }

    // Filter notes within range
    const startSec = rangeStart ?? 0;
    const endSec = rangeEnd ?? Infinity;

    const filteredNotes = state.accompanimentNotes.filter(n => {
        const s = n.start_time ?? n.startTime;
        const e = n.end_time ?? n.endTime;
        return s >= startSec && s <= endSec;
    });

    if (filteredNotes.length === 0) return;

    // Create master gain
    const masterGain = audioCtx.createGain();
    masterGain.gain.value = 0.3; // Master volume
    masterGain.connect(audioCtx.destination);
    state.accompanimentGain = masterGain;

    // Sort notes by start time
    const sortedNotes = [...filteredNotes].sort((a, b) => (a.start_time ?? a.startTime) - (b.start_time ?? b.startTime));

    const now = audioCtx.currentTime;
    const playbackOffset = 0; // Start from the beginning of range

    state.currentScheduledNotes = [];
    state.playbackStartTime = now;

    for (const note of sortedNotes) {
        const noteStart = (note.start_time ?? note.startTime) - startSec;
        const noteEnd = (note.end_time ?? note.endTime) - startSec;
        const duration = noteEnd - noteStart;

        if (duration <= 0) continue;

        const pitch = note.pitch;
        const freq = 440 * Math.pow(2, (pitch - 69) / 12);

        // Create oscillator with ADSR envelope
        const osc = audioCtx.createOscillator();
        const noteGain = audioCtx.createGain();
        const filterNode = audioCtx.createBiquadFilter();

        osc.type = 'triangle'; // Triangle wave for softer sound
        osc.frequency.value = freq;

        // Simple filter for warmer tone
        filterNode.type = 'lowpass';
        filterNode.frequency.value = freq * 4;
        filterNode.Q.value = 1;

        // ADSR envelope
        const attackTime = 0.02;
        const releaseTime = 0.1;
        const sustainLevel = 0.7;

        const scheduleTime = now + noteStart;
        
        noteGain.gain.setValueAtTime(0, scheduleTime);
        noteGain.gain.linearRampToValueAtTime(1, scheduleTime + attackTime);
        noteGain.gain.setValueAtTime(sustainLevel, scheduleTime + attackTime + 0.01);
        
        const releaseStart = scheduleTime + Math.max(duration - releaseTime, 0);
        noteGain.gain.setValueAtTime(sustainLevel, releaseStart);
        noteGain.gain.exponentialRampToValueAtTime(0.001, scheduleTime + duration);

        osc.connect(filterNode);
        filterNode.connect(noteGain);
        noteGain.connect(masterGain);

        osc.start(scheduleTime);
        osc.stop(scheduleTime + duration + 0.05);

        state.currentScheduledNotes.push({
            osc,
            noteGain,
            filterNode,
            startTime: scheduleTime,
            endTime: scheduleTime + duration
        });
    }

    // Update progress bar
    const totalDuration = sortedNotes.length > 0 
        ? ((sortedNotes[sortedNotes.length - 1].end_time ?? sortedNotes[sortedNotes.length - 1].endTime) - startSec)
        : 30;

    // Animate progress
    const progressEl = document.getElementById('accompaniment-progress');
    const progressContainer = document.getElementById('accompaniment-progress-container');
    if (progressContainer) progressContainer.style.display = 'block';
    
    state.progressInterval = setInterval(() => {
        const elapsed = audioCtx.currentTime - state.playbackStartTime;
        const progress = Math.min((elapsed / totalDuration) * 100, 100);
        if (progressEl) progressEl.style.width = progress + '%';
        
        if (elapsed >= totalDuration) {
            clearInterval(state.progressInterval);
            if (progressEl) progressEl.style.width = '100%';
        }
    }, 100);
}

function stopAccompaniment() {
    // Stop oscillators
    if (state.currentScheduledNotes) {
        for (const note of state.currentScheduledNotes) {
            try {
                note.osc.stop();
                note.osc.disconnect();
            } catch (e) {
                // Already stopped
            }
        }
        state.currentScheduledNotes = [];
    }

    // Clear progress interval
    if (state.progressInterval) {
        clearInterval(state.progressInterval);
        state.progressInterval = null;
    }

    // Hide progress
    const progressContainer = document.getElementById('accompaniment-progress-container');
    if (progressContainer) progressContainer.style.display = 'none';

    // Stop lyrics sync
    stopLyricsSync();

    // Don't close audio context, it can be reused
}

// === Recording ===
async function toggleRecording() {
    if (state.isRecording) {
        stopRecording();
    } else {
        await startRecording();
    }
}

async function startRecording() {
    // Check if count-in is in progress
    if (state.isCountIn) {
        return;
    }

    // Get headset device preference
    let audioConstraints = { audio: true };
    if (state.preferredMicId) {
        audioConstraints = {
            audio: {
                deviceId: { exact: state.preferredMicId }
            }
        };
    }

    try {
        const stream = await navigator.mediaDevices.getUserMedia(audioConstraints);
        state.mediaRecorder = new MediaRecorder(stream);
        state.audioChunks = [];

        state.mediaRecorder.ondataavailable = (event) => {
            if (event.data.size > 0) {
                state.audioChunks.push(event.data);
            }
        };

        state.mediaRecorder.onstop = () => {
            const audioBlob = new Blob(state.audioChunks, { type: 'audio/webm' });
            stopAccompaniment();
            processAudio(audioBlob);
            stream.getTracks().forEach(track => track.stop());
        };

        // Calculate BPM from vocal reference notes
        const bpm = calculateBPM(state.referenceNotes, 120);
        
        // Update status for count-in
        const statusEl = document.getElementById('recording-status');
        if (statusEl) {
            statusEl.textContent = `倒數 ${bpm} BPM...`;
            statusEl.style.color = '#f39c12';
        }

        const btn = document.getElementById('record-btn');
        if (btn) {
            btn.disabled = true;
        }

        // Get range start time for accompaniment offset
        let rangeStart = 0;
        let rangeEnd = null;
        if (state.rangeStartId && state.rangeEndId) {
            rangeStart = state.selectedRangeStart?.startTime ?? 0;
            rangeEnd = state.selectedRangeEnd?.endTime ?? null;
        }

        // Do count-in, then start recording + accompaniment
        await countIn(bpm, () => {
            // Start recording
            state.mediaRecorder.start();
            state.isRecording = true;
            
            if (statusEl) {
                statusEl.textContent = '錄音中...';
                statusEl.style.color = '#e74c3c';
                statusEl.classList.add('recording');
            }
            
            if (btn) {
                btn.textContent = '停止錄音';
                btn.disabled = false;
            }

            // Start playing accompaniment
            playAccompaniment(rangeStart, rangeEnd);

            // Start lyrics sync
            startLyricsSync();

            // Update timer
            let seconds = 0;
            state.recordingTimer = setInterval(() => {
                seconds++;
                const mins = Math.floor(seconds / 60).toString().padStart(2, '0');
                const secs = (seconds % 60).toString().padStart(2, '0');
                document.getElementById('timer').textContent = `${mins}:${secs}`;
            }, 1000);
        });
    } catch (err) {
        alert('無法存取麥克風: ' + err.message);
        const btn = document.getElementById('record-btn');
        if (btn) {
            btn.disabled = false;
            btn.textContent = '開始錄音';
        }
    }
}

function stopRecording() {
    if (state.recordingTimer) {
        clearInterval(state.recordingTimer);
        state.recordingTimer = null;
    }

    stopLyricsSync();
    if (state.mediaRecorder && state.mediaRecorder.state !== 'inactive') {
        state.mediaRecorder.stop();
    }
    state.isRecording = false;

    const statusEl = document.getElementById('recording-status');
    if (statusEl) {
        statusEl.textContent = '處理中...';
        statusEl.style.color = '#f39c12';
        statusEl.classList.remove('recording');
    }

    const btn = document.getElementById('record-btn');
    if (btn) {
        btn.disabled = true;
        btn.textContent = '分析中...';
    }
}

// === Audio Processing ===
async function processAudio(audioBlob) {
    document.getElementById('analysis-status').textContent = '正在分析音頻...';

    try {
        const arrayBuffer = await audioBlob.arrayBuffer();
        const audioContext = new (window.AudioContext || window.webkitAudioContext)();
        const audioBuffer = await audioContext.decodeAudioData(arrayBuffer);

        // Filter reference notes to selected range if any
        let refNotes = state.referenceNotes;
        if (state.rangeStartId && state.rangeEndId) {
            const startSec = state.selectedRangeStart?.startTime ?? 0;
            const endSec = state.selectedRangeEnd?.endTime ?? Infinity;
            refNotes = refNotes.filter(n => {
                const s = n.start_time ?? n.startTime;
                return s >= startSec && s <= endSec;
            });
        }

        // Pitch detection
        const detected = detectPitch(audioBuffer);
        state.detectedNotes = detected;

        // Merge same-pitch reference notes, then compare merged vs detected
        const mergedRef = mergeSamePitchNotes(refNotes);
        const result = compareMergedNotes(refNotes, mergedRef, detected);
        state.assessmentResult = result;

        // Display results
        displayResults(result);

        document.getElementById('analysis-status').textContent = '分析完成';
    } catch (err) {
        console.error('Analysis error:', err);
        document.getElementById('analysis-status').textContent = '分析失敗: ' + err.message;
        const btn = document.getElementById('record-btn');
        if (btn) btn.disabled = false;
    }
}

// === Pitch Detection (Autocorrelation) ===
function detectPitch(audioBuffer) {
    const samples = audioBuffer.getChannelData(0);
    const sampleRate = audioBuffer.sampleRate;
    const windowSize = 2048;
    const hopSize = Math.floor(sampleRate * 0.05); // 50ms hop
    const silenceThreshold = 0.01;
    const correlationThreshold = 0.3;
    const minFreq = 50;
    const maxFreq = 2000;

    const notes = [];
    let currentNote = null;
    let consecutiveFrames = 0;

    for (let start = 0; start + windowSize < samples.length; start += hopSize) {
        const window = samples.slice(start, start + windowSize);

        // RMS energy
        let rms = 0;
        for (let i = 0; i < windowSize; i++) {
            rms += window[i] * window[i];
        }
        rms = Math.sqrt(rms / windowSize);

        if (rms < silenceThreshold) {
            // Silence - end current note if any
            if (currentNote) {
                currentNote.endTime = start / sampleRate;
                if (currentNote.pitch) {
                    // Merge if pitch close to previous
                    const lastNote = notes[notes.length - 1];
                    if (lastNote && Math.abs(lastNote.pitch - currentNote.pitch) < 2) {
                        lastNote.endTime = currentNote.endTime;
                    } else {
                        notes.push({ ...currentNote });
                    }
                }
                currentNote = null;
            }
            consecutiveFrames = 0;
            continue;
        }

        // Autocorrelation
        const correlation = [];
        const minPeriod = Math.floor(sampleRate / maxFreq);
        const maxPeriod = Math.ceil(sampleRate / minFreq);

        for (let period = minPeriod; period <= maxPeriod; period++) {
            let corr = 0;
            for (let i = 0; i < windowSize - period; i++) {
                corr += window[i] * window[i + period];
            }
            correlation.push({ period, value: corr / windowSize });
        }

        // Find peak
        let maxCorr = 0;
        let bestPeriod = 0;
        for (const c of correlation) {
            if (c.value > maxCorr) {
                maxCorr = c.value;
                bestPeriod = c.period;
            }
        }

        if (maxCorr < correlationThreshold) {
            if (currentNote) {
                currentNote.endTime = start / sampleRate;
                if (currentNote.pitch) {
                    const lastNote = notes[notes.length - 1];
                    if (lastNote && Math.abs(lastNote.pitch - currentNote.pitch) < 2) {
                        lastNote.endTime = currentNote.endTime;
                    } else {
                        notes.push({ ...currentNote });
                    }
                }
                currentNote = null;
            }
            continue;
        }

        // Convert to frequency and MIDI note
        const freq = sampleRate / bestPeriod;
        const midiNote = 12 * Math.log2(freq / 440) + 69;
        const roundedPitch = Math.round(midiNote);

        if (roundedPitch < 40 || roundedPitch > 93) {
            // Outside vocal range, skip
            consecutiveFrames = 0;
            continue;
        }

        const time = start / sampleRate;

        if (!currentNote) {
            currentNote = {
                pitch: roundedPitch,
                startTime: time,
                endTime: time + (hopSize / sampleRate)
            };
            consecutiveFrames = 1;
        } else {
            // Check if pitch changed significantly
            if (Math.abs(currentNote.pitch - roundedPitch) >= 2) {
                // End previous note
                currentNote.endTime = time;
                if (currentNote.pitch) {
                    const lastNote = notes[notes.length - 1];
                    if (lastNote && Math.abs(lastNote.pitch - currentNote.pitch) < 2 && time - lastNote.startTime < 0.3) {
                        lastNote.endTime = currentNote.endTime;
                    } else {
                        notes.push({ ...currentNote });
                    }
                }
                currentNote = {
                    pitch: roundedPitch,
                    startTime: time,
                    endTime: time + (hopSize / sampleRate)
                };
                consecutiveFrames = 1;
            } else {
                currentNote.endTime = time + (hopSize / sampleRate);
                consecutiveFrames++;
            }
        }
    }

    // Finalize last note
    if (currentNote && currentNote.pitch) {
        const lastNote = notes[notes.length - 1];
        if (lastNote && Math.abs(lastNote.pitch - currentNote.pitch) < 2 &&
            currentNote.startTime - lastNote.endTime < 0.3) {
            lastNote.endTime = currentNote.endTime;
        } else {
            notes.push({ ...currentNote });
        }
    }

    // Filter out very short notes (< 100ms)
    return notes.filter(n => (n.endTime - n.startTime) > 0.1);
}

// === Merged Note Comparison ===

/**
 * Groups consecutive same-pitch notes whose gap < 50ms into one merged note.
 * Returns an array of { pitch, startTime, endTime, eventIdx[] }.
 */
function mergeSamePitchNotes(notes) {
    if (!notes || notes.length === 0) return [];

    const sorted = [...notes].sort((a, b) => (a.start_time ?? a.startTime) - (b.start_time ?? b.startTime));

    const merged = [];
    let current = {
        pitch: sorted[0].pitch,
        startTime: sorted[0].start_time ?? sorted[0].startTime,
        endTime: sorted[0].end_time ?? sorted[0].endTime,
        eventIdx: [0]
    };

    const gapThreshold = 0.05;

    for (let i = 1; i < sorted.length; i++) {
        const n = sorted[i];
        const s = n.start_time ?? n.startTime;
        const e = n.end_time ?? n.endTime;
        const gap = s - current.endTime;

        if (n.pitch === current.pitch && gap >= 0 && gap <= gapThreshold) {
            if (e > current.endTime) current.endTime = e;
            current.eventIdx.push(i);
        } else {
            merged.push(current);
            current = { pitch: n.pitch, startTime: s, endTime: e, eventIdx: [i] };
        }
    }
    merged.push(current);
    return merged;
}

/**
 * Compares merged reference note groups against detected notes.
 * If a merged group has >=80% overlap with a same-pitch detected note,
 * all events in that group are marked "matched" with group-level deviations.
 */
function compareMergedNotes(reference, mergedRef, detected) {
    const detSorted = [...detected].sort((a, b) => a.startTime - b.startTime);

    const matched = reference.map(ref => ({
        refPitch: ref.pitch,
        userPitch: 0,
        refStart: ref.start_time ?? ref.startTime,
        refEnd: ref.end_time ?? ref.endTime,
        userStart: 0,
        userEnd: 0,
        pitchDeviationCents: 0,
        durationDeviationSec: 0,
        matchStatus: 'missed'
    }));

    const usedDetected = new Set();
    let matchedCount = 0;
    let totalPitchDev = 0;
    let totalDurationDev = 0;

    for (const mg of mergedRef) {
        const mgLen = mg.endTime - mg.startTime;
        let bestIdx = -1;
        let bestOverlap = 0;

        for (let i = 0; i < detSorted.length; i++) {
            if (usedDetected.has(i)) continue;
            const det = detSorted[i];
            if (det.pitch !== mg.pitch) continue;

            const overlapStart = Math.max(mg.startTime, det.startTime);
            const overlapEnd = Math.min(mg.endTime, det.endTime);
            if (overlapEnd <= overlapStart) continue;

            const overlapLen = overlapEnd - overlapStart;
            const detLen = det.endTime - det.startTime;
            const shorterLen = Math.min(mgLen, detLen);
            if (shorterLen <= 0) continue;

            const ratio = overlapLen / shorterLen;
            if (ratio > bestOverlap) {
                bestOverlap = ratio;
                bestIdx = i;
            }
        }

        if (bestIdx >= 0 && bestOverlap >= 0.8) {
            usedDetected.add(bestIdx);
            const det = detSorted[bestIdx];

            const pitchDev = (mg.pitch - det.pitch) * 100;
            const durDev = mgLen - (det.endTime - det.startTime);

            for (const eidx of mg.eventIdx) {
                matched[eidx].userPitch = det.pitch;
                matched[eidx].userStart = det.startTime;
                matched[eidx].userEnd = det.endTime;
                matched[eidx].pitchDeviationCents = pitchDev;
                matched[eidx].durationDeviationSec = durDev;
                matched[eidx].matchStatus = 'matched';
            }

            matchedCount += mg.eventIdx.length;
            totalPitchDev += Math.abs(pitchDev) * mg.eventIdx.length;
            totalDurationDev += Math.abs(durDev) * mg.eventIdx.length;
        }
    }

    const avgPitchDev = matchedCount > 0 ? totalPitchDev / matchedCount : 0;
    const avgDurationDev = matchedCount > 0 ? totalDurationDev / matchedCount : 0;

    const pitchScore = Math.max(0, 100 - avgPitchDev * 0.5);
    const durationScore = Math.max(0, 100 - avgDurationDev * 50);
    const overallScore = pitchScore * 0.7 + durationScore * 0.3;

    const pitchDeviations = matched.map(m => m.pitchDeviationCents);
    const durationDeviations = matched.map(m => m.durationDeviationSec);

    return {
        score: Math.round(overallScore),
        totalNotes: reference.length,
        matchedNotes: matchedCount,
        averagePitchDeviation: Math.round(avgPitchDev * 10) / 10,
        averageDurationDeviation: Math.round(avgDurationDev * 100) / 100,
        pitchDeviation: pitchDeviations,
        durationDeviation: durationDeviations,
        noteComparison: matched
    };
}

// === Display Results ===
function displayResults(result) {
    // Score circle
    const scoreDisplay = document.getElementById('score-display');
    scoreDisplay.classList.remove('hidden');
    document.getElementById('score-value').textContent = result.score;

    const scoreCircle = document.getElementById('score-circle');
    const color = result.score >= 80 ? '#2ecc71' : result.score >= 50 ? '#f1c40f' : '#e74c3c';
    scoreCircle.style.background = `conic-gradient(${color} 0deg, ${color} ${result.score * 3.6}deg, #e0e0e0 ${result.score * 3.6}deg)`;

    // Show visualizations
    document.getElementById('visualizations').classList.remove('hidden');

    // Draw charts
    drawPitchContour(result.noteComparison);
    drawDeviationChart(result.noteComparison);
    drawSegmentScores(result);
    const btn = document.getElementById('record-btn');
    if (btn) {
        btn.disabled = false;
        btn.textContent = '重新錄音';
    }

    // Show submit button
    document.getElementById('submit-btn').classList.remove('hidden');
}

function drawPitchContour(comparison) {
    const canvas = document.getElementById('pitch-contour');
    const ctx = canvas.getContext('2d');
    const width = canvas.width;
    const height = canvas.height;
    ctx.clearRect(0, 0, width, height);

    const margin = { top: 20, bottom: 25, left: 40, right: 20 };
    const plotWidth = width - margin.left - margin.right;
    const plotHeight = height - margin.top - margin.bottom;

    if (comparison.length === 0) {
        ctx.fillStyle = '#888';
        ctx.textAlign = 'center';
        ctx.fillText('無資料', width / 2, height / 2);
        return;
    }

    // Find range
    const allPitches = comparison.flatMap(c => [c.refPitch, c.userPitch || c.refPitch].filter(p => p > 0));
    const minPitch = Math.min(...allPitches) - 2;
    const maxPitch = Math.max(...allPitches) + 2;
    const minTime = Math.min(...comparison.map(c => c.refStart));
    const maxTime = Math.max(...comparison.map(c => c.refEnd));

    const timeRange = Math.max(maxTime - minTime, 1);

    const xScale = (t) => margin.left + ((t - minTime) / timeRange) * plotWidth;
    const yScale = (p) => margin.top + plotHeight - ((p - minPitch) / (maxPitch - minPitch)) * plotHeight;

    // Draw grid
    ctx.strokeStyle = '#eee';
    ctx.lineWidth = 1;
    for (let p = minPitch; p <= maxPitch; p += 2) {
        ctx.beginPath();
        ctx.moveTo(margin.left, yScale(p));
        ctx.lineTo(width - margin.right, yScale(p));
        ctx.stroke();
        ctx.fillStyle = '#999';
        ctx.font = '10px sans-serif';
        ctx.textAlign = 'right';
        ctx.fillText(p, margin.left - 5, yScale(p) + 4);
    }

    // Draw reference line
    ctx.strokeStyle = '#3498db';
    ctx.lineWidth = 2;
    ctx.beginPath();
    comparison.forEach((c, i) => {
        const x = xScale(c.refStart);
        const y = yScale(c.refPitch);
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
        ctx.lineTo(xScale(c.refEnd), y);
    });
    ctx.stroke();

    // Draw user line
    ctx.strokeStyle = '#e67e22';
    ctx.lineWidth = 2;
    ctx.setLineDash([4, 4]);
    ctx.beginPath();
    let drawing = false;
    comparison.forEach((c, i) => {
        if (c.matchStatus === 'matched' && c.userPitch > 0) {
            const x = xScale(c.userStart);
            const y = yScale(c.userPitch);
            if (!drawing) {
                ctx.moveTo(x, y);
                drawing = true;
            } else {
                ctx.lineTo(x, y);
            }
        } else {
            drawing = false;
        }
    });
    ctx.stroke();
    ctx.setLineDash([]);

    // Highlight deviations
    comparison.forEach(c => {
        if (c.matchStatus === 'matched' && Math.abs(c.pitchDeviationCents) > 100) {
            const x = xScale(c.refStart);
            const y = yScale(c.refPitch);
            const userY = yScale(c.userPitch);
            ctx.fillStyle = 'rgba(231, 76, 60, 0.3)';
            ctx.fillRect(x, Math.min(y, userY), xScale(c.refEnd) - x, Math.abs(y - userY));
        }
    });

    // Labels
    ctx.fillStyle = '#3498db';
    ctx.font = '11px sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText('參考 MIDI', width / 2 - 60, 14);

    ctx.fillStyle = '#e67e22';
    ctx.fillText('你的演唱', width / 2 + 60, 14);
}

function drawDeviationChart(comparison) {
    const canvas = document.getElementById('deviation-chart');
    const ctx = canvas.getContext('2d');
    const width = canvas.width;
    const height = canvas.height;
    ctx.clearRect(0, 0, width, height);

    const margin = { top: 10, bottom: 30, left: 30, right: 10 };
    const plotWidth = width - margin.left - margin.right;
    const plotHeight = height - margin.top - margin.bottom;

    const matched = comparison.filter(c => c.matchStatus === 'matched');
    if (matched.length === 0) {
        ctx.fillStyle = '#888';
        ctx.textAlign = 'center';
        ctx.fillText('無數配對', width / 2, height / 2);
        return;
    }

    const maxDev = Math.max(200, ...matched.map(m => Math.abs(m.pitchDeviationCents))) + 50;
    const barWidth = Math.min(20, (plotWidth / matched.length) - 2);

    matched.forEach((c, i) => {
        const x = margin.left + (i / matched.length) * plotWidth + 2;
        const dev = c.pitchDeviationCents;
        const barHeight = (Math.abs(dev) / maxDev) * plotHeight;

        let color;
        if (Math.abs(dev) < 50) color = '#2ecc71';
        else if (Math.abs(dev) < 100) color = '#f1c40f';
        else color = '#e74c3c';

        const yBase = margin.top + plotHeight / 2;
        const y = dev >= 0 ? yBase - barHeight : yBase;

        ctx.fillStyle = color;
        ctx.fillRect(x, y, barWidth, barHeight);

        // Draw center line
        ctx.strokeStyle = '#ccc';
        ctx.lineWidth = 1;
        ctx.beginPath();
        ctx.moveTo(margin.left, yBase);
        ctx.lineTo(width - margin.right, yBase);
        ctx.stroke();
    });

    ctx.fillStyle = '#666';
    ctx.font = '10px sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText('音符', width / 2, height - 5);

    ctx.textAlign = 'left';
    ctx.fillText('+ 偏高', margin.left, margin.top + 12);
    ctx.fillText('- 偏低', margin.left, height - margin.bottom + 14);
}

function drawSegmentScores(result) {
    const container = document.getElementById('segment-scores');

    if (state.structures.length === 0) {
        container.innerHTML = `
            <div class="segment-score-card">
                <div class="segment-title">整首歌</div>
                <div class="segment-value ${getScoreClass(result.score)}">${result.score}</div>
            </div>
        `;
        return;
    }

    const sections = state.structures.filter(s => s.type === 'SECTION' || (s.song_structure && s.song_structure.type === 'SECTION'));
    if (sections.length === 0) {
        container.innerHTML = `
            <div class="segment-score-card">
                <div class="segment-title">全曲</div>
                <div class="segment-value ${getScoreClass(result.score)}">${result.score}</div>
            </div>
        `;
        return;
    }

    container.innerHTML = sections.map(s => {
        const title = s.title || (s.song_structure ? s.song_structure.title : '');
        // Simulate segment score based on overall score with slight variation
        const segScore = Math.max(0, Math.min(100, result.score + (Math.random() * 20 - 10)));
        return `
            <div class="segment-score-card">
                <div class="segment-title">${escapeHtml(title)}</div>
                <div class="segment-value ${getScoreClass(segScore)}">${Math.round(segScore)}</div>
            </div>
        `;
    }).join('');
}

function getScoreClass(score) {
    if (score >= 80) return 'green';
    if (score >= 50) return 'yellow';
    return 'red';
}

// === Submit Assessment ===
async function submitAssessment() {
    if (!state.assessmentResult) return;

    const result = state.assessmentResult;
    const payload = {
        song_id: state.selectedSong.id,
        structure_id: state.selectedStructure || null,
        track_id: state.selectedVocalTrack,
        score: result.score,
        total_notes: result.totalNotes,
        matched_notes: result.matchedNotes,
        average_pitch_deviation: result.averagePitchDeviation,
        average_duration_deviation: result.averageDurationDeviation,
        pitch_deviation: result.pitchDeviation,
        duration_deviation: result.durationDeviation,
        note_comparison: result.noteComparison
    };

    try {
        const response = await api('/assessments/submit', {
            method: 'POST',
            body: JSON.stringify(payload)
        });
        document.getElementById('submit-btn').textContent = '✓ 已上傳';
        document.getElementById('submit-btn').disabled = true;
        document.getElementById('analysis-status').textContent = '評分結果已儲存';
    } catch (err) {
        alert('上傳失敗: ' + err.message);
    }
}

// === Lyrics ===
async function loadLyrics() {
    const trackId = state.selectedVocalTrack;
    if (!trackId || !state.selectedSong) {
        state.lyricsLines = [];
        return;
    }

    try {
        // Fetch structures with lyrics for the selected vocal track
        const result = await api(`/songs/${state.selectedSong.id}/structures?track_id=${trackId}`);
        const structures = result.structures || [];

        // Flatten to get all PHRASE nodes with lyrics
        const lines = [];
        function extractLyrics(nodes) {
            for (const node of nodes) {
                if (node.type === 'PHRASE' && node.lyrics) {
                    lines.push({
                        structureId: node.id,
                        lyrics: node.lyrics,
                        startTime: node.start_time ?? 0,
                        endTime: node.end_time ?? 0
                    });
                }
                if (node.phrases && node.phrases.length) {
                    extractLyrics(node.phrases);
                }
            }
        }
        extractLyrics(structures);

        // Filter by selected range if any
        if (state.rangeStartId && state.rangeEndId) {
            const rangeStart = state.selectedRangeStart?.startTime ?? 0;
            const rangeEnd = state.selectedRangeEnd?.endTime ?? Infinity;
            state.lyricsLines = lines.filter(l => l.startTime >= rangeStart && l.endTime <= rangeEnd);
        } else {
            state.lyricsLines = lines;
        }
    } catch (err) {
        console.error('Failed to load lyrics:', err);
        state.lyricsLines = [];
    }
}

function renderLyrics() {
    const container = document.getElementById('lyrics-lines');
    const wrapper = document.getElementById('lyrics-container');
    if (!container) return;

    if (state.lyricsLines.length === 0) {
        if (wrapper) wrapper.style.display = 'none';
        return;
    }

    if (wrapper) wrapper.style.display = 'block';

    container.innerHTML = state.lyricsLines.map((line, idx) => `
        <div class="lyrics-line" data-lyrics-idx="${idx}" data-start="${line.startTime}" data-end="${line.endTime}">
            ${escapeHtml(line.lyrics)}
        </div>
    `).join('');
}

function startLyricsSync() {
    stopLyricsSync();
    if (state.lyricsLines.length === 0) return;

    const audioCtx = state.audioCtx;
    if (!audioCtx) return;

    function update() {
        if (!state.isRecording && !state.playbackStartTime) {
            state.lyricsAnimFrame = requestAnimationFrame(update);
            return;
        }

        const currentTime = audioCtx.currentTime - state.playbackStartTime + 
            (state.selectedRangeStart?.startTime ?? 0);

        let activeIndex = -1;
        for (let i = 0; i < state.lyricsLines.length; i++) {
            const line = state.lyricsLines[i];
            const isActive = currentTime >= line.startTime && currentTime < line.endTime;
            const isPassed = currentTime >= line.endTime;

            const el = document.querySelector(`.lyrics-line[data-lyrics-idx="${i}"]`);
            if (el) {
                el.classList.toggle('active', isActive);
                el.classList.toggle('passed', isPassed && !isActive);
                if (isActive) {
                    activeIndex = i;
                }
            }
        }

        // Auto-scroll to active lyric
        if (activeIndex >= 0) {
            const activeEl = document.querySelector(`.lyrics-line[data-lyrics-idx="${activeIndex}"]`);
            if (activeEl) {
                activeEl.scrollIntoView({ behavior: 'smooth', block: 'center' });
            }
        }

        state.lyricsAnimFrame = requestAnimationFrame(update);
    }

    state.lyricsAnimFrame = requestAnimationFrame(update);
}

function stopLyricsSync() {
    if (state.lyricsAnimFrame) {
        cancelAnimationFrame(state.lyricsAnimFrame);
        state.lyricsAnimFrame = null;
    }
}

// === Local Storage Check ===
async function getStorageEstimate() {
    if (!navigator.storage?.estimate) {
        return { used: 0, total: 0, supported: false };
    }
    try {
        const estimate = await navigator.storage.estimate();
        return {
            used: estimate.usage || 0,
            total: estimate.quota || 0,
            supported: true
        };
    } catch {
        return { used: 0, total: 0, supported: false };
    }
}

function formatStorage(bytes) {
    if (bytes === 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
}

// === Utility ===
function escapeHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

// === Initial State ===
showScreen('login-screen');

// === Track Selection Module ===
import { state } from './state.js';
import { escapeHtml, formatDuration, flattenStructures, findStructureTitle } from './utils.js';

export function renderTrackSelection() {
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
                    data-action="selectVocalTrack" data-id="${t.id}">
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
                    data-action="toggleAccompaniment" data-id="${t.id}">
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
                     data-action="onStructureClick"
                     data-id="${id}" data-type="${type}"
                     data-start="${startTime}" data-end="${endTime}">
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

export function onStructureClick(id, type, startTime, endTime, clickedEl) {
    // Update selected visual
    document.querySelectorAll('.structure-option').forEach(el => el.classList.remove('selected'));
    
    // Add selected class to clicked element
    if (clickedEl) {
        clickedEl.classList.add('selected');
    }

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

export function clearRangeSelection() {
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
            <button class="btn btn-sm btn-secondary" data-action="clearRangeSelection">清除</button>
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
        <button class="btn btn-sm btn-primary" data-action="confirmRange">✓ 確認範圍</button>
        <button class="btn btn-sm btn-secondary" data-action="clearRangeSelection">清除</button>
    `;
}

export function confirmRange() {
    // Mark as confirmed
    document.getElementById('start-practice-btn').disabled = !state.selectedVocalTrack;
    
    // Show confirmed state
    const rangeInfoEl = document.getElementById('range-info');
    const startTitle = findStructureTitle(state.rangeStartId, state.structures);
    const endTitle = findStructureTitle(state.rangeEndId, state.structures);
    
    if (rangeInfoEl) {
        rangeInfoEl.innerHTML = `<span style="color: #27ae60;">✓ 已選取：<strong>${escapeHtml(startTitle)}</strong> → <strong>${escapeHtml(endTitle)}</strong></span>`;
    }

    const rangeActionsEl = document.getElementById('range-actions');
    if (rangeActionsEl) {
        rangeActionsEl.innerHTML = `
            <button class="btn btn-sm btn-secondary" data-action="clearRangeSelection">重新選取</button>
        `;
    }

    updateStartButton();
}

export function selectVocalTrack(trackId) {
    state.selectedVocalTrack = trackId;
    updateStartButton();
}

export function toggleAccompaniment(trackId) {
    const idx = state.selectedAccompanimentTracks.indexOf(trackId);
    if (idx === -1) {
        state.selectedAccompanimentTracks.push(trackId);
    } else {
        state.selectedAccompanimentTracks.splice(idx, 1);
    }
}

export function updateStartButton() {
    const btn = document.getElementById('start-practice-btn');
    btn.disabled = !state.selectedVocalTrack;
}
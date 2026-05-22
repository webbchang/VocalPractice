// practice-ui.js — 純 UI 渲染
// 依賴：state.js, audio.js
// 僅做 DOM 操作，不含業務邏輯

import state from './state.js';
import { playRange, togglePlayback, getIsPlaying, stopPlayback, setUpdatePlayBtn } from './audio.js';

function formatTime(sec) {
    if (sec == null) return '0:00';
    const m = Math.floor(sec / 60);
    const s = Math.floor(sec % 60);
    return m + ':' + s.toString().padStart(2, '0');
}

// --- 播放按鈕 ---
export function updatePlayBtn() {
    const btn = document.getElementById('record-btn');
    if (!btn) return;
    if (getIsPlaying()) {
        btn.innerHTML = `<svg viewBox="0 0 24 24" width="30" height="30">
            <rect x="6" y="6" width="5" height="12" fill="white"/>
            <rect x="13" y="6" width="5" height="12" fill="white"/>
        </svg>`;
    } else {
        btn.innerHTML = `<svg viewBox="0 0 24 24" width="30" height="30">
            <polygon points="8,5 19,12 8,19" fill="white"/>
        </svg>`;
    }
}

// Register updatePlayBtn with audio module
setUpdatePlayBtn(updatePlayBtn);

// --- 音軌渲染 ---
export function renderVocalTracks(tracks) {
    const container = document.getElementById('vocal-tracks');
    if (!tracks || tracks.length === 0) {
        container.innerHTML = '<div class="no-structures">此歌曲沒有音軌</div>';
        return;
    }

    let html = '';
    let hasVocal = false;

    for (const t of tracks) {
        if (!t.is_vocal) continue;
        hasVocal = true;
        const isSelected = t.id === state.selectedTrackId;
        const badgeText = t.is_vocal ? '🎤 主聲部' : '🎵 伴唱聲部';
        html += `
            <div class="track-option ${isSelected ? 'selected' : ''}" onclick="window.__selectTrack('${t.id}')">
                <input type="radio" name="vocal-track" ${isSelected ? 'checked' : ''}>
                <span class="track-name">${t.name}</span>
                <span class="track-badge has-structure">${badgeText}</span>
            </div>
        `;
    }
    if (!hasVocal) {
        container.innerHTML = '<div class="no-structures">此歌曲沒有主聲部音軌</div>';
        return;
    }
    container.innerHTML = html;
}

export function renderAccompanimentTracks(tracks) {
    const container = document.getElementById('accompaniment-tracks');
    if (!tracks || tracks.length === 0) {
        container.innerHTML = '<div class="no-structures">此歌曲沒有音軌</div>';
        return;
    }

    let html = '';
    for (const t of tracks) {
        const isChecked = state.selectedAccompanimentTrackIds.includes(t.id);
        const badgeText = t.is_vocal ? '🎤 主聲部' : '🎵 伴唱聲部';
        const isDisabled = t.id === state.selectedTrackId;
        html += `
            <div class="track-option ${isChecked ? 'selected' : ''}" onclick="${isDisabled ? '' : `window.__toggleAccompanimentTrack('${t.id}')`}" style="${isDisabled ? 'opacity:0.5;cursor:not-allowed;' : ''}">
                <input type="checkbox" ${isChecked ? 'checked' : ''} ${isDisabled ? 'disabled' : ''} style="margin-right:12px;">
                <span class="track-name">${t.name}</span>
                <span class="track-badge has-structure">${badgeText}</span>
            </div>
        `;
    }
    container.innerHTML = html;
}

// --- 段落結構渲染 ---
export function renderStructureList() {
    const container = document.getElementById('structure-list');
    const { structures, selectedStructure } = state;
    if (!structures || structures.length === 0) {
        container.innerHTML = '<div class="no-structures">此音軌尚無段落結構<br><small>請管理員在後台設定段落</small></div>';
        return;
    }

    let html = '';
    for (const s of structures) {
        const isSelected = selectedStructure && selectedStructure.id === s.id;
        html += `
            <div class="structure-option ${isSelected ? 'selected' : ''}" onclick="window.__selectStructure('${s.id}')">
                <span class="structure-icon">📂</span>
                <span class="structure-name">${s.title}</span>
                <span class="structure-time">${formatTime(s.start_time)} - ${formatTime(s.end_time)}</span>
                <button class="btn-play" onclick="event.stopPropagation();playRange(${s.start_time},${s.end_time})">▶️</button>
            </div>
        `;
        if (s.phrases) {
            for (const p of s.phrases) {
                const isPhraseSelected = selectedStructure && selectedStructure.id === p.id;
                html += `
                    <div class="structure-option phrased ${isPhraseSelected ? 'selected' : ''}" onclick="window.__selectStructure('${p.id}')">
                        <span class="structure-icon">📄</span>
                        <span class="structure-name">${p.title} ${p.lyrics ? '🎤' : ''}</span>
                        <span class="structure-time">${formatTime(p.start_time)} - ${p.lyrics ? '' : formatTime(p.end_time)}</span>
                        <button class="btn-play" onclick="event.stopPropagation();playRange(${p.start_time},${p.end_time})">▶️</button>
                    </div>
                `;
            }
        }
    }
    container.innerHTML = html;
}

// --- 選擇 UI 管理 ---
export function updateSelectionUI() {
    if (state.selectedStructure) {
        document.getElementById('range-info').textContent =
            `🎯 ${state.selectedStructure.title}：${formatTime(state.selectedStructure.start)} - ${formatTime(state.selectedStructure.end)}`;
        document.getElementById('range-actions').style.display = 'flex';
        document.getElementById('current-range-label').textContent =
            `${state.selectedStructure.title} (${formatTime(state.selectedStructure.start)})`;
        renderStructureList();
    }
}

export function clearSelection() {
    state.selectedStructure = null;
    document.getElementById('range-info').textContent = '選擇一個段落或句子開始練習';
    document.getElementById('range-actions').style.display = 'none';
    document.getElementById('current-range-label').textContent = '未選擇';
    renderStructureList();
}

// --- 雜項 UI ---
export function switchTab(el, tab) {
    document.querySelectorAll('.viz-tab').forEach(t => t.classList.remove('active'));
    el.classList.add('active');
    // Placeholder - would switch visualization types
}

export function logout() {
    document.cookie = 'token=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;';
    window.location.href = '/index.html';
}
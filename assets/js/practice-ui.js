// practice-ui.js — 純 UI 渲染
// 依賴：state.js, audio.js
// 僅做 DOM 操作，不含業務邏輯

import state from './state.js';
import { playRange, togglePlayback, getIsPlaying, stopPlayback, setUpdatePlayBtn } from './audio.js';
import { playRangeWithBeats, interruptPractice } from './practice-business.js';
import { getLyricsForSelection } from './practice-business.js';

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
    
    // 同時更新段落/句子按鈕的中止按鈕顯示狀態
    updateStopButtonsVisibility();
}

// 更新所有段落/句子中止按鈕的顯示/隱藏
export function updateStopButtonsVisibility() {
    const stopButtons = document.querySelectorAll('.btn-stop');
    const isPlaying = getIsPlaying();
    
    stopButtons.forEach(btn => {
        btn.style.display = isPlaying ? 'inline-block' : 'none';
    });
}

// Register updatePlayBtn with audio module
setUpdatePlayBtn(() => {
    updatePlayBtn();
    updateStopButtonsVisibility();
});

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
        html += `
            <div class="track-option ${isChecked ? 'selected' : ''}" onclick="window.__toggleAccompanimentTrack('${t.id}')">
                <input type="checkbox" ${isChecked ? 'checked' : ''} style="margin-right:12px;">
                <span class="track-name">${t.name}</span>
                <span class="track-badge has-structure">${badgeText}</span>
            </div>
        `;
    }
    container.innerHTML = html;
}

/**
 * 檢查 flatItems 中的某個 index 是否在選取範圍內
 */
function isItemSelected(idx) {
    const { from, to } = state.selectionRange;
    if (from === null || to === null) return false;
    const minIdx = Math.min(from, to);
    const maxIdx = Math.max(from, to);
    return idx >= minIdx && idx <= maxIdx;
}

/**
 * 檢查 section 是否因為其 phrases 全被選取而間接選取
 */
function isSectionImplicitlySelected(sectionId) {
    const { flatItems, selectionRange } = state;
    const { from, to } = selectionRange;
    if (from === null || to === null) return false;

    // 找出這個 section 在 flatItems 中的範圍
    let sectionIdx = -1;
    let phraseStartIdx = -1;
    let phraseEndIdx = -1;
    for (let i = 0; i < flatItems.length; i++) {
        const item = flatItems[i];
        if (item.type === 'section' && item.data.id === sectionId) {
            sectionIdx = i;
            phraseStartIdx = i + 1;
            continue;
        }
        // 找到下一個 section 或結尾
        if (item.type === 'section' && sectionIdx >= 0 && phraseStartIdx >= 0) {
            phraseEndIdx = i - 1;
            break;
        }
    }
    if (phraseStartIdx < 0) return false;
    if (phraseEndIdx < 0) phraseEndIdx = flatItems.length - 1;
    if (phraseStartIdx > phraseEndIdx) return false; // no phrases

    // 檢查是否所有 phrases 都在選取範圍內
    const minIdx = Math.min(from, to);
    const maxIdx = Math.max(from, to);
    for (let i = phraseStartIdx; i <= phraseEndIdx; i++) {
        if (flatItems[i].type !== 'phrase') continue;
        if (i < minIdx || i > maxIdx) return false;
    }
    return true;
}

// --- 段落結構渲染（支援混合多選段落+句子）---
export function renderStructureList() {
    const container = document.getElementById('structure-list');
    const { flatItems, selectionRange } = state;
    if (!flatItems || flatItems.length === 0) {
        // fallback: try using structures directly
        const { structures } = state;
        if (!structures || structures.length === 0) {
            container.innerHTML = '<div class="no-structures">此音軌尚無段落結構<br><small>請管理員在後台設定段落</small></div>';
            return;
        }
    }

    // 如果沒有 flatItems 但 structures 有資料，嘗試建立
    let items = flatItems;
    if (items.length === 0 && state.structures.length > 0) {
        // call buildFlatItems from business — but we can't import it, so build inline
        for (const s of state.structures) {
            items.push({ type: 'section', data: s, section: s });
            if (s.phrases) {
                for (const p of s.phrases) {
                    items.push({ type: 'phrase', data: p, section: s });
                }
            }
        }
    }

    // 如果還是沒有，顯示提示
    if (items.length === 0) {
        container.innerHTML = '<div class="no-structures">此音軌尚無段落結構<br><small>請管理員在後台設定段落</small></div>';
        return;
    }

    let html = '';
    for (let i = 0; i < items.length; i++) {
        const item = items[i];
        const isSelected = isItemSelected(i);

        if (item.type === 'section') {
            // 檢查是否因為 phrases 全選而間接選取
            const isImplicit = !isSelected && isSectionImplicitlySelected(item.data.id);
            html += `
                <div class="structure-option ${isSelected || isImplicit ? 'selected' : ''}" onclick="window.__selectStructure('${item.data.id}')">
                    <span class="structure-icon">📂</span>
                    <span class="structure-name">${item.data.title}</span>
                    <span class="structure-time">${formatTime(item.data.start_time)} - ${formatTime(item.data.end_time)}</span>
                    <button class="btn-play" onclick="event.stopPropagation();playRangeWithBeats(${item.data.start_time},${item.data.end_time},false)">▶️</button>
                    <button class="btn-stop" onclick="event.stopPropagation();window.__stopPlayback()" style="display:none;margin-left:4px;">⏹️</button>
                </div>
            `;
        } else if (item.type === 'phrase') {
            html += `
                <div class="structure-option phrased ${isSelected ? 'selected' : ''}" onclick="window.__selectStructure('${item.data.id}')">
                    <span class="structure-icon">📄</span>
                    <span class="structure-name">${item.data.title} ${item.data.lyrics ? '🎤' : ''}</span>
                    <span class="structure-time">${formatTime(item.data.start_time)} - ${item.data.lyrics ? '' : formatTime(item.data.end_time)}</span>
                    <button class="btn-play" onclick="event.stopPropagation();playRangeWithBeats(${item.data.start_time},${item.data.end_time},false)">▶️</button>
                    <button class="btn-stop" onclick="event.stopPropagation();window.__stopPlayback()" style="display:none;margin-left:4px;">⏹️</button>
                </div>
            `;
        }
    }
    container.innerHTML = html;
}

// --- 歌詞面板渲染 ---
export function renderLyricsPanel() {
    const container = document.getElementById('lyrics-content');
    const statusEl = document.getElementById('lyrics-status');
    const { selectedStructure, selectedPhraseIds } = state;

    if (!selectedStructure && selectedPhraseIds.length === 0) {
        container.innerHTML = '<div class="lyrics-placeholder">選擇段落或句子以顯示歌詞</div>';
        if (statusEl) statusEl.textContent = '未選取';
        return;
    }

    const lyricsData = getLyricsForSelection();
    if (!lyricsData || lyricsData.phrases.length === 0) {
        container.innerHTML = '<div class="lyrics-placeholder">所選範圍無歌詞資料</div>';
        if (statusEl) statusEl.textContent = '無歌詞';
        return;
    }

    if (statusEl) statusEl.textContent = `${lyricsData.count} 句`;

    let html = '';
    for (const item of lyricsData.phrases) {
        html += `<div class="lyrics-phrase-label">📄 ${item.title}</div>`;
        if (item.lyrics) {
            html += `<div class="lyrics-text">${escapeHtml(item.lyrics)}</div>`;
        } else {
            html += `<div class="lyrics-no-lyrics">（此句無歌詞）</div>`;
        }
    }
    container.innerHTML = html;
}

function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

// --- 選擇 UI 管理（已由 business 的 updateSelectionUI 取代）---
export function updateSelectionUI() {
    if (state.selectedStructure) {
        const formatTime = (sec) => {
            if (sec == null) return '0:00';
            const m = Math.floor(sec / 60);
            const s = Math.floor(sec % 60);
            return m + ':' + s.toString().padStart(2, '0');
        };
        document.getElementById('range-info').textContent =
            `🎯 ${state.selectedStructure.title}：${formatTime(state.selectedStructure.start)} - ${formatTime(state.selectedStructure.end)}`;
        document.getElementById('range-actions').style.display = 'flex';
        document.getElementById('current-range-label').textContent =
            `${state.selectedStructure.title} (${formatTime(state.selectedStructure.start)})`;
        renderStructureList();
        renderLyricsPanel();
    }
}

export function clearSelection() {
    state.selectedStructure = null;
    state.selectedPhraseIds = [];
    state.selectedPhrasesData = [];
    state.selectionRange = { from: null, to: null };
    document.getElementById('range-info').textContent = '選擇一個段落或句子開始練習';
    document.getElementById('range-actions').style.display = 'none';
    document.getElementById('current-range-label').textContent = '未選擇';
    renderStructureList();
    renderLyricsPanel();
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
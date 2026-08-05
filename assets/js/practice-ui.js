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
    // 隱藏結果面板
    const resultsPanel = document.getElementById('results-panel');
    if (resultsPanel) {
        resultsPanel.classList.remove('visible');
    }
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

// === 視覺化函式 ===

/**
 * 繪製分數儀表（圓形進度條）
 * @param {number} score - 分數 (0-100)
 * @param {HTMLElement} container - 容器元素
 */
export function drawScoreGauge(score, container) {
    // 清空容器
    container.innerHTML = '';
    
    // 創建 SVG 元素
    const size = 120;
    const svgNS = "http://www.w3.org/2000/svg";
    const svg = document.createElementNS(svgNS, "svg");
    svg.setAttribute("width", size);
    svg.setAttribute("height", size);
    svg.setAttribute("viewBox", `0 0 ${size} ${size}`);
    
    // 背景圓環
    const backgroundCircle = document.createElementNS(svgNS, "circle");
    backgroundCircle.setAttribute("cx", size/2);
    backgroundCircle.setAttribute("cy", size/2);
    backgroundCircle.setAttribute("r", size/2 - 10);
    backgroundCircle.setAttribute("fill", "none");
    backgroundCircle.setAttribute("stroke", "#333");
    backgroundCircle.setAttribute("stroke-width", 8);
    svg.appendChild(backgroundCircle);
    
    // 前景圓環 (根據分數)
    const foregroundCircle = document.createElementNS(svgNS, "circle");
    foregroundCircle.setAttribute("cx", size/2);
    foregroundCircle.setAttribute("cy", size/2);
    foregroundCircle.setAttribute("r", size/2 - 10);
    foregroundCircle.setAttribute("fill", "none");
    foregroundCircle.setAttribute("stroke", getScoreColor(score));
    foregroundCircle.setAttribute("stroke-width", 8);
    foregroundCircle.setAttribute("stroke-dasharray", `${score * (Math.PI * (size/2 - 10)) / 50}, 1000`);
    foregroundCircle.setAttribute("stroke-dashoffset", `${(100 - score) * (Math.PI * (size/2 - 10)) / 50}`);
    foregroundCircle.setAttribute("transform", `rotate(-90 ${size/2} ${size/2})`);
    svg.appendChild(foregroundCircle);
    
    // 中心文字
    const text = document.createElementNS(svgNS, "text");
    text.setAttribute("x", size/2);
    text.setAttribute("y", size/2 + 5);
    text.setAttribute("text-anchor", "middle");
    text.setAttribute("fill", "#fff");
    text.setAttribute("font-size", "24");
    text.setAttribute("font-weight", "bold");
    text.textContent = Math.round(score);
    svg.appendChild(text);
    
    // 分數標籤
    const label = document.createElementNS(svgNS, "text");
    label.setAttribute("x", size/2);
    label.setAttribute("y", size/2 + 28);
    label.setAttribute("text-anchor", "middle");
    label.setAttribute("fill", "#888");
    label.setAttribute("font-size", "14");
    label.textContent = "分數";
    svg.appendChild(label);
    
    container.appendChild(svg);
}

/**
 * 根據分數取得顏色
 * @param {number} score - 分數 (0-100)
 * @returns {string} - 顏色代碼
 */
function getScoreColor(score) {
    if (score >= 90) return "#4CAF50";    // 綠色 - 優秀
    if (score >= 80) return "#8BC34A";    // 淺綠 - 良好
    if (score >= 70) return "#FFC107";    // 黃色 - 普通
    if (score >= 60) return "#FF9800";    // 橙色 - 及格
    return "#F44336";                     // 紅色 - 不及格
}

/**
 * 繪製音高偏差圖表
 * @param {Array} noteComparisons - 音符比較陣列
 * @param {HTMLElement} container - 容器元素
 */
export function drawPitchDeviationChart(noteComparisons, container) {
    // 清空容器
    container.innerHTML = '';
    
    if (!noteComparisons || noteComparisons.length === 0) {
        container.textContent = '無音符資料';
        return;
    }
    
    // 創建 SVG 元素
    const width = container.clientWidth || 300;
    const height = 150;
    const padding = 20;
    const svgNS = "http://www.w3.org/2000/svg";
    const svg = document.createElementNS(svgNS, "svg");
    svg.setAttribute("width", width);
    svg.setAttribute("height", height);
    svg.setAttribute("style", "display: block; margin: 0 auto;");
    
    // 背景
    const rect = document.createElementNS(svgNS, "rect");
    rect.setAttribute("width", width);
    rect.setAttribute("height", height);
    rect.setAttribute("fill", "#1a1a2e");
    svg.appendChild(rect);
    
    // X 軸 (時間)
    const maxTime = Math.max(...noteComparisons.map(n => n.ref_end ?? n.RefEnd ?? 0));
    const xScale = (width - 2 * padding) / Math.max(maxTime, 0.1);
    
    // Y 軸 (音分偏差)
    const deviations = noteComparisons.map(n => n.pitch_deviation_cents ?? n.PitchDeviationCents ?? 0);
    const maxDeviation = Math.max(...deviations.map(Math.abs), 50); // 至少顯示 ±50 cent
    const yScale = (height - 2 * padding) / (2 * maxDeviation);
    const centerY = height / 2;
    
    // X 軸線
    const xAxis = document.createElementNS(svgNS, "line");
    xAxis.setAttribute("x1", padding);
    xAxis.setAttribute("y1", centerY);
    xAxis.setAttribute("x2", width - padding);
    xAxis.setAttribute("y2", centerY);
    xAxis.setAttribute("stroke", "#555");
    xAxis.setAttribute("stroke-width", 1);
    svg.appendChild(xAxis);
    
    // Y 軸線
    const yAxis = document.createElementNS(svgNS, "line");
    yAxis.setAttribute("x1", padding);
    yAxis.setAttribute("y1", padding);
    yAxis.setAttribute("x2", padding);
    yAxis.setAttribute("y2", height - padding);
    yAxis.setAttribute("stroke", "#555");
    yAxis.setAttribute("stroke-width", 1);
    svg.appendChild(yAxis);
    
    // 零線
    const zeroLine = document.createElementNS(svgNS, "line");
    zeroLine.setAttribute("x1", padding);
    zeroLine.setAttribute("y1", centerY);
    zeroLine.setAttribute("x2", width - padding);
    zeroLine.setAttribute("y2", centerY);
    zeroLine.setAttribute("stroke", "#333");
    zeroLine.setAttribute("stroke-width", 1);
    zeroLine.setAttribute("stroke-dasharray", "2,2");
    svg.appendChild(zeroLine);
    
    // 數據點和連線
    const path = document.createElementNS(svgNS, "path");
    let pathData = `M ${padding} ${centerY - (deviations[0] || 0) * yScale}`;
    
    for (let i = 0; i < noteComparisons.length; i++) {
        const note = noteComparisons[i];
        const x = padding + (note.ref_start ?? note.RefStart ?? 0) * xScale;
        const y = centerY - ((note.pitch_deviation_cents ?? note.PitchDeviationCents ?? 0) * yScale);
        
        if (i === 0) {
            pathData += `M ${x} ${y}`;
        } else {
            pathData += `L ${x} ${y}`;
        }
        
        // 資料點
        const circle = document.createElementNS(svgNS, "circle");
        circle.setAttribute("cx", x);
        circle.setAttribute("cy", y);
        circle.setAttribute("r", 4);
        circle.setAttribute("fill", Math.abs(note.pitch_deviation_cents ?? note.PitchDeviationCents ?? 0) < 20 ? "#4CAF50" : "#F44336");
        circle.setAttribute("stroke", "#fff");
        circle.setAttribute("stroke-width", 1);
        svg.appendChild(circle);
    }
    
    path.setAttribute("d", pathData);
    path.setAttribute("fill", "none");
    path.setAttribute("stroke", "#1E90FF");
    path.setAttribute("stroke-width", 2);
    svg.appendChild(path);
    
    // 標籤
    const xLabel = document.createElementNS(svgNS, "text");
    xLabel.setAttribute("x", width / 2);
    xLabel.setAttribute("y", height - 5);
    xLabel.setAttribute("text-anchor", "middle");
    xLabel.setAttribute("fill", "#888");
    xLabel.setAttribute("font-size", "12");
    xLabel.textContent = "時間 (秒)";
    svg.appendChild(xLabel);
    
    const yLabel = document.createElementNS(svgNS, "text");
    yLabel.setAttribute("x", 8);
    yLabel.setAttribute("y", 20);
    yLabel.setAttribute("text-anchor", "middle");
    yLabel.setAttribute("fill", "#888");
    yLabel.setAttribute("font-size", "12");
    yLabel.setAttribute("transform", "rotate(-90, 8, 20)");
    yLabel.textContent = "音分偏差";
    svg.appendChild(yLabel);
    
    container.appendChild(svg);
}

/**
 * 繪製音符比對表格
 * @param {Array} noteComparisons - 音符比較陣列
 * @param {HTMLElement} container - 容器元素
 */
export function drawNoteComparisonTable(noteComparisons, container) {
    // 清空容器
    container.innerHTML = '';
    
    if (!noteComparisons || noteComparisons.length === 0) {
        const noData = document.createElement('div');
        noData.textContent = '無音符比對資料';
        noData.style.textAlign = 'center';
        noData.style.padding = '20px';
        noData.style.color = '#888';
        container.appendChild(noData);
        return;
    }
    
    // 創建表格
    const table = document.createElement('table');
    table.style.width = '100%';
    table.style.borderCollapse = 'collapse';
    table.style.margin = '10px 0';
    
    // 標題列
    const thead = document.createElement('thead');
    const headerRow = document.createElement('tr');
    
    const headers = ['音符', '開始時間', '結束時間', '狀態', '音高偏差 (cent)', '時長偏差 (秒)'];
    headers.forEach(headerText => {
        const th = document.createElement('th');
        th.textContent = headerText;
        th.style.border = '1px solid #444';
        th.style.backgroundColor = '#1a1a2e';
        th.style.color = '#eee';
        th.style.padding = '8px';
        th.style.textAlign = 'left';
        th.style.fontSize = '13px';
        headerRow.appendChild(th);
    });
    
    thead.appendChild(headerRow);
    table.appendChild(thead);
    
    // 身體
    const tbody = document.createElement('tbody');
    
    noteComparisons.forEach((note, index) => {
        const tr = document.createElement('tr');
        
        // 音符名稱
        const noteNameTd = document.createElement('td');
        const refPitch = note.ref_pitch ?? note.RefPitch ?? 0;
        noteNameTd.textContent = getNoteName(refPitch);
        noteNameTd.style.border = '1px solid #444';
        noteNameTd.style.padding = '8px';
        noteNameTd.style.color = '#ddd';
        noteNameTd.style.fontSize = '13px';
        tr.appendChild(noteNameTd);
        
        // 開始時間
        const startTd = document.createElement('td');
        const refStart = note.ref_start ?? note.RefStart ?? 0;
        startTd.textContent = refStart.toFixed(2);
        startTd.style.border = '1px solid #444';
        startTd.style.padding = '8px';
        startTd.style.color = '#ddd';
        startTd.style.fontSize = '13px';
        tr.appendChild(startTd);
        
        // 結束時間
        const endTd = document.createElement('td');
        const refEnd = note.ref_end ?? note.RefEnd ?? 0;
        endTd.textContent = refEnd.toFixed(2);
        endTd.style.border = '1px solid #444';
        endTd.style.padding = '8px';
        endTd.style.color = '#ddd';
        endTd.style.fontSize = '13px';
        tr.appendChild(endTd);
        
        // 狀態
        const statusTd = document.createElement('td');
        const matchStatus = note.match_status ?? note.MatchStatus ?? 'unknown';
        const matchStatusLower = typeof matchStatus === 'string' ? matchStatus.toLowerCase() : 'unknown';
        statusTd.textContent = matchStatus;
        statusTd.style.border = '1px solid #444';
        statusTd.style.padding = '8px';
        statusTd.style.textAlign = 'center';
        statusTd.style.fontSize = '13px';
        if (matchStatusLower === 'matched') {
            statusTd.style.color = '#4CAF50';
            statusTd.style.fontWeight = 'bold';
        } else {
            statusTd.style.color = '#F44336';
            statusTd.style.fontWeight = 'bold';
        }
        tr.appendChild(statusTd);
        
        // 音高偏差
        const pitchTd = document.createElement('td');
        const pitchDev = note.pitch_deviation_cents ?? note.PitchDeviationCents ?? 0;
        pitchTd.textContent = pitchDev.toFixed(1);
        pitchTd.style.border = '1px solid #444';
        pitchTd.style.padding = '8px';
        pitchTd.style.color = Math.abs(pitchDev) < 20 ? '#4CAF50' : '#F44336';
        pitchTd.style.fontSize = '13px';
        tr.appendChild(pitchTd);
        
        // 時長偏差
        const durationTd = document.createElement('td');
        const durationDev = note.duration_deviation_sec ?? note.DurationDeviationSec ?? 0;
        durationTd.textContent = durationDev.toFixed(3);
        durationTd.style.border = '1px solid #444';
        durationTd.style.padding = '8px';
        durationTd.style.color = Math.abs(durationDev) < 0.1 ? '#4CAF50' : '#F44336';
        durationTd.style.fontSize = '13px';
        tr.appendChild(durationTd);
        
        tbody.appendChild(tr);
    });
    
    table.appendChild(tbody);
    container.appendChild(table);
}

/**
 * 將 MIDI 音高號碼轉換為音符名稱
 * @param {number} midiNote - MIDI 音高號碼 (0-127)
 * @returns {string} - 音符名稱 (如 C4, D#4 等)
 */
function getNoteName(midiNote) {
    const noteNames = ['C', 'C#', 'D', 'D#', 'E', 'F', 'F#', 'G', 'G#', 'A', 'A#', 'B'];
    const octave = Math.floor(midiNote / 12) - 1;
    const noteIndex = midiNote % 12;
    return `${noteNames[noteIndex]}${octave}`;
}

/**
 * 繪製聲音類型分佈（母音/子音）
 * @param {Object} analysisResult - 分析結果物件
 * @param {HTMLElement} container - 容器元素
 */
export function drawVoicingDistribution(analysisResult, container) {
    // 清空容器
    container.innerHTML = '';
    
    // 這裡需要額外的資料來顯示母音/子音分佈
    // 由於目前的評估結果中沒有這個資訊，我們顯示一個佔位符
    const placeholder = document.createElement('div');
    placeholder.textContent = '聲音類型分析 (需要額外的聲音處理)';
    placeholder.style.textAlign = 'center';
    placeholder.style.padding = '20px';
    placeholder.style.color = '#888';
    placeholder.style.fontStyle = 'italic';
    container.appendChild(placeholder);
}
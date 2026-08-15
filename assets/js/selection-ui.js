// selection-ui.js — 段落/句子選取清單的渲染（selection UI）
// 只負責把 selection.js 計算出來的選取狀態畫出來。
import state from './state.js';
import { renderLyricsPanel } from './practice-ui.js';

function formatTime(sec) {
  if (sec == null) return '0:00';
  const m = Math.floor(sec / 60);
  const s = Math.floor(sec % 60);
  return m + ':' + s.toString().padStart(2, '0');
}

// 判斷某 phrase 是否作用中（所屬整段被選取，或本身被個別選取）
function isPhraseActive(section, phrase) {
  return state.selectedSectionIds.includes(section.id) ||
         state.selectedPhraseIds.includes(phrase.id);
}

// 渲染「段落與句子」清單：section 為可收合群組，phrase 巢狀縮排
export function renderStructureList() {
  const container = document.getElementById('structure-list');
  const { structures } = state;

  if (!structures || structures.length === 0) {
    container.innerHTML = '<div class="no-structures">此音軌尚無段落結構<br><small>請管理員在後台設定段落</small></div>';
    return;
  }

  let html = '';
  for (const section of structures) {
    const sectionSelected = state.selectedSectionIds.includes(section.id);
    const phrases = section.phrases || [];
    html += '<div class="structure-group">' +
        '<div class="structure-option section ' + (sectionSelected ? 'selected' : '') + '" onclick="window.selectSection(\'' + section.id + '\')">' +
          '<span class="structure-icon">📂</span>' +
          '<span class="structure-name">' + section.title + '</span>' +
          '<span class="structure-time">' + formatTime(section.start_time) + ' - ' + formatTime(section.end_time) + '</span>' +
          '<button class="btn-play" onclick="event.stopPropagation();window.playRangeWithBeats(' + section.start_time + ',' + section.end_time + ',false)">▶️</button>' +
          '<button class="btn-stop" onclick="event.stopPropagation();window.__stopPlayback()" style="display:none;margin-left:4px;">⏹️</button>' +
        '</div>';
    if (phrases.length > 0) {
      html += '<div class="structure-children">';
      for (const phrase of phrases) {
        const active = isPhraseActive(section, phrase);
        html += '<div class="structure-option phrase ' + (active ? 'selected' : '') + '" onclick="window.selectPhrase(\'' + phrase.id + '\')">' +
            '<span class="structure-icon">📄</span>' +
            '<span class="structure-name">' + phrase.title + (phrase.lyrics ? ' 🎤' : '') + '</span>' +
            '<span class="structure-time">' + formatTime(phrase.start_time) + ' - ' + formatTime(phrase.end_time) + '</span>' +
            '<button class="btn-play" onclick="event.stopPropagation();window.playRangeWithBeats(' + phrase.start_time + ',' + phrase.end_time + ',false)">▶️</button>' +
            '<button class="btn-stop" onclick="event.stopPropagation();window.__stopPlayback()" style="display:none;margin-left:4px;">⏹️</button>' +
          '</div>';
      }
      html += '</div>';
    }
    html += '</div>';
  }
  container.innerHTML = html;
}

// 選取後刷新相關 UI：範圍資訊、動作按鈕、清單與歌詞
export function refreshSelectionUI() {
  const { selectedStructure } = state;
  const rangeInfo = document.getElementById('range-info');
  const rangeActions = document.getElementById('range-actions');
  const currentRangeLabel = document.getElementById('current-range-label');

  if (selectedStructure) {
    rangeInfo.textContent = '🎯 ' + selectedStructure.title + '：' + formatTime(selectedStructure.start) + ' - ' + formatTime(selectedStructure.end);
    rangeActions.style.display = 'flex';
    currentRangeLabel.textContent = selectedStructure.title + ' (' + formatTime(selectedStructure.start) + ')';
  } else {
    rangeInfo.textContent = '選擇一個段落或句子開始練習';
    rangeActions.style.display = 'none';
    currentRangeLabel.textContent = '未選擇';
  }

  renderStructureList();
  renderLyricsPanel();
}

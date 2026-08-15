// selection.js — 段落/句子選取邏輯（selection business logic）
// 與 UI 解耦：本模組只處理「選了什麼」的狀態與衍生資料，
// 選取後的畫面刷新交由 selection-ui.js 的 refreshSelectionUI()。
import state from './state.js';
import { refreshSelectionUI } from './selection-ui.js';

// 找出某 phrase 所屬的 section
function findParentSection(phraseId) {
  for (const s of state.structures) {
    if ((s.phrases || []).some(p => p.id === phraseId)) return s;
  }
  return null;
}

// 收集所有「作用中」的 phrase（整段選取 + 個別選取）
function activePhrases() {
  const ids = new Set(state.selectedPhraseIds);
  for (const s of state.structures) {
    if (state.selectedSectionIds.includes(s.id)) {
      (s.phrases || []).forEach(p => ids.add(p.id));
    }
  }
  const result = [];
  for (const s of state.structures) {
    (s.phrases || []).forEach(p => { if (ids.has(p.id)) result.push(p); });
  }
  return result;
}

// 由目前選取狀態重新計算 selectedStructure / selectedPhrasesData / selectedPhraseIds
function recomputeSelection() {
  const active = activePhrases();

  // 整段選取但該段無句子的 section，仍要貢獻時間範圍
  const sectionSpans = state.structures
    .filter(s => state.selectedSectionIds.includes(s.id))
    .map(s => ({ start: s.start_time, end: s.end_time }));

  if (active.length === 0 && sectionSpans.length === 0) {
    state.selectedStructure = null;
    state.selectedPhrasesData = [];
    state.selectedPhraseIds = [];
    return;
  }

  const starts = [...active.map(p => p.start_time), ...sectionSpans.map(s => s.start)];
  const ends = [...active.map(p => p.end_time), ...sectionSpans.map(s => s.end)];
  const minStart = Math.min(...starts);
  const maxEnd = Math.max(...ends);

  let title;
  if (active.length === 1 && sectionSpans.length === 0) {
    title = active[0].title;
  } else if (sectionSpans.length > 0) {
    const names = state.selectedSectionIds
      .map(sid => (state.structures.find(s => s.id === sid) || {}).title)
      .filter(Boolean);
    title = names.join(' + ') + '（' + active.length + ' 句）';
  } else {
    const titles = active.map(p => p.title);
    title = titles[0] + ' … ' + titles[titles.length - 1] + '（' + active.length + ' 句）';
  }

  state.selectedStructure = { id: 'sel-' + Date.now(), title, start: minStart, end: maxEnd };
  state.selectedPhrasesData = active;
  state.selectedPhraseIds = active.map(p => p.id);
}

// 產生參考音符（原在 practice-business.js，與選取緊密相關故一併移入）
export function generateReferenceData() {
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
}

// 整段（section）選取切換：選取整段即包含其下所有句子
export function selectSection(id) {
  const section = state.structures.find(s => s.id === id);
  if (!section) return;
  if (state.selectedSectionIds.includes(id)) {
    state.selectedSectionIds = state.selectedSectionIds.filter(x => x !== id);
    state.selectedPhraseIds = state.selectedPhraseIds.filter(pid =>
      !(section.phrases || []).some(p => p.id === pid));
  } else {
    state.selectedSectionIds = [...state.selectedSectionIds, id];
    (section.phrases || []).forEach(p => {
      if (!state.selectedPhraseIds.includes(p.id)) state.selectedPhraseIds.push(p.id);
    });
  }
  recomputeSelection();
  generateReferenceData();
  refreshSelectionUI();
}

// 單句（phrase）選取切換：獨立切換；若某段所有句子皆選取則視為整段選取
export function selectPhrase(id) {
  const section = findParentSection(id);
  if (!section) return;
  if (state.selectedPhraseIds.includes(id)) {
    state.selectedPhraseIds = state.selectedPhraseIds.filter(x => x !== id);
    state.selectedSectionIds = state.selectedSectionIds.filter(x => x !== section.id);
  } else {
    state.selectedPhraseIds = [...state.selectedPhraseIds, id];
    if ((section.phrases || []).every(p => state.selectedPhraseIds.includes(p.id))) {
      state.selectedSectionIds = [...state.selectedSectionIds, section.id];
    }
  }
  recomputeSelection();
  generateReferenceData();
  refreshSelectionUI();
}

// 清除選取狀態（不含結果/音高面板的隱藏，由各 UI 模組處理）
export function resetSelection() {
  state.selectedSectionIds = [];
  state.selectedPhraseIds = [];
  state.selectedPhrasesData = [];
  state.selectedStructure = null;
}

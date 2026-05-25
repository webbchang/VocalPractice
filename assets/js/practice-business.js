// practice-business.js — 純業務邏輯
// 依賴：state.js, api.js, audio.js, songDataExtractor.js (global)
// 不直接操作 DOM（UI 操作委託給 practice-ui.js）

import state from './state.js';
import { api, loadMIDI, loadStructures } from './api.js';
import { playRange, stopPlayback } from './audio.js';

function determineParsedTrackIndex() {
    const { currentSongFull, selectedTrackId } = state;
    if (!currentSongFull || !currentSongFull.tracks) return -1;
    for (const t of currentSongFull.tracks) {
        if (t.id === selectedTrackId) {
            return t.midi_index;
        }
    }
    return -1;
}

function generateReferenceData() {
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
    console.log(`Reference data: ${state.referenceNotes.length} notes from track ${parsedTrackIndex} [${start.toFixed(2)}-${end.toFixed(2)}]`);
}

/**
 * 收集當前選取範圍的歌詞資料
 * 回傳：{ phrases: [{ title, lyrics }], count: number }
 */
export function getLyricsForSelection() {
    const { structures, selectedStructure, selectedPhraseIds, selectedPhrasesData } = state;
    const result = { phrases: [], count: 0 };

    // 如果有多選句子，用 selectedPhrasesData
    if (selectedPhraseIds.length > 0 && selectedPhrasesData.length > 0) {
        for (const p of selectedPhrasesData) {
            result.phrases.push({
                title: p.title,
                lyrics: p.lyrics || ''
            });
        }
        result.count = result.phrases.length;
        return result;
    }

    // 如果有選取的 structure（section），顯示該 section 下所有 phrases 的歌詞
    if (selectedStructure) {
        for (const s of structures) {
            if (s.id === selectedStructure.id) {
                const phrases = s.phrases || [];
                if (phrases.length > 0) {
                    for (const p of phrases) {
                        result.phrases.push({
                            title: p.title,
                            lyrics: p.lyrics || ''
                        });
                    }
                } else {
                    // section 本身沒有 phrases，顯示 section 標題
                    result.phrases.push({
                        title: s.title,
                        lyrics: s.lyrics || ''
                    });
                }
                result.count = result.phrases.length;
                return result;
            }
        }
    }

    return result;
}

export async function selectTrack(trackId) {
    state.selectedTrackId = trackId;
    state.referenceNotes = null;
    state.parsedTrackIndex = determineParsedTrackIndex();
    state.selectedPhraseIds = [];
    state.selectedPhrasesData = [];
    // Re-render both vocal and accompaniment track lists
    const { renderVocalTracks, renderAccompanimentTracks } = await import('./practice-ui.js');
    if (state.currentSongFull && state.currentSongFull.tracks) {
        renderVocalTracks(state.currentSongFull.tracks);
        renderAccompanimentTracks(state.currentSongFull.tracks);
    }
    const structures = await loadStructures(state.currentSongId, trackId);
    state.structures = structures;
    const { renderStructureList, renderLyricsPanel } = await import('./practice-ui.js');
    renderStructureList();
    renderLyricsPanel();
}

export function selectStructure(id) {
    const { structures } = state;
    for (const s of structures) {
        if (s.id === id) {
            // 點選 section → 清除多選，選取整個 section
            state.selectedPhraseIds = [];
            state.selectedPhrasesData = [];
            state.selectedStructure = { id: s.id, title: s.title, start: s.start_time, end: s.end_time };
            generateReferenceData();
            updateSelectionUI();
            return;
        }
        if (s.phrases) {
            for (const p of s.phrases) {
                if (p.id === id) {
                    // 點選 phrase → toggle 多選
                    togglePhraseSelection(p, s);
                    return;
                }
            }
        }
    }
}

/**
 * 切換句子的選取狀態（連續多選）
 */
function togglePhraseSelection(clickedPhrase, parentSection) {
    const { selectedPhraseIds, selectedPhrasesData, structures } = state;
    const allPhrases = [];
    for (const s of structures) {
        if (s.phrases) {
            for (const p of s.phrases) {
                allPhrases.push({ phrase: p, section: s });
            }
        }
    }

    // 找出被點擊句子的索引
    const phraseIndices = allPhrases.map(item => item.phrase.id);
    const clickedIdx = phraseIndices.indexOf(clickedPhrase.id);
    if (clickedIdx === -1) return;

    // 計算選取範圍
    let newIds;
    if (selectedPhraseIds.includes(clickedPhrase.id)) {
        // 如果已選取且是第一個或最後一個，縮減
        if (selectedPhraseIds.length === 1) {
            // 只有一個時清除選取 → 改為選取 parent section
            state.selectedPhraseIds = [];
            state.selectedPhrasesData = [];
            if (parentSection) {
                state.selectedStructure = { id: parentSection.id, title: parentSection.title, start: parentSection.start_time, end: parentSection.end_time };
            }
            generateReferenceData();
            updateSelectionUI();
            return;
        }
        // 移除末尾或開頭
        const firstIdx = phraseIndices.indexOf(selectedPhraseIds[0]);
        const lastIdx = phraseIndices.indexOf(selectedPhraseIds[selectedPhraseIds.length - 1]);
        if (clickedIdx === firstIdx) {
            newIds = selectedPhraseIds.slice(1);
        } else if (clickedIdx === lastIdx) {
            newIds = selectedPhraseIds.slice(0, -1);
        } else {
            // 點選中間的 → 不做變化或重選
            newIds = [clickedPhrase.id];
        }
    } else {
        // 新選取：判斷連續性
        if (selectedPhraseIds.length === 0) {
            newIds = [clickedPhrase.id];
        } else {
            const firstSelectedIdx = phraseIndices.indexOf(selectedPhraseIds[0]);
            const lastSelectedIdx = phraseIndices.indexOf(selectedPhraseIds[selectedPhraseIds.length - 1]);
            const minIdx = Math.min(firstSelectedIdx, lastSelectedIdx);
            const maxIdx = Math.max(firstSelectedIdx, lastSelectedIdx);

            if (clickedIdx < minIdx) {
                // 往前擴展
                newIds = phraseIndices.slice(clickedIdx, maxIdx + 1);
            } else if (clickedIdx > maxIdx) {
                // 往後擴展
                newIds = phraseIndices.slice(minIdx, clickedIdx + 1);
            } else {
                // 在範圍內 → 單選此句
                newIds = [clickedPhrase.id];
            }
        }
    }

    // 更新 state
    state.selectedPhraseIds = newIds;
    state.selectedPhrasesData = newIds.map(id => {
        for (const item of allPhrases) {
            if (item.phrase.id === id) return item.phrase;
        }
        return null;
    }).filter(Boolean);

    // 計算合併範圍
    if (newIds.length > 0) {
        const firstPhrase = allPhrases.find(item => item.phrase.id === newIds[0]);
        const lastPhrase = allPhrases.find(item => item.phrase.id === newIds[newIds.length - 1]);
        if (firstPhrase && lastPhrase) {
            const firstStart = firstPhrase.phrase.start_time;
            const lastEnd = lastPhrase.phrase.end_time;
            const title = newIds.length === 1
                ? firstPhrase.phrase.title
                : `${firstPhrase.phrase.title} ∼ ${lastPhrase.phrase.title}（${newIds.length}句）`;
            state.selectedStructure = { id: newIds.join(','), title, start: firstStart, end: lastEnd };
        }
    } else {
        state.selectedStructure = null;
    }

    generateReferenceData();
    updateSelectionUI();
}

function updateSelectionUI() {
    const { selectedStructure } = state;
    if (selectedStructure) {
        const formatTime = (sec) => {
            if (sec == null) return '0:00';
            const m = Math.floor(sec / 60);
            const s = Math.floor(sec % 60);
            return m + ':' + s.toString().padStart(2, '0');
        };
        document.getElementById('range-info').textContent =
            `🎯 ${selectedStructure.title}：${formatTime(selectedStructure.start)} - ${formatTime(selectedStructure.end)}`;
        document.getElementById('range-actions').style.display = 'flex';
        document.getElementById('current-range-label').textContent =
            `${selectedStructure.title} (${formatTime(selectedStructure.start)})`;
        // re-render structure list to show selection
        import('./practice-ui.js').then(m => {
            m.renderStructureList();
            m.renderLyricsPanel();
        });
    }
}

export async function onSongChange() {
    const select = document.getElementById('song-select');
    state.currentSongId = select.value || null;
    state.selectedStructure = null;
    state.selectedPhraseIds = [];
    state.selectedPhrasesData = [];
    document.getElementById('range-actions').style.display = 'none';
    document.getElementById('range-info').textContent = '選擇一個段落或句子開始練習';

    if (!state.currentSongId) {
        document.getElementById('song-title').textContent = '請選擇歌曲';
        document.getElementById('song-artist').textContent = '';
        document.getElementById('vocal-tracks').innerHTML = '<div class="no-structures">請先選擇歌曲</div>';
        document.getElementById('structure-list').innerHTML = '<div class="no-structures">請先選擇歌曲</div>';
        return;
    }

    try {
        const song = await api('/songs/' + state.currentSongId);
        state.currentSongFull = song;
        document.getElementById('song-title').textContent = song.title;
        document.getElementById('song-artist').textContent = song.artist;

        // Load MIDI for playback
        const midiData = await loadMIDI(state.currentSongId);
        state.midiData = midiData;
        state.parsedNotes = midiData ? window.MidiParser.parseMIDINotes(midiData) : null;

        // Render vocal tracks and accompaniment tracks
        const { renderVocalTracks, renderAccompanimentTracks } = await import('./practice-ui.js');
        renderVocalTracks(song.tracks);
        renderAccompanimentTracks(song.tracks);

        // Load structures for first track if available
        if (song.tracks.length > 0) {
            state.selectedTrackId = song.tracks[0].id;
            await selectTrack(song.tracks[0].id);
        }
    } catch (err) {
        console.error('Failed to load song:', err);
    }
}

export function toggleAccompanimentTrack(trackId) {
    const idx = state.selectedAccompanimentTrackIds.indexOf(trackId);
    if (idx === -1) {
        state.selectedAccompanimentTrackIds.push(trackId);
    } else {
        state.selectedAccompanimentTrackIds.splice(idx, 1);
    }
    // Re-render
    if (state.currentSongFull && state.currentSongFull.tracks) {
        import('./practice-ui.js').then(m => m.renderAccompanimentTracks(state.currentSongFull.tracks));
    }
}

export function startPractice() {
    const { selectedStructure, referenceNotes } = state;
    if (!selectedStructure) {
        alert('請先選擇一個段落或句子');
        return;
    }
    if (!referenceNotes || referenceNotes.length === 0) {
        alert('此段落尚無比對資料，請確認已選取正確的聲部');
        return;
    }
    const formatTime = (sec) => {
        if (sec == null) return '0:00';
        const m = Math.floor(sec / 60);
        const s = Math.floor(sec % 60);
        return m + ':' + s.toString().padStart(2, '0');
    };
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
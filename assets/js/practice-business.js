// practice-business.js — 純業務邏輯
// 依賴：state.js, api.js, audio.js, songDataExtractor.js (global)
// 不直接操作 DOM（UI 操作委託給 practice-ui.js）

import state from './state.js';
import { api, loadMIDI, loadStructures } from './api.js';
import { getAudioCtx, playRange, playRangeDelayed, stopPlayback, stopReplay, getIsReplaying, setSetReplayAudio, scheduleBeats, AudioProcess, stopAudioProcess } from './audio.js';

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
 * 從 structures 建立扁平化項目清單
 * 每個 section 和其底下的 phrases 依序排列
 */
function buildFlatItems() {
    const items = [];
    for (const s of state.structures) {
        items.push({ type: 'section', data: s, section: s });
        if (s.phrases) {
            for (const p of s.phrases) {
                items.push({ type: 'phrase', data: p, section: s });
            }
        }
    }
    return items;
}

/**
 * 從 selectionRange 計算合併後的 selectedStructure（start, end, title）
 */
function computeSelectedStructure() {
    const { flatItems, selectionRange } = state;
    const { from, to } = selectionRange;
    if (from === null || to === null || from > to) {
        state.selectedStructure = null;
        state.selectedPhraseIds = [];
        state.selectedPhrasesData = [];
        return;
    }

    const selected = flatItems.slice(from, to + 1);
    if (selected.length === 0) {
        state.selectedStructure = null;
        state.selectedPhraseIds = [];
        state.selectedPhrasesData = [];
        return;
    }

    // 收集所有選取的 phrases（含 section 隱含的 phrases）
    const allPhrases = [];
    const allPhraseIds = [];
    for (const item of selected) {
        if (item.type === 'phrase') {
            if (!allPhraseIds.includes(item.data.id)) {
                allPhraseIds.push(item.data.id);
                allPhrases.push(item.data);
            }
        } else if (item.type === 'section') {
            // section 含有其下所有 phrases
            if (item.data.phrases) {
                for (const p of item.data.phrases) {
                    if (!allPhraseIds.includes(p.id)) {
                        allPhraseIds.push(p.id);
                        allPhrases.push(p);
                    }
                }
            }
        }
    }

    // 從選取項目計算 start/end
    let minStart = Infinity;
    let maxEnd = -Infinity;
    for (const item of selected) {
        const start = item.type === 'section' ? item.data.start_time : item.data.start_time;
        const end = item.type === 'section' ? item.data.end_time : item.data.end_time;
        if (start < minStart) minStart = start;
        if (end > maxEnd) maxEnd = end;
    }

    if (minStart === Infinity) {
        state.selectedStructure = null;
        state.selectedPhraseIds = [];
        state.selectedPhrasesData = [];
        return;
    }

    // 產生 title
    let title;
    if (selected.length === 1) {
        title = selected[0].data.title;
    } else {
        const firstTitle = selected[0].data.title;
        const lastTitle = selected[selected.length - 1].data.title;
        const count = selected.length;
        const totalPhrases = allPhrases.length;
        title = `${firstTitle} ∼ ${lastTitle}（${count}項${totalPhrases > count ? `, ${totalPhrases}句` : ''}）`;
    }

    state.selectedStructure = { id: from + '-' + to, title, start: minStart, end: maxEnd };
    state.selectedPhraseIds = allPhraseIds;
    state.selectedPhrasesData = allPhrases;
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
    state.flatItems = [];
    state.selectionRange = { from: null, to: null };
    // Re-render both vocal and accompaniment track lists
    const { renderVocalTracks, renderAccompanimentTracks } = await import('./practice-ui.js');
    if (state.currentSongFull && state.currentSongFull.tracks) {
        renderVocalTracks(state.currentSongFull.tracks);
        renderAccompanimentTracks(state.currentSongFull.tracks);
    }
    const structures = await loadStructures(state.currentSongId, trackId);
    state.structures = structures;
    state.flatItems = buildFlatItems();
    const { renderStructureList, renderLyricsPanel } = await import('./practice-ui.js');
    renderStructureList();
    renderLyricsPanel();
}

/**
 * 選取結構（段落或句子），支援混合連續多選
 * @param {string} id - section 或 phrase 的 id
 */
export function selectStructure(id) {
    const { flatItems, selectionRange } = state;

    // 如果 flatItems 還沒建立，先建立
    if (flatItems.length === 0) {
        state.flatItems = buildFlatItems();
    }

    // 在 flatItems 中找到被點選的項目
    const clickedIdx = flatItems.findIndex(item => item.data.id === id);
    if (clickedIdx === -1) return;

    const { from, to } = selectionRange;

    // 情況 1: 目前沒有選取 → 選取點選的項目
    if (from === null || to === null) {
        state.selectionRange = { from: clickedIdx, to: clickedIdx };
        computeSelectedStructure();
        generateReferenceData();
        updateSelectionUI();
        return;
    }

    const minIdx = Math.min(from, to);
    const maxIdx = Math.max(from, to);

    // 情況 2: 點選的項目已在選取範圍內
    if (clickedIdx >= minIdx && clickedIdx <= maxIdx) {
        // 點選邊緣 → 縮減
        if (clickedIdx === minIdx) {
            state.selectionRange = { from: minIdx + 1, to: maxIdx };
        } else if (clickedIdx === maxIdx) {
            state.selectionRange = { from: minIdx, to: maxIdx - 1 };
        } else {
            // 點選中間 → 重設為只選該項
            state.selectionRange = { from: clickedIdx, to: clickedIdx };
        }
    } else {
        // 情況 3: 在範圍外 → 擴展
        if (clickedIdx < minIdx) {
            state.selectionRange = { from: clickedIdx, to: maxIdx };
        } else {
            state.selectionRange = { from: minIdx, to: clickedIdx };
        }
    }

    computeSelectedStructure();
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

export function clearSelection() {
    state.selectedStructure = null;
    state.selectedPhraseIds = [];
    state.selectedPhrasesData = [];
    state.selectionRange = { from: null, to: null };
    document.getElementById('range-info').textContent = '選擇一個段落或句子開始練習';
    document.getElementById('range-actions').style.display = 'none';
    document.getElementById('current-range-label').textContent = '未選擇';
    import('./practice-ui.js').then(m => {
        m.renderStructureList();
        m.renderLyricsPanel();
    });
}

export async function onSongChange() {
    const select = document.getElementById('song-select');
    state.currentSongId = select.value || null;
    state.selectedStructure = null;
    state.selectedPhraseIds = [];
    state.selectedPhrasesData = [];
    state.flatItems = [];
    state.selectionRange = { from: null, to: null };
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

// --- 3-beat + accompaniment playback helpers ---

/**
 * Calculate beat interval (quarter note duration in seconds) from BPM.
 * @param {number} bpm
 * @returns {number} interval in seconds
 */
function getBeatInterval(bpm) {
    return 60 / (bpm || 120);
}

/**
 * Get the BPM of the current song from parsed MIDI data.
 * @returns {number}
 */
function getSongBPM() {
    const { midiData } = state;
    if (!midiData) return 120;
    try {
        return window.MidiParser.extractBPM ? window.MidiParser.extractBPM(midiData) : 120;
    } catch (e) {
        return 120;
    }
}

/**
 * Get the first note's start time (in seconds relative to section start) for the practice range.
 * Used to calculate beat timing.
 * @param {number} sectionStart
 * @param {number} sectionEnd
 * @returns {{firstNoteTime: number, beatInterval: number, bpm: number}}
 */
function getTimingInfo(sectionStart, sectionEnd) {
    const { parsedNotes, parsedTrackIndex } = state;
    const bpm = getSongBPM();
    const beatInt = getBeatInterval(bpm);

    // Find the first note in the vocal track within this range
    let firstNoteTime = 0;
    if (parsedNotes && parsedTrackIndex >= 0) {
        const rangeNotes = parsedNotes.filter(n =>
            n.track === parsedTrackIndex &&
            n.start >= sectionStart &&
            n.start < sectionEnd
        );
        if (rangeNotes.length > 0) {
            rangeNotes.sort((a, b) => a.start - b.start);
            firstNoteTime = rangeNotes[0].start - sectionStart;
        }
    }

    return { firstNoteTime, beatInterval: beatInt, bpm };
}

/**
 * Play a section/phrase range with 3-beat lead-in.
 * Used by both section/phrase ▶️ buttons (no recording) and "開始練習" (with recording).
 *
 * Timing reference (relative to the section's first note):
 *   T = firstNote - 3×beatInterval - 0.25: Accompaniment starts
 *   T = firstNote - 2×beatInterval - 0.25: Beat 1
 *   T = firstNote - beatInterval - 0.25:    Beat 2
 *   T = firstNote - 0.25:                   Beat 3
 *   T = firstNote - 0.25:                   Recording starts (when withRecording=true) — 250ms before first note
 *
 * Accompaniment plays the section range with a scheduled delay so the first note
 * lands at the correct musical time after the 3-beat count-in.
 *
 * @param {number} sectionStart - Section start time in seconds
 * @param {number} sectionEnd - Section end time in seconds
 * @param {boolean} withRecording - Whether to also start recording
 */
export function playRangeWithBeats(sectionStart, sectionEnd, withRecording = false) {
    const { firstNoteTime, beatInterval, bpm } = getTimingInfo(sectionStart, sectionEnd);

    const ctx = getAudioCtx();
    const now = ctx.currentTime;

    // We want the first note of the section to play at:
    //   now + accDelay + firstNoteTime
    // where accDelay is the delay before accompaniment starts, and firstNoteTime is the
    // offset from sectionStart to the first note.
    //
    // Beats:
    //   Beat 3 (last beat) at firstNoteAbsTime - 0.25 (= 250ms before first note)
    //   Beat 2 at firstNoteAbsTime - beatInterval - 0.25
    //   Beat 1 at firstNoteAbsTime - 2×beatInterval - 0.25
    //
    // To ensure Beat 1 is in the future: accDelay + firstNoteTime - 2×beatInterval - 0.25 >= 0
    // => accDelay >= 2×beatInterval + 0.25 - firstNoteTime
    const minAccDelay = Math.max(2 * beatInterval + 0.25 - firstNoteTime, 0.1);
    const accDelay = minAccDelay;

    const firstNoteAbsTime = now + accDelay + firstNoteTime;

    // Beat 1, 2, 3: 3 clicks at beatInterval intervals
    // Beat 3 (the last count-in beat) is at firstNoteAbsTime - 0.25
    // So Beat 1 = firstNoteAbsTime - 2×beatInterval - 0.25
    const beat1Time = firstNoteAbsTime - 2 * beatInterval - 0.25;
    scheduleBeats(ctx, 3, beatInterval, beat1Time);

    // Play accompaniment with delay so the first note lands at firstNoteAbsTime
    playRangeDelayed(sectionStart, sectionEnd, accDelay);

    // Start recording from Beat 1 of the count-in (metronome beats)
    // Beat 1 time = accDelay + firstNoteTime - 2*beatInterval - 0.25
    // Will trim the first 2*beatInterval + 0.25 seconds later to skip all beats
    if (withRecording) {
        const beat1Time = accDelay + firstNoteTime - 2 * beatInterval - 0.25;
        const recDelay = Math.max(0, beat1Time) * 1000;
        setTimeout(() => {
            startMediaRecorder();
        }, recDelay);
    }
}

/**
 * Start MediaRecorder for recording user's voice.
 * Tracks are stored for later upload/analysis.
 */
let mediaRecorder = null;
let recordedChunks = [];
let practiceTimerInterval = null;
let practiceEndTimer = null;
let isPracticeActive = false;

function startMediaRecorder() {
    if (mediaRecorder && mediaRecorder.state === 'recording') return;
    recordedChunks = [];
    if (!window.isSecureContext) {
        console.warn('Not a secure context — getUserMedia may be blocked. Use localhost or HTTPS.');
    }
    navigator.mediaDevices.getUserMedia({ audio: true })
        .then(stream => {
            mediaRecorder = new MediaRecorder(stream);
            mediaRecorder.ondataavailable = (e) => {
                if (e.data.size > 0) {
                    recordedChunks.push(e.data);
                }
            };
            mediaRecorder.start();
        })
        .catch(err => {
            console.error('Failed to start recording:', err);
            alert('無法存取麥克風。請確認已允許麥克風權限，並使用 localhost 或 HTTPS 連線。');
        });
}

export function stopMediaRecorder() {
    return new Promise((resolve) => {
        if (mediaRecorder && mediaRecorder.state === 'recording') {
            mediaRecorder.onstop = () => {
                try {
                    mediaRecorder.stream.getTracks().forEach(t => t.stop());
                } catch (e) {
                    // stream may already be stopped
                }
                mediaRecorder = null;
                resolve();
            };
            mediaRecorder.stop();
        } else {
            mediaRecorder = null;
            resolve();
        }
    });
}

/**
 * 清除練習狀態：timer、end timer、錄音
 * @param {boolean} discardRecording - 是否丟棄錄音資料
 */
function cleanupPractice(discardRecording = false) {
    if (practiceTimerInterval) {
        // practiceTimerInterval 可能是 setTimeout 或 setInterval 的 ID
        clearTimeout(practiceTimerInterval);
        clearInterval(practiceTimerInterval);
        practiceTimerInterval = null;
    }
    if (practiceEndTimer) {
        clearTimeout(practiceEndTimer);
        practiceEndTimer = null;
    }
    isPracticeActive = false;
    const stopPromise = stopMediaRecorder();
    if (discardRecording) {
        recordedChunks = [];
    }
    // Timer 歸零
    const timerEl = document.getElementById('timer');
    if (timerEl) timerEl.textContent = '00:00';
    return stopPromise;
}

/**
 * 顯示回放錄音按鈕（練習結束後）
 */
function showReplayButton() {
    const chartArea = document.getElementById('chart-area');
    if (!chartArea) return;
    if (recordedChunks.length === 0) return;

    // 創建按鈕容器
    const btnContainer = document.createElement('div');
    btnContainer.style.cssText = 'margin-top:16px;display:flex;gap:8px;align-items:center;';

    const replayBtn = document.createElement('button');
    replayBtn.className = 'btn btn-sm btn-success';
    replayBtn.style.cssText = 'padding:10px 20px;font-size:14px;';
    replayBtn.textContent = '🔁 回放錄音';

    const stopReplayBtn = document.createElement('button');
    stopReplayBtn.className = 'btn btn-sm btn-danger';
    stopReplayBtn.style.cssText = 'padding:10px 16px;font-size:14px;display:none;';
    stopReplayBtn.textContent = '⏹ 中斷回放';

    replayBtn.onclick = () => {
        const blob = new Blob(recordedChunks, { type: 'audio/webm' });
        const url = URL.createObjectURL(blob);
        const audio = new Audio(url);
        
        // 設置回放音頻引用
        setSetReplayAudio(audio);
        
        audio.onloadedmetadata = () => {
            // 從錄音截除點之後開始播放，截掉所有提示音（Beat 1, 2, 3）
            const { beatInterval } = getTimingInfo(state.selectedStructure.start, state.selectedStructure.end);
            const trimOffset = 2 * beatInterval + 0.25;
            audio.currentTime = trimOffset;
        };
        
        audio.onended = () => {
            URL.revokeObjectURL(url);
            setSetReplayAudio(null);
            replayBtn.textContent = '🔁 再次回放';
            replayBtn.disabled = false;
            stopReplayBtn.style.display = 'none';
        };
        
        audio.onplay = () => {
            stopReplayBtn.style.display = 'inline-block';
            replayBtn.disabled = true;
        };
        
        audio.play();
    };

    stopReplayBtn.onclick = () => {
        stopAudioProcess(AudioProcess.RECORDING_REPLAY);
        setSetReplayAudio(null);
        replayBtn.textContent = '🔁 再次回放';
        replayBtn.disabled = false;
        stopReplayBtn.style.display = 'none';
    };

    btnContainer.appendChild(replayBtn);
    btnContainer.appendChild(stopReplayBtn);
    chartArea.appendChild(btnContainer);
}

/**
 * 停止回放錄音
 */
export function stopReplayPlayback() {
    stopReplay();
}

/**
 * Start practice: plays accompaniment with 3-beat lead-in, then starts recording.
 * User sings along with the accompaniment and the recording is sent for analysis.
 */
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
    const statusText = `練習：${selectedStructure.title}`;
    document.getElementById('status-text').textContent = statusText;
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

    // 清除之前的練習狀態
    cleanupPractice(true);

    // 計算練習時間參數（與 playRangeWithBeats 一致）
    const duration = selectedStructure.end - selectedStructure.start;
    const { firstNoteTime, beatInterval } = getTimingInfo(selectedStructure.start, selectedStructure.end);
    const minAccDelay = Math.max(2 * beatInterval + 0.25 - firstNoteTime, 0.1);
    const accDelay = minAccDelay;
    // 錄音開始時間（從第一個提示音 Beat 1 開始）
    const beat1Time = Math.max(0, accDelay + firstNoteTime - 2 * beatInterval - 0.25);
    const recStartDelay = beat1Time;
    // 截除偏移：從 Beat 1 到 Beat 3 結束的時間
    const recordingTrimOffset = 2 * beatInterval + 0.25;
    // 練習結束時間 = 錄音開始 + 截除偏移 + 練習範圍時長
    const practiceTotalMs = (recStartDelay + recordingTrimOffset + duration) * 1000;

    // 啟動秒數動畫（倒數計時）— 從錄音截除後開始計時
    const timerEl = document.getElementById('timer');
    if (timerEl) timerEl.textContent = formatTime(duration);
    let remaining = duration;
    practiceTimerInterval = setTimeout(() => {
        practiceTimerInterval = setInterval(() => {
            remaining -= 0.1;
            if (remaining <= 0) {
                remaining = 0;
            }
            if (timerEl) timerEl.textContent = formatTime(remaining);
        }, 100);
    }, Math.max(0, (recStartDelay + recordingTrimOffset) * 1000));

    // 設定練習結束時間（錄音結束後仍繼續錄到練習結束）
    isPracticeActive = true;
    practiceEndTimer = setTimeout(() => {
        cleanupPractice(false).then(() => { // 保留錄音，等待 MediaRecorder onstop 完成
            // 顯示回放按鈕
            showReplayButton();
            document.getElementById('status-text').textContent = '練習完成';
            // 顯示覆蓋提示訊息
            if (window.updateWarningVisibility) {
                window.updateWarningVisibility();
            }
        });
    }, practiceTotalMs);

    // Auto-play the range with beats + recording
    playRangeWithBeats(selectedStructure.start, selectedStructure.end, true);
}

/**
 * 中斷練習：停止播放、停止錄音、丟棄錄音資料、timer 歸零
 */
export function interruptPractice() {
    cleanupPractice(true); // 丟棄錄音
    stopAudioProcess(AudioProcess.PRACTICE_PLAYBACK);
    stopAudioProcess(AudioProcess.RECORDING_REPLAY);
    document.getElementById('status-text').textContent = '準備就緒';
    document.getElementById('chart-area').innerHTML = `
        <div>
            <div style="font-size:48px;margin-bottom:12px;">🎵</div>
            <div>練習已中斷</div>
        </div>
    `;
    // 更新提示訊息可見性
    if (window.updateWarningVisibility) {
        window.updateWarningVisibility();
    }
}

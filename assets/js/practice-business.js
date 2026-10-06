// practice-business.js — 純業務邏輯
// 依賴：state.js, api.js, audio.js, songDataExtractor.js (global)
// 不直接操作 DOM（UI 操作委託給 practice-ui.js）

import state from './state.js';
import { resetSelection } from './selection.js';
import { api, loadMIDI, loadStructures } from './api.js';
import { getAudioCtx, playRange, playRangeDelayed, stopPlayback, stopReplay, getIsReplaying, setReplayAudio, scheduleBeats, AudioProcess, stopAudioProcess } from './audio.js';

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


export function getLyricsForSelection() {
    const { structures, selectedPhrasesData, selectedSectionIds } = state;
    const result = { phrases: [], count: 0 };

    // 個別選取的句子，以及整段選取所涵蓋的句子
    for (const p of selectedPhrasesData) {
        result.phrases.push({
            title: p.title,
            lyrics: p.lyrics || ''
        });
    }

    // 整段選取但該段沒有句子：顯示 section 自己的標題/歌詞
    // (selectedStructure.id 是合成的 'sel-<timestamp>'，不對應任何真實 structure，
    // 所以這裡改用 selectedSectionIds 直接比對)
    for (const sectionId of selectedSectionIds) {
        const s = structures.find(x => x.id === sectionId);
        if (s && (!s.phrases || s.phrases.length === 0)) {
            result.phrases.push({
                title: s.title,
                lyrics: s.lyrics || ''
            });
        }
    }

    result.count = result.phrases.length;
    return result;
}

export async function selectTrack(trackId) {
    state.selectedTrackId = trackId;
    state.referenceNotes = null;
    state.parsedTrackIndex = determineParsedTrackIndex();
    resetSelection();
    const { renderVocalTracks, renderAccompanimentTracks } = await import('./practice-ui.js');
    if (state.currentSongFull && state.currentSongFull.tracks) {
        renderVocalTracks(state.currentSongFull.tracks);
        renderAccompanimentTracks(state.currentSongFull.tracks);
    }
    const structures = await loadStructures(state.currentSongId, trackId);
    state.structures = structures;
    const selection = await import('./selection.js');
    if (state.structures.length > 0) {
        await selection.selectSection(state.structures[0].id);
    } else {
        const ui = await import('./selection-ui.js');
        ui.refreshSelectionUI();
    }
}



export async function onSongChange() {
    const select = document.getElementById('song-select');
    state.currentSongId = select.value || null;
    resetSelection();
    document.getElementById('range-actions').style.display = 'none';
    document.getElementById('range-info').textContent = '選擇一個段落或句子開始練習';

    if (!state.currentSongId) {
        document.getElementById('song-title').textContent = '請選擇歌曲';
        document.getElementById('song-artist').textContent = '';
        document.getElementById('vocal-tracks').innerHTML = '<div class="no-structures">請先選擇歌曲</div>';
        document.getElementById('structure-list').innerHTML = '<div class="no-structures">請先選擇歌曲</div>';
        // 隱藏結果面板
        const resultsPanel = document.getElementById('results-panel-foldable');
        if (resultsPanel) {
            resultsPanel.classList.remove('visible');
            resultsPanel.classList.add('hidden');
            resultsPanel.setAttribute('data-folded', 'true');
            const icon = document.getElementById('results-panel-toggle-icon');
            if (icon) icon.textContent = '▼';
            const body = document.getElementById('results-panel-body');
            if (body) body.innerHTML = '';
        }
        // 隱藏音高準度折疊面板
        const pitchPanel = document.getElementById('pitch-panel');
        if (pitchPanel) {
            pitchPanel.classList.remove('visible');
            pitchPanel.classList.add('hidden');
            pitchPanel.setAttribute('data-folded', 'true');
            const icon = document.getElementById('pitch-panel-toggle-icon');
            if (icon) icon.textContent = '▼';
            const pitchChart = document.getElementById('pitch-chart');
            if (pitchChart) pitchChart.innerHTML = '';
        }
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
 * Compute the count-in/accompaniment timing for a practice range, anchored to a given
 * AudioContext.currentTime. A standard "1, 2, 3, (sing)" count-in: the first note lands
 * exactly one beat interval after the last click, regardless of tempo — a fixed gap
 * (the previous behavior) only felt musically right by coincidence around ~240 BPM.
 *
 * @param {number} sectionStart
 * @param {number} sectionEnd
 * @param {number} now - ctx.currentTime at the moment playback is being scheduled
 */
function computeCountInTiming(sectionStart, sectionEnd, now) {
    const { firstNoteTime, beatInterval, bpm } = getTimingInfo(sectionStart, sectionEnd);

    // Ensure beat 1 lands in the future: accDelay + firstNoteTime - 3×beatInterval >= 0
    const accDelay = Math.max(3 * beatInterval - firstNoteTime, 0.1);
    const firstNoteAbsTime = now + accDelay + firstNoteTime;
    const beat1AbsTime = firstNoteAbsTime - 3 * beatInterval;
    const recordingTrimSec = 3 * beatInterval;

    return { firstNoteTime, beatInterval, bpm, accDelay, now, firstNoteAbsTime, beat1AbsTime, recordingTrimSec };
}

/**
 * Play a section/phrase range with 3-beat lead-in.
 * Used by both section/phrase ▶️ buttons (no recording) and "開始練習" (with recording).
 *
 * Accompaniment plays the section range with a scheduled delay so the first note
 * lands at the correct musical time after the 3-beat count-in.
 *
 * @param {number} sectionStart - Section start time in seconds
 * @param {number} sectionEnd - Section end time in seconds
 * @param {boolean} withRecording - Whether a recording is already in progress (the
 *   microphone must already be connected via prepareMicrophone() — see startPractice)
 * @returns {object} the computed timing, so callers scheduling their own timers (e.g.
 *   startPractice) can stay exactly consistent with the audio scheduling here instead
 *   of recomputing the same formula a second time.
 */
export function playRangeWithBeats(sectionStart, sectionEnd, withRecording = false) {
    const ctx = getAudioCtx();
    const now = ctx.currentTime;
    const timing = computeCountInTiming(sectionStart, sectionEnd, now);
    const { accDelay, beat1AbsTime, beatInterval } = timing;

    // Play accompaniment with delay so the first note lands at firstNoteAbsTime.
    // Must run BEFORE scheduleBeats: playRangeDelayed calls stopPlayback() as its
    // first step, which disconnects every node in the shared playbackNodes list —
    // if the click nodes were added first, they'd be disconnected here before
    // they ever get a chance to sound.
    playRangeDelayed(sectionStart, sectionEnd, accDelay);

    // 3 clicks at beatInterval intervals, the last one exactly one beat before the note.
    scheduleBeats(ctx, 3, beatInterval, beat1AbsTime);

    if (withRecording) {
        // The mic is expected to already be connected (startPractice awaits
        // prepareMicrophone() before calling this) — this just marks the ctx-time
        // anchor stopMediaRecorder() uses to compute an accurate trim.
        recordingAnchorCtxTime = beat1AbsTime;
    }

    return timing;
}

/**
 * Start recording user's voice using Web Audio API.
 * Audio is captured as 16-bit PCM WAV.
 */
let audioContext = null;
let mediaStream = null; // Keep reference to media stream to stop tracks later
let audioInput = null;
let scriptProcessorNode = null;
let audioBuffer = []; // Float32Array samples
let recordedWavBlob = null; // Encoded WAV blob
let practiceTimerInterval = null;
let practiceEndTimer = null;
let isPracticeActive = false;
// ctx-time (AudioContext.currentTime timeline) the mic buffer actually started, captured
// from the first onaudioprocess callback — used to compute an accurate trim instead of
// assuming the buffer start coincides with when we asked for the mic.
let recordingBufferStartCtxTime = null;
// ctx-time "beat 1" was scheduled for, set by playRangeWithBeats.
let recordingAnchorCtxTime = null;
// Incremented on every startPractice()/interruptPractice() call so an in-flight
// getUserMedia() promise from a superseded attempt can detect it's stale and back out
// instead of resurrecting a stream that was supposed to be torn down.
let practiceSessionId = 0;

function encodeWAV(samples, sampleRate) {
    // Convert Float32Array to 16-bit PCM
    const buffer = new ArrayBuffer(44 + samples.length * 2); // WAV header + PCM data
    const view = new DataView(buffer);

    // RIFF header
    writeString(view, 0, 'RIFF');
    view.setUint32(4, 36 + samples.length * 2, true); // ChunkSize
    writeString(view, 8, 'WAVE');
    writeString(view, 12, 'fmt ');
    view.setUint32(16, 16, true); // Subchunk1Size (PCM)
    view.setUint16(20, 1, true); // AudioFormat (PCM)
    view.setUint16(22, 1, true); // NumChannels (mono)
    view.setUint32(24, sampleRate, true); // SampleRate
    view.setUint32(28, sampleRate * 2, true); // ByteRate (SampleRate * NumChannels * BitsPerSample/8)
    view.setUint16(32, 2, true); // BlockAlign (NumChannels * BitsPerSample/8)
    view.setUint16(34, 16, true); // BitsPerSample
    writeString(view, 36, 'data');
    view.setUint32(40, samples.length * 2, true); // Subchunk2Size

    // Write PCM data
    let offset = 44;
    for (let i = 0; i < samples.length; i++) {
        let s = Math.max(-1, Math.min(1, samples[i])); // Clamp to [-1, 1]
        let val = s < 0 ? s * 0x8000 : s * 0x7FFF; // Convert to 16-bit signed integer
        view.setInt16(offset, val, true); // Little endian
        offset += 2;
    }

    return new Blob([buffer], { type: 'audio/wav' });
}

function writeString(view, offset, string) {
    for (let i = 0; i < string.length; i++) {
        view.setUint8(offset + i, string.charCodeAt(i));
    }
}

/**
 * Request microphone access and wire up the recording audio graph. Awaited by
 * startPractice() *before* the count-in starts, so mic-acquisition latency (which varies
 * a lot by browser/OS/permission state) happens during a visible "準備麥克風中" step
 * instead of racing silently against the count-in — see the caller for the user-facing
 * messaging.
 * @returns {Promise<void>}
 */
function prepareMicrophone() {
    audioBuffer = [];
    recordedWavBlob = null;
    recordingBufferStartCtxTime = null;

    const constraints = {
        audio: {
            sampleRate: 48000, // Use a standard sample rate
            channelCount: 1,
            echoCancellation: false,
            noiseSuppression: false,
            autoGainControl: false
        }
    };

    return navigator.mediaDevices.getUserMedia(constraints).then(stream => {
        const audioTracks = stream.getAudioTracks();
        if (audioTracks.length === 0) {
            stream.getTracks().forEach(track => track.stop());
            throw new Error('No audio input device found');
        }

        mediaStream = stream;
        audioContext = getAudioCtx();

        // Resume audio context if suspended (required for autoplay policies)
        if (audioContext.state === 'suspended') {
            audioContext.resume().catch(err => console.error('Failed to resume audio context:', err));
        }

        audioInput = audioContext.createMediaStreamSource(stream);

        const bufferSize = 4096;
        scriptProcessorNode = audioContext.createScriptProcessor(bufferSize, 1, 1);

        scriptProcessorNode.onaudioprocess = function(e) {
            if (!isPracticeActive) return; // Stop processing once practice ends/is interrupted

            // Capture the real ctx-time this buffer started on, on the first callback
            // only — used by stopMediaRecorder() to compute an accurate trim.
            if (recordingBufferStartCtxTime === null) {
                recordingBufferStartCtxTime = e.playbackTime;
            }

            const input = e.inputBuffer.getChannelData(0); // Mono - use channel 0
            for (let i = 0; i < input.length; i++) {
                audioBuffer.push(input[i]);
            }
        };

        audioInput.connect(scriptProcessorNode);
        scriptProcessorNode.connect(audioContext.destination);
    });
}

export function stopMediaRecorder() {
    return new Promise((resolve) => {
        // Disconnect and clean up audio nodes
        if (scriptProcessorNode) {
            scriptProcessorNode.onaudioprocess = null;
            scriptProcessorNode.disconnect();
            scriptProcessorNode = null;
        }
        if (audioInput) {
            audioInput.disconnect();
            audioInput = null;
        }
        // Note: We don't close audioContext as it's shared and used for playback

        // Stop any media stream tracks (cleanup)
        if (mediaStream) {
            mediaStream.getTracks().forEach(track => track.stop());
            mediaStream = null;
        }

        // Encode WAV blob if we have audio data
        if (audioBuffer.length > 0) {
            // Use AudioContext sample rate if available, otherwise default to 44100
            const sampleRate = audioContext ? audioContext.sampleRate : 44100;

            // Trim the count-in lead-in from the beginning of the recording. Prefer the
            // *measured* gap between when the mic buffer actually started
            // (recordingBufferStartCtxTime, captured from the first onaudioprocess call)
            // and when the count-in's beat 1 was scheduled (recordingAnchorCtxTime) — both
            // on the same AudioContext.currentTime timeline — over the fixed-duration
            // fallback, which has no way to account for how long the mic took to connect.
            let trimSec = state.recordingTrimOffset || 0;
            if (recordingBufferStartCtxTime != null && recordingAnchorCtxTime != null) {
                trimSec = Math.max(0, recordingAnchorCtxTime - recordingBufferStartCtxTime);
            }

            let bufferToEncode = audioBuffer;
            const trimSamples = Math.floor(trimSec * sampleRate);
            if (trimSamples > 0 && trimSamples < audioBuffer.length) {
                bufferToEncode = audioBuffer.slice(trimSamples);
            }

            const nonZeroCount = bufferToEncode.reduce((count, sample) => Math.abs(sample) > 0.001 ? count + 1 : count, 0);
            const percentNonZero = (nonZeroCount / bufferToEncode.length) * 100;
            if (percentNonZero < 0.1) {
                console.warn('Warning: Audio appears to be mostly silence - check microphone permissions and input');
            }

            recordedWavBlob = encodeWAV(new Float32Array(bufferToEncode), sampleRate);
            audioBuffer = []; // Clear buffer after encoding
        }

        recordingBufferStartCtxTime = null;
        recordingAnchorCtxTime = null;
        resolve();
    });
}

/**
 * 顯示/隱藏「錄音中」提示，讓使用者能確認系統目前是否正在聽自己唱歌。
 * @param {boolean} show
 */
function showRecordingBadge(show) {
    let badge = document.getElementById('recording-badge');
    if (show) {
        if (!badge) {
            const statusEl = document.getElementById('status-text');
            if (!statusEl || !statusEl.parentNode) return;
            badge = document.createElement('span');
            badge.id = 'recording-badge';
            badge.style.cssText = 'margin-left:8px;color:#e74c3c;font-weight:bold;';
            badge.textContent = '🔴 錄音中';
            statusEl.parentNode.insertBefore(badge, statusEl.nextSibling);
        }
    } else if (badge) {
        badge.remove();
    }
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
    showRecordingBadge(false);
    const stopPromise = stopMediaRecorder();
    if (discardRecording) {
        recordedWavBlob = null;
    }
    // Timer 歸零
    const timerEl = document.getElementById('timer');
    if (timerEl) timerEl.textContent = '00:00';
    return stopPromise;
}

/**
 * 顯示回放錄音按鈕（練習結束後）
 */
export function showReplayButton() {
    const chartArea = document.getElementById('chart-area');
    if (!chartArea) return;

    // 移除已存在的回放按鈕容器（避免重複顯示）
    const existingContainer = document.getElementById('replay-btn-container');
    if (existingContainer) {
        existingContainer.remove();
    }

    if (!recordedWavBlob) return;

    // 創建按鈕容器
    const btnContainer = document.createElement('div');
    btnContainer.id = 'replay-btn-container';
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
        const url = URL.createObjectURL(recordedWavBlob);
        const audio = new Audio(url);
        
        // 設置回放音頻引用
        setReplayAudio(audio);
        
        audio.onloadedmetadata = () => {
            // The count-in (3-beat lead-in) was already trimmed during WAV encoding
            // in stopMediaRecorder(), so the WAV blob starts at the user's first note.
            // No need to seek forward during replay.
            audio.currentTime = 0;
        };
        
        audio.onended = () => {
            URL.revokeObjectURL(url);
            setReplayAudio(null);
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
        setReplayAudio(null);
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
    // 移除回放按鈕容器
    const existingContainer = document.getElementById('replay-btn-container');
    if (existingContainer) {
        existingContainer.remove();
    }
}

/**
 * 錄製的音訊 Blob (WAV 格式) 取得器
 * @returns {Blob|null} 錄製的音訊 Blob，如果沒有錄製則返回 null
 */
export function getRecordedAudioBlob() {
    return recordedWavBlob || null;
}

/**
 * 取得當前是否處於練習活動狀態
 * @returns {boolean}
 */
export function isPracticeActivityActive() {
    return isPracticeActive;
}

/**
 * 取得錄製的音訊作為 Base64 編碼字串
 * @returns {Promise<string|null>} Base64 編碼的音訊資料，如果沒有錄製則返回 null
 */
export function getRecordedAudioBase64() {
    return new Promise((resolve, reject) => {
        if (!recordedWavBlob) {
        // console.log('No recorded WAV blob available');
            resolve(null);
            return;
        }
        
        // console.log('Converting WAV blob to base64, size:', recordedWavBlob.size, 'bytes');
        
        const reader = new FileReader();
        reader.onloadend = () => {
            // Remove the data:audio/wav;base64, prefix if present
            const base64String = (reader.result || '').toString().split(',')[1] || '';
        // console.log('Base64 conversion complete, length:', base64String.length, 'characters');
            resolve(base64String);
        };
        reader.onerror = (err) => {
            console.error('Failed to convert WAV blob to base64:', err);
            reject(err);
        };
        reader.readAsDataURL(recordedWavBlob);
    });
}

/**
 * 排程視覺倒數提示（3、2、1、開始唱！），與音訊排程使用同一組 timing 資訊，
 * 確保畫面與提示音同步。只有在 practiceSessionId 仍相符時才會實際更新畫面，
 * 避免使用者已按下中斷後，稍後才顯示過期的倒數文字。
 * @param {object} timing - playRangeWithBeats() 回傳的 timing 物件
 * @param {number} mySession - 這次練習的 practiceSessionId
 */
function scheduleCountInVisuals(timing, mySession) {
    const { now, beat1AbsTime, beatInterval, firstNoteAbsTime } = timing;
    const chartArea = document.getElementById('chart-area');
    if (!chartArea) return;

    const cueEl = document.createElement('div');
    cueEl.className = 'count-in-cue';
    cueEl.style.cssText = 'font-size:56px;font-weight:bold;margin-top:12px;color:#f39c12;';
    chartArea.appendChild(cueEl);

    const showCue = (text) => {
        if (mySession !== practiceSessionId) return; // 已被中斷/取代，不再更新畫面
        const cue = chartArea.querySelector('.count-in-cue');
        if (cue) cue.textContent = text;
    };

    const cues = [
        [beat1AbsTime, '3'],
        [beat1AbsTime + beatInterval, '2'],
        [beat1AbsTime + 2 * beatInterval, '1'],
        [firstNoteAbsTime, '🎤 開始唱！']
    ];
    for (const [absTime, text] of cues) {
        setTimeout(() => showCue(text), Math.max(0, (absTime - now) * 1000));
    }

    // 開始唱的同時顯示「錄音中」提示，讓使用者確認系統正在聽自己唱歌。
    setTimeout(() => {
        if (mySession === practiceSessionId) showRecordingBadge(true);
    }, Math.max(0, (firstNoteAbsTime - now) * 1000));
}

/**
 * Start practice: requests/confirms the microphone first (with the user told what's
 * happening at each step), then plays accompaniment with a 3-beat lead-in and records.
 * User sings along with the accompaniment and the recording is sent for analysis.
 */
export async function startPractice() {
    const { selectedStructure, referenceNotes } = state;
    if (!selectedStructure) {
        alert('請先選擇一個段落或句子');
        return;
    }
    if (!referenceNotes || referenceNotes.length === 0) {
        alert('此段落尚無比對資料，請確認已選取正確的聲部');
        return;
    }

    const statusEl = document.getElementById('status-text');

    // 環境檢查：在播放任何東西之前先確認錄音功能可用，避免使用者練習完才發現沒錄到。
    if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
        alert('此瀏覽器不支援錄音功能，請使用最新版 Chrome、Firefox 或 Safari 再試一次。');
        return;
    }
    if (!window.isSecureContext) {
        alert('目前的連線環境不支援錄音功能，請透過 HTTPS 或 localhost 連線後再試一次。');
        return;
    }

    const formatTime = (sec) => {
        if (sec == null) return '0:00';
        const m = Math.floor(sec / 60);
        const s = Math.floor(sec % 60);
        return m + ':' + s.toString().padStart(2, '0');
    };

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

    // 清除之前的練習狀態
    cleanupPractice(true);
    // 隱藏之前的結果面板
    const resultsPanel = document.getElementById('results-panel-foldable');
    if (resultsPanel) {
        resultsPanel.classList.remove('visible');
        resultsPanel.classList.add('hidden');
        resultsPanel.setAttribute('data-folded', 'true');
        const icon = document.getElementById('results-panel-toggle-icon');
        if (icon) icon.textContent = '▼';
        const body = document.getElementById('results-panel-body');
        if (body) body.innerHTML = '';
    }
    // 隱藏音高準度折疊面板
    const pitchPanel = document.getElementById('pitch-panel');
    if (pitchPanel) {
        pitchPanel.classList.remove('visible');
        pitchPanel.classList.add('hidden');
        pitchPanel.setAttribute('data-folded', 'true');
        const icon = document.getElementById('pitch-panel-toggle-icon');
        if (icon) icon.textContent = '▼';
        const pitchChart = document.getElementById('pitch-chart');
        if (pitchChart) pitchChart.innerHTML = '';
    }

    // 這次練習的識別碼：若使用者在等待麥克風權限時按下中斷，
    // interruptPractice() 會讓 practiceSessionId 不再相符，藉此偵測並中止。
    const mySession = ++practiceSessionId;

    // 先確認麥克風可用，再開始提示音/伴奏 —— 讓 getUserMedia() 的不確定延遲
    // 發生在明確告知使用者「準備中」的階段，而不是悄悄跟倒數提示音賽跑。
    statusEl.textContent = '準備麥克風中，請允許瀏覽器使用麥克風權限…';
    try {
        await prepareMicrophone();
    } catch (err) {
        console.error('Failed to start recording:', err);
        statusEl.textContent = '無法使用麥克風';
        document.getElementById('chart-area').innerHTML = `
            <div>
                <div style="font-size:48px;margin-bottom:12px;">🎤🚫</div>
                <div>無法取得麥克風權限</div>
                <div style="color:#888;font-size:13px;margin-top:8px;">
                    請確認已允許此網站使用麥克風，並確認裝置已連接麥克風後再試一次。
                </div>
                <div style="color:#555;font-size:11px;margin-top:6px;">${err.message || ''}</div>
            </div>
        `;
        return;
    }

    // 等待麥克風權限期間，使用者可能已按下中斷或重新開始了另一次練習。
    if (mySession !== practiceSessionId) {
        stopMediaRecorder();
        return;
    }

    statusEl.textContent = `練習：${selectedStructure.title}`;

    const duration = selectedStructure.end - selectedStructure.start;

    // Resume audio context if suspended
    const audioCtx = getAudioCtx();
    if (audioCtx.state === 'suspended') {
        audioCtx.resume().catch(err => console.error('Failed to resume audio context:', err));
    }

    isPracticeActive = true;

    // Auto-play the range with beats + recording; playRangeWithBeats returns the exact
    // timing it scheduled so the timers below stay perfectly consistent with the audio.
    const timing = playRangeWithBeats(selectedStructure.start, selectedStructure.end, true);
    const { firstNoteTime, beat1AbsTime, recordingTrimSec, now } = timing;

    scheduleCountInVisuals(timing, mySession);

    const recStartDelay = Math.max(0, beat1AbsTime - now);
    const practiceTotalMs = (recStartDelay + recordingTrimSec + duration) * 1000;

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
    }, Math.max(0, (recStartDelay + recordingTrimSec) * 1000));

    // 存儲錄音時間資訊，用於後端音符比對的時間對齊
    state.recordingTrimOffset = recordingTrimSec;
    state.recordingStartTime = selectedStructure.start + firstNoteTime;

    practiceEndTimer = setTimeout(() => {
        cleanupPractice(false).then(() => { // 保留錄音，等待 MediaRecorder onstop 完成
            // 分析並提交錄音
            analyzeAndSubmitRecording().then(() => {
                // 分析完成後顯示回放按鈕（狀態文字已由 analyzeAndSubmitRecording 設定）
                showReplayButton();
                if (window.updateWarningVisibility) {
                    window.updateWarningVisibility();
                }
            }).catch(error => {
                // 分析失敗時仍顯示回放按鈕並顯示錯誤
                showReplayButton();
                statusEl.textContent = '分析失敗: ' + error.message;
                if (window.updateWarningVisibility) {
                    window.updateWarningVisibility();
                }
            });
        });
    }, practiceTotalMs);
}

/**
 * 中斷練習：停止播放、停止錄音、丟棄錄音資料、timer 歸零
 */
export function interruptPractice() {
    practiceSessionId++; // 讓等待中的 prepareMicrophone()/視覺倒數偵測到已被中斷
    cleanupPractice(true); // 丟棄錄音
    stopAudioProcess(AudioProcess.PRACTICE_PLAYBACK);
    stopAudioProcess(AudioProcess.RECORDING_REPLAY);
    // 隱藏結果面板
    const resultsPanel = document.getElementById('results-panel-foldable');
    if (resultsPanel) {
        resultsPanel.classList.remove('visible');
        resultsPanel.classList.add('hidden');
        resultsPanel.setAttribute('data-folded', 'true');
        const icon = document.getElementById('results-panel-toggle-icon');
        if (icon) icon.textContent = '▼';
        const body = document.getElementById('results-panel-body');
        if (body) body.innerHTML = '';
    }
    // 隱藏音高準度折疊面板
    const pitchPanel = document.getElementById('pitch-panel');
    if (pitchPanel) {
        pitchPanel.classList.remove('visible');
        pitchPanel.classList.add('hidden');
        pitchPanel.setAttribute('data-folded', 'true');
        const icon = document.getElementById('pitch-panel-toggle-icon');
        if (icon) icon.textContent = '▼';
        const pitchChart = document.getElementById('pitch-chart');
        if (pitchChart) pitchChart.innerHTML = '';
    }
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

/**
 * 分析並提交錄音
 * 錄製結束後自動呼叫此函數進行音訊分析和結果提交
 * 會執行兩次評估：一次使用母音過濾（/analyze），一次不過濾（/analyze/basic）
 */
export async function analyzeAndSubmitRecording() {
    try {
        // 更新狀態
        document.getElementById('status-text').textContent = '正在分析錄音...';
        
        // 取得錄製的音訊 (Base64 編碼的 WAV)
        const audioBase64 = await getRecordedAudioBase64();
        if (!audioBase64) {
            throw new Error('無法取得錄音資料');
        }
        
        // 取得參考音符
        const referenceNotes = state.referenceNotes || [];
        if (!referenceNotes || referenceNotes.length === 0) {
            throw new Error('無可用的參考音符進行分析');
        }
        
        // 共用的請求 body
        const commonBody = {
            audio_data: audioBase64,
            audio_format: 'wav',
            reference_notes: referenceNotes,
            recording_start_time: state.recordingStartTime
        };
        
        // 同時發送兩個請求：母音過濾 + 基本（無過濾）
        const [vowelResult, basicResult] = await Promise.allSettled([
            api('/assessments/analyze', {
                method: 'POST',
                body: JSON.stringify(commonBody)
            }),
            api('/assessments/analyze/basic', {
                method: 'POST',
                body: JSON.stringify(commonBody)
            })
        ]);
        
        // 日誌記錄
        // console.log('Analysis API (vowel-filtered) response:', vowelResult);
        // console.log('Analysis API (basic) response:', basicResult);
        
        // 擷取結果（如果請求失敗則為 null）
        const vowelAssessmentResult = vowelResult.status === 'fulfilled' ? vowelResult.value : null;
        const basicAssessmentResult = basicResult.status === 'fulfilled' ? basicResult.value : null;
        
        if (vowelResult.status === 'rejected' && basicResult.status === 'rejected') {
            throw new Error('兩種評估都失敗: ' + vowelResult.reason?.message + ' / ' + basicResult.reason?.message);
        }
        
        // 更新分析結果 UI - 顯示兩種評估結果
        updateAnalysisResultsUI(vowelAssessmentResult, basicAssessmentResult);

        // 儲存至練習紀錄：優先使用母音過濾結果，若失敗則退回基本比對結果。
        const resultToSave = vowelAssessmentResult || basicAssessmentResult;
        let savedToHistory = false;
        if (resultToSave) {
            try {
                // selectedStructure.id 可能是 selection.js 產生的合成 id
                // (格式為 'sel-<timestamp>')，代表整段/自訂選取範圍而非真實
                // structure，此時應送出 null，否則伺服器端 UUID 解析會直接失敗。
                const rawStructureId = state.selectedStructure?.id;
                const structureId = (rawStructureId && !rawStructureId.startsWith('sel-'))
                    ? rawStructureId
                    : null;
                await api('/assessments/submit', {
                    method: 'POST',
                    body: JSON.stringify({
                        song_id: state.currentSongId,
                        structure_id: structureId,
                        track_id: state.selectedTrackId,
                        score: resultToSave.score,
                        total_notes: resultToSave.total_notes,
                        matched_notes: resultToSave.matched_notes,
                        average_pitch_deviation: resultToSave.average_pitch_deviation,
                        average_duration_deviation: resultToSave.average_duration_deviation,
                        pitch_deviation: resultToSave.pitch_deviation,
                        duration_deviation: resultToSave.duration_deviation,
                        note_comparison: resultToSave.note_comparison
                    })
                });
                savedToHistory = true;
            } catch (err) {
                console.error('Failed to save assessment history:', err);
            }
        }

        // 更新狀態：明確告知使用者這次結果是否有被保留，避免誤以為已存入紀錄。
        document.getElementById('status-text').textContent = savedToHistory
            ? '分析完成，已加入練習紀錄'
            : '分析完成，但儲存紀錄失敗（本次結果不會保留）';

        return { vowelAssessmentResult, basicAssessmentResult };
    } catch (error) {
        console.error('分析錄音時發生錯誤:', error);
        document.getElementById('status-text').textContent = '分析失敗: ' + error.message;
        
        // 簡單的重試邏輯（最多重試 2 次）
        if (window.analysisRetryCount === undefined) {
            window.analysisRetryCount = 0;
        }
        
        if (window.analysisRetryCount < 2) {
            window.analysisRetryCount++;
            document.getElementById('status-text').textContent = `分析失敗，正在重試... (${window.analysisRetryCount}/2)`;
            // 短暫延遲後重試
            return new Promise(resolve => {
                setTimeout(() => {
                    resolve(analyzeAndSubmitRecording());
                }, 1000);
            });
        } else {
            // 重試次數用完，重置計數器
            window.analysisRetryCount = 0;
            throw error;
        }
    }
}

/**
 * 正規化單一評估結果
 * 處理 snake_case 和 PascalCase 兩種格式的 API 回傳
 */
function normalizeResult(result) {
    if (!result) return null;
    return {
        score: result.score ?? result.Score ?? 0,
        totalNotes: result.total_notes ?? result.TotalNotes ?? 0,
        matchedNotes: result.matched_notes ?? result.MatchedNotes ?? 0,
        avgPitchDev: result.average_pitch_deviation ?? result.AveragePitchDeviation ?? 0,
        avgDurationDev: result.average_duration_deviation ?? result.AverageDurationDeviation ?? 0,
        noteComparison: result.note_comparison ?? result.NoteComparison ?? []
    };
}

/**
 * 正規化音符比對資料
 */
function normalizeNoteComparison(noteComparison) {
    return noteComparison.map(note => {
        const matchStatus = note.match_status ?? note.MatchStatus ?? 'unknown';
        return {
            RefPitch: note.ref_pitch ?? note.RefPitch ?? 0,
            RefStart: note.ref_start ?? note.RefStart ?? 0,
            RefEnd: note.ref_end ?? note.RefEnd ?? 0,
            MatchStatus: typeof matchStatus === 'string' ? matchStatus.toLowerCase() : 'unknown',
            PitchDeviationCents: note.pitch_deviation_cents ?? note.PitchDeviationCents ?? 0,
            DurationDeviationSec: note.duration_deviation_sec ?? note.DurationDeviationSec ?? 0
        };
    });
}

/**
 * 將音高偏差與音符匹配數字翻譯成一般使用者能理解、能採取行動的一句話。
 * Cents 門檻與音高偏差圖例一致：<50 良好、50-100 尚可、>100 需改善。
 */
function buildPlainLanguageSummary(matchedNotes, totalNotes, avgPitchDev) {
    const absDev = Math.abs(avgPitchDev);
    const missedNotes = Math.max(0, totalNotes - matchedNotes);
    const missedNote = missedNotes > 0
        ? `；有 ${missedNotes}/${totalNotes} 個音符沒有比對成功，可以放慢速度再試一次`
        : '';

    if (absDev < 50) {
        return `🎉 音準掌握得很好（平均偏差 ${absDev.toFixed(0)} cents），繼續保持！${missedNote}`;
    }
    if (absDev < 100) {
        return `👍 音準大致抓到了（平均偏差 ${absDev.toFixed(0)} cents），還可以再更精準一些${missedNote}`;
    }
    return `💡 平均音高偏差 ${absDev.toFixed(0)} cents，建議放慢速度多練習這句${missedNote}`;
}

/**
 * 更新分析結果到 UI
 * 顯示兩種評估結果：母音過濾 vs 基本（無過濾）
 * @param {Object|null} vowelResult - 母音過濾評估結果
 * @param {Object|null} basicResult - 基本評估結果（無母音過濾）
 */
function updateAnalysisResultsUI(vowelResult, basicResult) {
    // 打印完整的 API 回應結果進行診斷
        // console.log('Updating analysis results UI with:', { vowelResult, basicResult });
    
    // 正規化兩種結果
    const vowelNorm = normalizeResult(vowelResult);
    const basicNorm = normalizeResult(basicResult);
    
    // 優先使用母音過濾結果作為主顯示
    const primary = vowelNorm || basicNorm;
    if (!primary) return;
    
    const matchedNotes = primary.matchedNotes;
    const totalNotes = primary.totalNotes;
    const avgPitchDev = primary.avgPitchDev;
    const avgDurationDev = primary.avgDurationDev;
    const score = primary.score;
    const normalizedNoteComparison = normalizeNoteComparison(primary.noteComparison);
    
        // console.log('Normalized values - Score:', score, 'Matched:', matchedNotes + '/' + totalNotes, 'NoteComparison length:', normalizedNoteComparison.length);
    
    // 更新圖表區域顯示分數儀表（僅分數，音高準度移至獨立折疊面板）
    const chartArea = document.getElementById('chart-area');
    chartArea.innerHTML = `
        <div class="viz-container">
            <div class="viz-plain-summary" style="margin-bottom:12px;font-size:14px;color:#ddd;">
                ${buildPlainLanguageSummary(matchedNotes, totalNotes, avgPitchDev)}
            </div>
            <div class="viz-score">
                <div class="viz-title">分數</div>
                <div id="score-gauge"></div>
            </div>
        </div>
    `;
    
    // 展開音高準度面板（位於分數面板之下，水平滾動）
    const pitchPanel = document.getElementById('pitch-panel');
    if (pitchPanel) {
        pitchPanel.classList.remove('hidden');
        pitchPanel.classList.add('visible');
        pitchPanel.setAttribute('data-folded', 'false');
        const toggleIcon = document.getElementById('pitch-panel-toggle-icon');
        if (toggleIcon) toggleIcon.textContent = '▲';
    }
    
    // 繪製分數儀表與音高偏差圖表
    import('./practice-ui.js').then(ui => {
        const scoreGaugeContainer = document.getElementById('score-gauge');
        if (scoreGaugeContainer) {
            ui.drawScoreGauge(score, scoreGaugeContainer);
        }
        const pitchChartContainer = document.getElementById('pitch-chart');
        if (pitchChartContainer && normalizedNoteComparison.length > 0) {
            ui.drawPitchDeviationChart(normalizedNoteComparison, pitchChartContainer);
        } else if (pitchChartContainer) {
            pitchChartContainer.innerHTML = '<div class="pitch-placeholder">無音符資料</div>';
        }
    });
    
    // 更新詳細結果面板 - 顯示兩種評估結果
    const resultsPanel = document.getElementById('results-panel-foldable');
    if (resultsPanel) {
        // 更新摘要數據
        const matchedEl = document.getElementById('result-matched');
        if (matchedEl) matchedEl.textContent = `${matchedNotes}/${totalNotes}`;
        const pitchEl = document.getElementById('result-pitch');
        if (pitchEl) pitchEl.textContent = avgPitchDev.toFixed(1);
        const durationEl = document.getElementById('result-duration');
        if (durationEl) durationEl.textContent = avgDurationDev.toFixed(3);
        
        // 展開面板
        resultsPanel.classList.remove('hidden');
        resultsPanel.classList.add('visible');
        resultsPanel.setAttribute('data-folded', 'false');
        const toggleIcon = document.getElementById('results-panel-toggle-icon');
        if (toggleIcon) toggleIcon.textContent = '▲';
        
        // 渲染評估卡片到內容區
        const body = document.getElementById('results-panel-body');
        if (body) {
            body.innerHTML = `
                <div class="assessment-comparison">
                    <h3 style="color:#fff;margin:0 0 12px 0;font-size:16px;display:flex;justify-content:space-between;align-items:center;">
                        練習評估結果
                        <button id="retry-practice-btn" class="btn btn-sm btn-success">🔁 再次練習這句</button>
                    </h3>
                    <div class="results-grid-two">
                        ${renderAssessmentCard(vowelNorm, '母音過濾', normalizedNoteComparison)}
                        ${renderAssessmentCard(basicNorm, '基本比對', basicNorm ? normalizeNoteComparison(basicNorm.noteComparison) : [])}
                    </div>
                </div>
            `;
            // 直接重新練習目前選取的段落，不需捲動回結構樹重新點擊。
            const retryBtn = document.getElementById('retry-practice-btn');
            if (retryBtn) retryBtn.onclick = () => startPractice();
        }
        
        // 繪製母音過濾版本的音符比對表格
        if (vowelNorm && vowelNorm.noteComparison.length > 0) {
            import('./practice-ui.js').then(ui => {
                const vowelTable = document.getElementById('note-table-vowel');
                if (vowelTable) {
                    ui.drawNoteComparisonTable(normalizedNoteComparison, vowelTable);
                }
                const basicTable = document.getElementById('note-table-basic');
                if (basicTable && basicNorm && basicNorm.noteComparison.length > 0) {
                    const basicNormalized = normalizeNoteComparison(basicNorm.noteComparison);
                    ui.drawNoteComparisonTable(basicNormalized, basicTable);
                }
            });
        } else if (basicNorm && basicNorm.noteComparison.length > 0) {
            import('./practice-ui.js').then(ui => {
                const basicTable = document.getElementById('note-table-basic');
                if (basicTable) {
                    const basicNormalized = normalizeNoteComparison(basicNorm.noteComparison);
                    ui.drawNoteComparisonTable(basicNormalized, basicTable);
                }
            });
        }
    }
}

/**
 * 渲染單張評估卡片
 */
function renderAssessmentCard(norm, label, normalizedNoteComparison) {
    const tableId = label === '母音過濾' ? 'vowel' : 'basic';

    if (!norm) {
        return `
            <div class="result-card">
                <div class="result-card-header">
                    <span class="result-card-title">${label}</span>
                    <span class="result-card-status failed">失敗</span>
                </div>
                <div class="result-card-body">
                    <div class="result-stat-value">錯誤</div>
                    <div class="result-stat-label">計算失敗</div>
                </div>
            </div>
        `;
    }
    
    return `
        <div class="result-card">
            <div class="result-card-header">
                <span class="result-card-title">${label}</span>
                <span class="result-card-status success">完成</span>
            </div>
            <div class="result-card-grid">
                <div class="result-stat">
                    <div class="result-stat-value">${norm.matchedNotes}/${norm.totalNotes}</div>
                    <div class="result-stat-label">音符匹配</div>
                </div>
                <div class="result-stat">
                    <div class="result-stat-value">${norm.avgPitchDev.toFixed(1)}</div>
                    <div class="result-stat-label">平均音高偏差</div>
                </div>
                <div class="result-stat">
                    <div class="result-stat-value">${norm.avgDurationDev.toFixed(3)}</div>
                    <div class="result-stat-label">平均時長偏差</div>
                </div>
                <div class="result-stat">
                    <div class="result-stat-value">${norm.score}</div>
                    <div class="result-stat-label">分數</div>
                </div>
            </div>
            <div class="result-card-table">
                <h4>音符比對（按樂句分組）</h4>
                <div id="note-table-${tableId}"></div>
            </div>
        </div>
    `;
}

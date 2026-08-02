// audio.js — 音訊播放模組
// 依賴：state.js（parsedNotes, parsedTrackIndex, selectedAccompanimentTrackIds, currentSongFull）

import state from './state.js';

// Audio Process 常量 - 用於識別不同的音訊行為
export const AudioProcess = {
  PRACTICE_PLAYBACK: 'practice_playback',      // 練習音檔播放（伴奏+節拍器）
  ACCOMPANIMENT: 'accompaniment',               // 單獨播放伴奏
  RECORDING_REPLAY: 'recording_replay'          // 使用者錄音回放
};

let audioCtx = null;
let playbackNodes = [];
let playbackTimer = null;
let isPlaying = false;
let replayAudio = null; // Track current replay audio

export function getAudioCtx() {
    if (!audioCtx) audioCtx = new (window.AudioContext || window.webkitAudioContext)();
    return audioCtx;
}

function midiPitchToFreq(pitch) {
    return 440 * Math.pow(2, (pitch - 69) / 12);
}

/**
 * Schedule metronome beat click sounds on the audio context timeline.
 * Creates short sine wave clicks (~1000Hz, ~30ms) at the specified times.
 * @param {AudioContext} ctx - The Web Audio context
 * @param {number} count - Number of beats to schedule
 * @param {number} intervalSec - Time between each beat in seconds
 * @param {number} startTime - Absolute time on the audio context for first beat
 */
export function scheduleBeats(ctx, count, intervalSec, startTime) {
    for (let i = 0; i < count; i++) {
        const beatTime = startTime + i * intervalSec;
        const osc = ctx.createOscillator();
        const gain = ctx.createGain();
        osc.connect(gain);
        gain.connect(ctx.destination);
        osc.type = 'sine';
        osc.frequency.setValueAtTime(1000, beatTime);
        gain.gain.setValueAtTime(0.8, beatTime);
        gain.gain.exponentialRampToValueAtTime(0.001, beatTime + 0.03);
        osc.start(beatTime);
        osc.stop(beatTime + 0.05);
    }
}

export function playRange(startTime, endTime) {
    const { parsedNotes, parsedTrackIndex, selectedAccompanimentTrackIds, currentSongFull } = state;
    if (!parsedNotes || parsedNotes.length === 0) return;
    stopPlayback();
    const ctx = getAudioCtx();
    const gainNode = ctx.createGain();
    gainNode.gain.value = 0.3;
    gainNode.connect(ctx.destination);
    const dur = endTime - startTime;
    const now = ctx.currentTime;

    // 只播放選取的音軌：主聲部 + 伴唱聲部
    const accompanimentTrackIndices = selectedAccompanimentTrackIds
        .map(id => {
            if (!currentSongFull || !currentSongFull.tracks) return -1;
            for (const t of currentSongFull.tracks) {
                if (t.id === id) return t.midi_index;
            }
            return -1;
        })
        .filter(idx => idx >= 0);
    const playableTrackIndices = [...accompanimentTrackIndices].filter(idx => idx >= 0);

    const rangeNotes = parsedNotes.filter(n =>
        n.start >= startTime && n.start < endTime &&
        playableTrackIndices.includes(n.track)
    );

    for (const n of rangeNotes) {
        const osc = ctx.createOscillator();
        const ng = ctx.createGain();
        osc.type = 'triangle';
        osc.frequency.value = midiPitchToFreq(n.pitch);
        const ls = n.start - startTime;
        const nd = Math.min(n.dur, dur - ls);
        if (nd <= 0.01) continue;
        ng.gain.setValueAtTime(0, now + ls);
        ng.gain.linearRampToValueAtTime(0.3, now + ls + 0.005);
        ng.gain.setValueAtTime(0.3, now + ls + nd - 0.01);
        ng.gain.linearRampToValueAtTime(0, now + ls + nd);
        osc.connect(ng); ng.connect(gainNode);
        osc.start(now + ls); osc.stop(now + ls + nd + 0.01);
        playbackNodes.push(osc, ng);
    }
    playbackTimer = setTimeout(() => { isPlaying = false; updatePlayBtn(); stopPlayback(); }, dur * 1000 + 300);
    isPlaying = true;
    updatePlayBtn();
}

/**
 * Play a range of notes with a scheduled start delay.
 * The accompaniment begins at clickTime + startDelay, with notes
 * scheduled relative to that time.
 * @param {number} startTime - Range start in seconds (section start)
 * @param {number} endTime - Range end in seconds
 * @param {number} startDelay - Delay in seconds before playback begins
 */
export function playRangeDelayed(startTime, endTime, startDelay) {
    const { parsedNotes, parsedTrackIndex, selectedAccompanimentTrackIds, currentSongFull } = state;
    if (!parsedNotes || parsedNotes.length === 0) return;
    stopPlayback();
    const ctx = getAudioCtx();
    const gainNode = ctx.createGain();
    gainNode.gain.value = 0.3;
    gainNode.connect(ctx.destination);
    const dur = endTime - startTime;
    const now = ctx.currentTime;
    const startAt = now + startDelay;

    const accompanimentTrackIndices = selectedAccompanimentTrackIds
        .map(id => {
            if (!currentSongFull || !currentSongFull.tracks) return -1;
            for (const t of currentSongFull.tracks) {
                if (t.id === id) return t.midi_index;
            }
            return -1;
        })
        .filter(idx => idx >= 0);
    const playableTrackIndices = [...accompanimentTrackIndices].filter(idx => idx >= 0);

    const rangeNotes = parsedNotes.filter(n =>
        n.start >= startTime && n.start < endTime &&
        playableTrackIndices.includes(n.track)
    );

    for (const n of rangeNotes) {
        const osc = ctx.createOscillator();
        const ng = ctx.createGain();
        osc.type = 'triangle';
        osc.frequency.value = midiPitchToFreq(n.pitch);
        const ls = n.start - startTime;
        const nd = Math.min(n.dur, dur - ls);
        if (nd <= 0.01) continue;
        ng.gain.setValueAtTime(0, startAt + ls);
        ng.gain.linearRampToValueAtTime(0.3, startAt + ls + 0.005);
        ng.gain.setValueAtTime(0.3, startAt + ls + nd - 0.01);
        ng.gain.linearRampToValueAtTime(0, startAt + ls + nd);
        osc.connect(ng); ng.connect(gainNode);
        osc.start(startAt + ls); osc.stop(startAt + ls + nd + 0.01);
        playbackNodes.push(osc, ng);
    }
    playbackTimer = setTimeout(() => { isPlaying = false; updatePlayBtn(); stopPlayback(); }, dur * 1000 + startDelay * 1000 + 300);
    isPlaying = true;
    updatePlayBtn();
}

export function stopPlayback() {
    if (playbackTimer) { clearTimeout(playbackTimer); playbackTimer = null; }
    for (const n of playbackNodes) { try { n.disconnect(); } catch(e) {} }
    playbackNodes = [];
    isPlaying = false;
    updatePlayBtn();
}

export function stopReplay() {
    if (replayAudio) {
        replayAudio.pause();
        replayAudio.currentTime = 0;
        replayAudio = null;
    }
}

/**
 * 統一停止音訊 process
 * @param {string} processId - AudioProcess 常量
 */
export function stopAudioProcess(processId) {
    switch(processId) {
        case AudioProcess.PRACTICE_PLAYBACK:
            stopPlayback();
            break;
        case AudioProcess.RECORDING_REPLAY:
            stopReplay();
            break;
        case AudioProcess.ACCOMPANIMENT:
            stopPlayback();
            break;
        default:
            // 如果沒有匹配的 process，停止所有音訊
            stopPlayback();
            stopReplay();
    }
}

export function getIsReplaying() {
    return replayAudio !== null;
}

export function togglePlayback() {
    const { selectedStructure } = state;
    if (isPlaying) {
        stopPlayback();
        // 觸發中斷回呼（練習中斷時丟棄錄音）
        triggerInterrupt();
        return;
    }
    if (selectedStructure) {
        playRange(selectedStructure.start, selectedStructure.end);
    } else {
        alert('請先選擇一個段落或句子');
    }
}

export function getIsPlaying() {
    return isPlaying;
}

// updatePlayBtn 需要從外部注入（因為涉及 DOM）
let updatePlayBtn = () => {};

export function setUpdatePlayBtn(fn) {
    updatePlayBtn = fn;
}

// 中斷回呼：當使用者手動停止播放時觸發（用於中斷練習）
let onInterruptCallback = null;

export function setOnInterruptCallback(fn) {
    onInterruptCallback = fn;
}

export function triggerInterrupt() {
    if (onInterruptCallback) onInterruptCallback();
}

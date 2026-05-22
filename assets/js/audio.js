// audio.js — 音訊播放模組
// 依賴：state.js（parsedNotes, parsedTrackIndex, selectedAccompanimentTrackIds, currentSongFull）

import state from './state.js';

let audioCtx = null;
let playbackNodes = [];
let playbackTimer = null;
let isPlaying = false;

function getAudioCtx() {
    if (!audioCtx) audioCtx = new (window.AudioContext || window.webkitAudioContext)();
    return audioCtx;
}

function midiPitchToFreq(pitch) {
    return 440 * Math.pow(2, (pitch - 69) / 12);
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
    const playableTrackIndices = [parsedTrackIndex, ...accompanimentTrackIndices].filter(idx => idx >= 0);

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

export function stopPlayback() {
    if (playbackTimer) { clearTimeout(playbackTimer); playbackTimer = null; }
    for (const n of playbackNodes) { try { n.disconnect(); } catch(e) {} }
    playbackNodes = [];
    isPlaying = false;
    updatePlayBtn();
}

export function togglePlayback() {
    const { selectedStructure } = state;
    if (isPlaying) { stopPlayback(); return; }
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
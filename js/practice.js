// === Practice Module ===
import { state } from './state.js';
import { api } from './api.js';
import { findStructureTitle, escapeHtml } from './utils.js';
import { showScreen } from './navigation.js';
import { checkAudioSetup } from './device.js';
import { loadLyrics, renderLyrics, startLyricsSync, stopLyricsSync } from './lyrics.js';

export function calculateBPM(notes, defaultBPM = 120) {
    if (!notes || notes.length < 2) return defaultBPM;

    const sorted = [...notes].sort((a, b) => (a.start_time ?? a.startTime) - (b.start_time ?? b.startTime));
    
    // Get the first few note onsets
    const onsets = sorted.map(n => n.start_time ?? n.startTime);
    
    // Calculate inter-onset intervals for first 8 notes (or fewer)
    const intervals = [];
    for (let i = 1; i < Math.min(onsets.length, 9); i++) {
        const interval = onsets[i] - onsets[i - 1];
        if (interval > 0.1 && interval < 3) { // reasonable note interval
            intervals.push(interval);
        }
    }

    if (intervals.length === 0) return defaultBPM;

    // Use median interval
    intervals.sort((a, b) => a - b);
    const mid = Math.floor(intervals.length / 2);
    const medianInterval = intervals.length % 2 === 0 
        ? (intervals[mid - 1] + intervals[mid]) / 2 
        : intervals[mid];

    // BPM = 60 / interval_in_seconds * beat_unit
    // Assuming quarter note = one interval
    const bpm = Math.round(60 / medianInterval);
    
    // Sanity check
    if (bpm < 30 || bpm > 300) return defaultBPM;
    return bpm;
}

export async function countIn(bpm, callback) {
    const countDownEl = document.getElementById('count-down');
    if (!countDownEl) {
        callback();
        return;
    }

    state.isCountIn = true;
    countDownEl.classList.remove('hidden');

    const audioCtx = state.audioCtx || new (window.AudioContext || window.webkitAudioContext)();
    state.audioCtx = audioCtx;

    const beatDuration = 60 / bpm;
    const startTime = audioCtx.currentTime + 0.1;

    for (let i = 0; i < 3; i++) {
        const beatTime = startTime + i * beatDuration;
        
        // Visual count
        setTimeout(() => {
            countDownEl.textContent = `${3 - i}`;
            countDownEl.style.color = '#3498db';
            countDownEl.style.fontSize = '72px';
        }, (i * beatDuration) * 1000);

        // Metronome click (short sine burst)
        const osc = audioCtx.createOscillator();
        const gain = audioCtx.createGain();
        osc.connect(gain);
        gain.connect(audioCtx.destination);
        
        osc.type = 'triangle';
        osc.frequency.value = i === 0 ? 880 : 660; // Higher pitch on first beat
        
        const clickDuration = 0.05;
        gain.gain.setValueAtTime(0.3, beatTime);
        gain.gain.exponentialRampToValueAtTime(0.01, beatTime + clickDuration);
        
        osc.start(beatTime);
        osc.stop(beatTime + clickDuration);
    }

    // Wait for count-in to finish
    const totalDuration = 3 * beatDuration;
    setTimeout(() => {
        countDownEl.classList.add('hidden');
        state.isCountIn = false;
        callback();
    }, totalDuration * 1000 + 100);
}

export function playAccompaniment(rangeStart, rangeEnd) {
    if (!state.accompanimentNotes || state.accompanimentNotes.length === 0) {
        console.log('No accompaniment notes to play');
        return;
    }

    const audioCtx = state.audioCtx || new (window.AudioContext || window.webkitAudioContext)();
    state.audioCtx = audioCtx;

    // Resume context if suspended (autoplay policy)
    if (audioCtx.state === 'suspended') {
        audioCtx.resume();
    }

    // Filter notes within range
    const startSec = rangeStart ?? 0;
    const endSec = rangeEnd ?? Infinity;

    const filteredNotes = state.accompanimentNotes.filter(n => {
        const s = n.start_time ?? n.startTime;
        const e = n.end_time ?? n.endTime;
        return s >= startSec && s <= endSec;
    });

    if (filteredNotes.length === 0) return;

    // Create master gain
    const masterGain = audioCtx.createGain();
    masterGain.gain.value = 0.3; // Master volume
    masterGain.connect(audioCtx.destination);
    state.accompanimentGain = masterGain;

    // Sort notes by start time
    const sortedNotes = [...filteredNotes].sort((a, b) => (a.start_time ?? a.startTime) - (b.start_time ?? b.startTime));

    const now = audioCtx.currentTime;
    const playbackOffset = 0; // Start from the beginning of range

    state.currentScheduledNotes = [];
    state.playbackStartTime = now;

    for (const note of sortedNotes) {
        const noteStart = (note.start_time ?? note.startTime) - startSec;
        const noteEnd = (note.end_time ?? note.endTime) - startSec;
        const duration = noteEnd - noteStart;

        if (duration <= 0) continue;

        const pitch = note.pitch;
        const freq = 440 * Math.pow(2, (pitch - 69) / 12);

        // Create oscillator with ADSR envelope
        const osc = audioCtx.createOscillator();
        const noteGain = audioCtx.createGain();
        const filterNode = audioCtx.createBiquadFilter();

        osc.type = 'triangle'; // Triangle wave for softer sound
        osc.frequency.value = freq;

        // Simple filter for warmer tone
        filterNode.type = 'lowpass';
        filterNode.frequency.value = freq * 4;
        filterNode.Q.value = 1;

        // ADSR envelope
        const attackTime = 0.02;
        const releaseTime = 0.1;
        const sustainLevel = 0.7;

        const scheduleTime = now + noteStart;
        
        noteGain.gain.setValueAtTime(0, scheduleTime);
        noteGain.gain.linearRampToValueAtTime(1, scheduleTime + attackTime);
        noteGain.gain.setValueAtTime(sustainLevel, scheduleTime + attackTime + 0.01);
        
        const releaseStart = scheduleTime + Math.max(duration - releaseTime, 0);
        noteGain.gain.setValueAtTime(sustainLevel, releaseStart);
        noteGain.gain.exponentialRampToValueAtTime(0.001, scheduleTime + duration);

        osc.connect(filterNode);
        filterNode.connect(noteGain);
        noteGain.connect(masterGain);

        osc.start(scheduleTime);
        osc.stop(scheduleTime + duration + 0.05);

        state.currentScheduledNotes.push({
            osc,
            noteGain,
            filterNode,
            startTime: scheduleTime,
            endTime: scheduleTime + duration
        });
    }

    // Update progress bar
    const totalDuration = sortedNotes.length > 0 
        ? ((sortedNotes[sortedNotes.length - 1].end_time ?? sortedNotes[sortedNotes.length - 1].endTime) - startSec)
        : 30;

    // Animate progress
    const progressEl = document.getElementById('accompaniment-progress');
    const progressContainer = document.getElementById('accompaniment-progress-container');
    if (progressContainer) progressContainer.style.display = 'block';
    
    state.progressInterval = setInterval(() => {
        const elapsed = audioCtx.currentTime - state.playbackStartTime;
        const progress = Math.min((elapsed / totalDuration) * 100, 100);
        if (progressEl) progressEl.style.width = progress + '%';
        
        if (elapsed >= totalDuration) {
            clearInterval(state.progressInterval);
            if (progressEl) progressEl.style.width = '100%';
        }
    }, 100);
}

export function stopAccompaniment() {
    // Stop oscillators
    if (state.currentScheduledNotes) {
        for (const note of state.currentScheduledNotes) {
            try {
                note.osc.stop();
                note.osc.disconnect();
            } catch (e) {
                // Already stopped
            }
        }
        state.currentScheduledNotes = [];
    }

    // Clear progress interval
    if (state.progressInterval) {
        clearInterval(state.progressInterval);
        state.progressInterval = null;
    }

    // Hide progress
    const progressContainer = document.getElementById('accompaniment-progress-container');
    if (progressContainer) progressContainer.style.display = 'none';

    // Stop lyrics sync
    stopLyricsSync();

    // Don't close audio context, it can be reused
}

export async function startPractice() {
    showScreen('practice-screen');
    document.getElementById('analysis-status').textContent = '準備錄音...';

    const song = state.selectedSong;
    if (!song) return;

    // Load notes for vocal and accompaniment tracks
    try {
        const fullSong = await api(`/songs/${song.id}`);
        
        // Vocal reference notes
        const vocalTrack = fullSong.tracks.find(t => t.id === state.selectedVocalTrack);
        if (vocalTrack) {
            state.referenceNotes = vocalTrack.notes || [];
        }

        // Accompaniment notes
        state.accompanimentNotes = [];
        for (const accId of state.selectedAccompanimentTracks) {
            const accTrack = fullSong.tracks.find(t => t.id === accId);
            if (accTrack && accTrack.notes) {
                state.accompanimentNotes = state.accompanimentNotes.concat(accTrack.notes);
            }
        }
    } catch (err) {
        console.error('Failed to load track notes:', err);
    }

    if (state.referenceNotes.length === 0) {
        document.getElementById('analysis-status').textContent = '無法取得參考音符資料';
    }

    // Update structures with lyrics for selected vocal track
    await loadLyrics();
    renderLyrics();

    // Check audio setup
    await checkAudioSetup();

    // Setup recording UI
    setupPracticeUI();
}

function setupPracticeUI() {
    // Show headset banner
    const headsetBannerEl = document.getElementById('headset-banner');
    if (headsetBannerEl) {
        headsetBannerEl.style.display = 'block';
    }

    // Show range info if selected
    const practiceRangeEl = document.getElementById('practice-range-info');
    if (practiceRangeEl && state.rangeStartId && state.rangeEndId) {
        const startTitle = findStructureTitle(state.rangeStartId, state.structures);
        const endTitle = findStructureTitle(state.rangeEndId, state.structures);
        practiceRangeEl.innerHTML = `
            <span>📌 練習範圍：<strong>${escapeHtml(startTitle)}</strong> → <strong>${escapeHtml(endTitle)}</strong></span>
        `;
        practiceRangeEl.style.display = 'block';
    } else if (practiceRangeEl) {
        practiceRangeEl.style.display = 'none';
    }

    // Show accompaniment info
    const accInfoEl = document.getElementById('accompaniment-info');
    if (accInfoEl && state.selectedAccompanimentTracks.length > 0) {
        // Get track names
        const song = state.selectedSong;
        const trackNames = state.selectedAccompanimentTracks.map(id => {
            const t = song.tracks.find(t => t.id === id);
            return t ? t.name : id;
        });
        accInfoEl.innerHTML = `🎵 伴奏：${escapeHtml(trackNames.join(', '))}`;
        accInfoEl.style.display = 'block';
    } else if (accInfoEl) {
        accInfoEl.style.display = 'none';
    }

    // Reset recording UI
    const statusEl = document.getElementById('recording-status');
    if (statusEl) {
        statusEl.textContent = '準備錄音';
        statusEl.style.color = '#fff';
    }

    const btn = document.getElementById('record-btn');
    if (btn) {
        btn.disabled = false;
        btn.textContent = '開始錄音';
        btn.classList.remove('recording');
    }

    document.getElementById('timer').textContent = '00:00';

    // Show count down area
    const countDownEl = document.getElementById('count-down');
    if (countDownEl) {
        countDownEl.classList.add('hidden');
    }

    // Reset progress
    const progressEl = document.getElementById('accompaniment-progress');
    if (progressEl) {
        progressEl.style.width = '0%';
    }

    document.getElementById('results-panel').classList.remove('visible');
    document.getElementById('visualizations').classList.add('hidden');
    document.getElementById('score-display').classList.add('hidden');
    document.getElementById('submit-btn').classList.add('hidden');
}

export function goBackToTracks() {
    stopAccompaniment();
    state.selectedVocalTrack = null;
    state.selectedAccompanimentTracks = [];
    state.selectedStructure = null;
    state.referenceNotes = [];
    state.accompanimentNotes = [];
    state.detectedNotes = [];
    state.assessmentResult = null;
    state.rangeStartId = null;
    state.rangeEndId = null;
    state.selectedRangeStart = null;
    state.selectedRangeEnd = null;
    state.lyricsLines = [];
    stopLyricsSync();
    showScreen('tracks-screen');
}
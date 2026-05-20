// === Recording Module ===
import { state } from './state.js';
import { detectPitch, mergeSamePitchNotes, compareMergedNotes } from './pitch.js';
import { calculateBPM, countIn, playAccompaniment, stopAccompaniment } from './practice.js';
import { startLyricsSync, stopLyricsSync } from './lyrics.js';
import { displayResults } from './results.js';

export async function toggleRecording() {
    if (state.isRecording) {
        stopRecording();
    } else {
        await startRecording();
    }
}

async function startRecording() {
    // Check if count-in is in progress
    if (state.isCountIn) {
        return;
    }

    // Get headset device preference
    let audioConstraints = { audio: true };
    if (state.preferredMicId) {
        audioConstraints = {
            audio: {
                deviceId: { exact: state.preferredMicId }
            }
        };
    }

    try {
        const stream = await navigator.mediaDevices.getUserMedia(audioConstraints);
        state.mediaRecorder = new MediaRecorder(stream);
        state.audioChunks = [];

        state.mediaRecorder.ondataavailable = (event) => {
            if (event.data.size > 0) {
                state.audioChunks.push(event.data);
            }
        };

        state.mediaRecorder.onstop = () => {
            const audioBlob = new Blob(state.audioChunks, { type: 'audio/webm' });
            stopAccompaniment();
            processAudio(audioBlob);
            stream.getTracks().forEach(track => track.stop());
        };

        // Calculate BPM from vocal reference notes
        const bpm = calculateBPM(state.referenceNotes, 120);
        
        // Update status for count-in
        const statusEl = document.getElementById('recording-status');
        if (statusEl) {
            statusEl.textContent = `倒數 ${bpm} BPM...`;
            statusEl.style.color = '#f39c12';
        }

        const btn = document.getElementById('record-btn');
        if (btn) {
            btn.disabled = true;
        }

        // Get range start time for accompaniment offset
        let rangeStart = 0;
        let rangeEnd = null;
        if (state.rangeStartId && state.rangeEndId) {
            rangeStart = state.selectedRangeStart?.startTime ?? 0;
            rangeEnd = state.selectedRangeEnd?.endTime ?? null;
        }

        // Do count-in, then start recording + accompaniment
        await countIn(bpm, () => {
            // Start recording
            state.mediaRecorder.start();
            state.isRecording = true;
            
            if (statusEl) {
                statusEl.textContent = '錄音中...';
                statusEl.style.color = '#e74c3c';
                statusEl.classList.add('recording');
            }
            
            if (btn) {
                btn.textContent = '停止錄音';
                btn.disabled = false;
            }

            // Start playing accompaniment
            playAccompaniment(rangeStart, rangeEnd);

            // Start lyrics sync
            startLyricsSync();

            // Update timer
            let seconds = 0;
            state.recordingTimer = setInterval(() => {
                seconds++;
                const mins = Math.floor(seconds / 60).toString().padStart(2, '0');
                const secs = (seconds % 60).toString().padStart(2, '0');
                document.getElementById('timer').textContent = `${mins}:${secs}`;
            }, 1000);
        });
    } catch (err) {
        alert('無法存取麥克風: ' + err.message);
        const btn = document.getElementById('record-btn');
        if (btn) {
            btn.disabled = false;
            btn.textContent = '開始錄音';
        }
    }
}

function stopRecording() {
    if (state.recordingTimer) {
        clearInterval(state.recordingTimer);
        state.recordingTimer = null;
    }

    stopLyricsSync();
    if (state.mediaRecorder && state.mediaRecorder.state !== 'inactive') {
        state.mediaRecorder.stop();
    }
    state.isRecording = false;

    const statusEl = document.getElementById('recording-status');
    if (statusEl) {
        statusEl.textContent = '處理中...';
        statusEl.style.color = '#f39c12';
        statusEl.classList.remove('recording');
    }

    const btn = document.getElementById('record-btn');
    if (btn) {
        btn.disabled = true;
        btn.textContent = '分析中...';
    }
}

async function processAudio(audioBlob) {
    document.getElementById('analysis-status').textContent = '正在分析音頻...';

    try {
        const arrayBuffer = await audioBlob.arrayBuffer();
        const audioContext = new (window.AudioContext || window.webkitAudioContext)();
        const audioBuffer = await audioContext.decodeAudioData(arrayBuffer);

        // Filter reference notes to selected range if any
        let refNotes = state.referenceNotes;
        if (state.rangeStartId && state.rangeEndId) {
            const startSec = state.selectedRangeStart?.startTime ?? 0;
            const endSec = state.selectedRangeEnd?.endTime ?? Infinity;
            refNotes = refNotes.filter(n => {
                const s = n.start_time ?? n.startTime;
                return s >= startSec && s <= endSec;
            });
        }

        // Pitch detection
        const detected = detectPitch(audioBuffer);
        state.detectedNotes = detected;

        // Merge same-pitch reference notes, then compare merged vs detected
        const mergedRef = mergeSamePitchNotes(refNotes);
        const result = compareMergedNotes(refNotes, mergedRef, detected);
        state.assessmentResult = result;

        // Display results
        displayResults(result);

        document.getElementById('analysis-status').textContent = '分析完成';
    } catch (err) {
        console.error('Analysis error:', err);
        document.getElementById('analysis-status').textContent = '分析失敗: ' + err.message;
        const btn = document.getElementById('record-btn');
        if (btn) btn.disabled = false;
    }
}
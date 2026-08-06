// practice-business.js — 純業務邏輯
// 依賴：state.js, api.js, audio.js, songDataExtractor.js (global)
// 不直接操作 DOM（UI 操作委託給 practice-ui.js）

import state from './state.js';
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
    
    // Auto-select first structure if available
    if (state.flatItems.length > 0) {
        const firstId = state.flatItems[0].data.id;
        state.selectionRange = { from: 0, to: 0 };
        computeSelectedStructure();
        generateReferenceData();
        const { renderStructureList: reRender, renderLyricsPanel: reRenderLyrics } = await import('./practice-ui.js');
        reRender();
        reRenderLyrics();
        // Update range info display
        const { selectedStructure } = state;
        if (selectedStructure) {
            const formatTime = (sec) => {
                if (sec == null) return '0:00';
                const m = Math.floor(sec / 60);
                const s = Math.floor(sec % 60);
                return m + ':' + s.toString().padStart(2, '0');
            };
            const rangeInfo = document.getElementById('range-info');
            if (rangeInfo) {
                rangeInfo.textContent = `🎯 ${selectedStructure.title}：${formatTime(selectedStructure.start)} - ${formatTime(selectedStructure.end)}`;
            }
            const rangeActions = document.getElementById('range-actions');
            if (rangeActions) {
                rangeActions.style.display = 'flex';
            }
            const currentRangeLabel = document.getElementById('current-range-label');
            if (currentRangeLabel) {
                currentRangeLabel.textContent = `${selectedStructure.title} (${formatTime(selectedStructure.start)})`;
            }
        }
    }
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
    // 隱藏結果面板
    const resultsPanel = document.getElementById('results-panel');
    if (resultsPanel) {
        resultsPanel.classList.remove('visible');
    }
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
        // 隱藏結果面板
        const resultsPanel = document.getElementById('results-panel');
        if (resultsPanel) {
            resultsPanel.classList.remove('visible');
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

function startMediaRecorder() {
    // Clean up any previous recording
    stopMediaRecorder();

    if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
        console.warn('getUserMedia is not supported in this environment');
        return;
    }

    if (!window.isSecureContext) {
        console.warn('Not a secure context — getUserMedia may be blocked. Use localhost or HTTPS.');
    }

    // Reset recording state
    audioBuffer = [];
    recordedWavBlob = null;

    const constraints = {
        audio: {
            sampleRate: 48000, // Use a standard sample rate
            channelCount: 1,
            echoCancellation: false,
            noiseSuppression: false,
            autoGainControl: false
        }
    };

    navigator.mediaDevices.getUserMedia(constraints)
        .then(stream => {
            // Store media stream so we can stop it later
            mediaStream = stream;
            
            // Check if we actually got audio tracks
            const audioTracks = stream.getAudioTracks();
            if (audioTracks.length === 0) {
                throw new Error('No audio input device found');
            }
            console.log(`Microphone access granted, using device: ${audioTracks[0].label || 'Unknown'}`);
            
            // Use the shared AudioContext from audio.js
            audioContext = getAudioCtx();
            
            // Resume audio context if suspended (required for autoplay policies)
            if (audioContext.state === 'suspended') {
                audioContext.resume().catch(err => console.error('Failed to resume audio context:', err));
            }
            
            // Create media stream source
            audioInput = audioContext.createMediaStreamSource(stream);
            
            const bufferSize = 4096;
            scriptProcessorNode = audioContext.createScriptProcessor(bufferSize, 1, 1);
            
            // Process audio data
            scriptProcessorNode.onaudioprocess = function(e) {
                if (!isPracticeActive) return; // Stop processing if not actively recording
                
                const input = e.inputBuffer.getChannelData(0); // Mono - use channel 0
                // Log audio levels periodically to see if we're getting sound
                if (audioBuffer.length % 40960 === 0) { // Every ~0.85 seconds at 48kHz
                    const recentSamples = input.slice(Math.max(0, input.length - 10));
                    const hasNonZero = recentSamples.some(sample => Math.abs(sample) > 0.001);
                    const avgLevel = recentSamples.reduce((sum, sample) => sum + Math.abs(sample), 0) / recentSamples.length;
                    console.log(`Audio buffer: ${audioBuffer.length} samples, recent non-zero: ${hasNonZero}, avg level: ${avgLevel.toFixed(4)}, recent samples: [${recentSamples.map(x => x.toFixed(3)).join(', ')}]`);
                }
                // Log the first buffer to see if we are getting data
                if (audioBuffer.length === 0) {
                    console.log('First audio buffer received:', input.length, 'samples');
                    console.log('First 10 samples:', input.slice(0, 10));
                    // Also check min/max in first buffer
                    const minVal = Math.min(...input);
                    const maxVal = Math.max(...input);
                    console.log('First buffer range: [', minVal.toFixed(4), ',', maxVal.toFixed(4), ']');
                }
                // Copy samples to our buffer
                for (let i = 0; i < input.length; i++) {
                    audioBuffer.push(input[i]);
                }
            };
            
            // Connect the audio graph
            audioInput.connect(scriptProcessorNode);
            scriptProcessorNode.connect(audioContext.destination);
            
            // Start processing (implicitly starts when connected)
            console.log('Audio recording started successfully');
        })
        .catch(err => {
            console.error('Failed to start recording:', err);
            alert('無法存取麥克風。請確認已允許麥克風權限，並使用 localhost 或 HTTPS 連線。\n錯誤：' + err.message);
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
            mediaStream.getTracks().forEach(track => {
                console.log(`Stopping media track: ${track.kind}`);
                track.stop();
            });
            mediaStream = null;
        }
        
        // Encode WAV blob if we have audio data
         if (audioBuffer.length > 0) {
            // Use AudioContext sample rate if available, otherwise default to 44100
            const sampleRate = audioContext ? audioContext.sampleRate : 44100;
            console.log('Encoding WAV: sample rate', sampleRate, 'buffer length', audioBuffer.length);
            
            // Trim count-in (3-beat lead-in) from the beginning of the recording
            let bufferToEncode = audioBuffer;
            if (state.recordingTrimOffset > 0) {
                const trimSamples = Math.floor(state.recordingTrimOffset * sampleRate);
                if (trimSamples > 0 && trimSamples < audioBuffer.length) {
                    bufferToEncode = audioBuffer.slice(trimSamples);
                    console.log('Trimmed count-in:', trimSamples, 'samples (' + state.recordingTrimOffset.toFixed(3) + 's), remaining:', bufferToEncode.length, 'samples');
                }
            }
            
            // Analyze audio content
            const nonZeroCount = bufferToEncode.reduce((count, sample) => Math.abs(sample) > 0.001 ? count + 1 : count, 0);
            const percentNonZero = (nonZeroCount / bufferToEncode.length) * 100;
            
            // Calculate RMS (root mean square) as a measure of loudness
            const sumSquares = bufferToEncode.reduce((sum, sample) => sum + (sample * sample), 0);
            const rms = Math.sqrt(sumSquares / bufferToEncode.length);
            
            // Find peak amplitude (use reduce instead of Math.max with spread to avoid call stack limit)
            const peak = bufferToEncode.reduce((max, sample) => Math.abs(sample) > max ? Math.abs(sample) : max, 0);
            
            console.log(`Audio analysis: ${nonZeroCount}/${bufferToEncode.length} non-zero samples (${percentNonZero.toFixed(2)}%), RMS: ${rms.toFixed(6)}, Peak: ${peak.toFixed(6)}`);
            
            if (percentNonZero < 0.1) {
                console.warn('Warning: Audio appears to be mostly silence - check microphone permissions and input');
            }
            
            recordedWavBlob = encodeWAV(new Float32Array(bufferToEncode), sampleRate);
            console.log('Encoded WAV blob size:', recordedWavBlob.size, 'bytes');
            audioBuffer = []; // Clear buffer after encoding
        } else {
            console.log('No audio data to encode');
        }
        
        resolve();
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
            console.log('No recorded WAV blob available');
            resolve(null);
            return;
        }
        
        console.log('Converting WAV blob to base64, size:', recordedWavBlob.size, 'bytes');
        
        const reader = new FileReader();
        reader.onloadend = () => {
            // Remove the data:audio/wav;base64, prefix if present
            const base64String = (reader.result || '').toString().split(',')[1] || '';
            console.log('Base64 conversion complete, length:', base64String.length, 'characters');
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
    // 隱藏之前的結果面板
    const resultsPanel = document.getElementById('results-panel');
    if (resultsPanel) {
        resultsPanel.classList.remove('visible');
    }

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
     // Resume audio context if suspended
     const audioCtx = getAudioCtx();
     if (audioCtx.state === 'suspended') {
         audioCtx.resume().catch(err => console.error('Failed to resume audio context:', err));
     }
     
     // 存儲錄音時間資訊，用於後端音符比對的時間對齉
     state.recordingTrimOffset = recordingTrimOffset;
     state.recordingStartTime = selectedStructure.start + firstNoteTime;
     isPracticeActive = true;
    practiceEndTimer = setTimeout(() => {
        cleanupPractice(false).then(() => { // 保留錄音，等待 MediaRecorder onstop 完成
            // 分析並提交錄音
            analyzeAndSubmitRecording().then(result => {
                // 分析完成後顯示回放按鈕
                showReplayButton();
                document.getElementById('status-text').textContent = '分析完成';
                // 顯示覆蓋提示訊息
                if (window.updateWarningVisibility) {
                    window.updateWarningVisibility();
                }
            }).catch(error => {
                // 分析失敗時仍顯示回放按鈕並顯示錯誤
                showReplayButton();
                document.getElementById('status-text').textContent = '分析失敗: ' + error.message;
                // 顯示覆蓋提示訊息
                if (window.updateWarningVisibility) {
                    window.updateWarningVisibility();
                }
            });
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
    // 隱藏結果面板
    const resultsPanel = document.getElementById('results-panel');
    if (resultsPanel) {
        resultsPanel.classList.remove('visible');
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
        
        // 取得音訊樣本率 (從 AudioContext 取得)
        const audioContext = getAudioCtx();
        const sampleRate = audioContext ? audioContext.sampleRate : 44100;
        
        // Log what we're about to send to the API
        console.log('Sending to API - audio length:', audioBase64.length, 'base64 chars, reference notes:', referenceNotes.length);
        console.log('First reference note sample:', referenceNotes[0]);
        console.log('Recording start time offset:', state.recordingStartTime);
        console.log('Recording trim offset:', state.recordingTrimOffset);
        
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
        console.log('Analysis API (vowel-filtered) response:', vowelResult);
        console.log('Analysis API (basic) response:', basicResult);
        
        // 擷取結果（如果請求失敗則為 null）
        const vowelAssessmentResult = vowelResult.status === 'fulfilled' ? vowelResult.value : null;
        const basicAssessmentResult = basicResult.status === 'fulfilled' ? basicResult.value : null;
        
        if (vowelResult.status === 'rejected' && basicResult.status === 'rejected') {
            throw new Error('兩種評估都失敗: ' + vowelResult.reason?.message + ' / ' + basicResult.reason?.message);
        }
        
        // 更新分析結果 UI - 顯示兩種評估結果
        updateAnalysisResultsUI(vowelAssessmentResult, basicAssessmentResult);
        
        // 可選：自動提交評估結果到歷史記錄
        // 註解掉此部分如果只想顯示分析而不儲存
        /*
        await api('/assessments/submit', {
            method: 'POST',
            body: JSON.stringify({
                song_id: state.currentSongId,
                structure_id: state.selectedStructure?.id || null,
                track_id: state.selectedTrackId,
                score: analysisResult.Score,
                total_notes: analysisResult.TotalNotes,
                matched_notes: analysisResult.MatchedNotes,
                average_pitch_deviation: analysisResult.AveragePitchDeviation,
                average_duration_deviation: analysisResult.AverageDurationDeviation,
                pitch_deviation: analysisResult.PitchDeviation,
                duration_deviation: analysisResult.DurationDeviation,
                note_comparison: analysisResult.NoteComparison
            })
        });
        */
        
        // 更新狀態
        document.getElementById('status-text').textContent = '分析完成';
        
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
 * 更新分析結果到 UI
 * 顯示兩種評估結果：母音過濾 vs 基本（無過濾）
 * @param {Object|null} vowelResult - 母音過濾評估結果
 * @param {Object|null} basicResult - 基本評估結果（無母音過濾）
 */
function updateAnalysisResultsUI(vowelResult, basicResult) {
    // 打印完整的 API 回應結果進行診斷
    console.log('Updating analysis results UI with:', { vowelResult, basicResult });
    
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
    
    console.log('Normalized values - Score:', score, 'Matched:', matchedNotes + '/' + totalNotes, 'NoteComparison length:', normalizedNoteComparison.length);
    
    // 更新圖表區域顯示視覺化
    const chartArea = document.getElementById('chart-area');
    chartArea.innerHTML = `
        <div class="viz-container">
            <div class="viz-score">
                <div class="viz-title">分數</div>
                <div id="score-gauge"></div>
            </div>
            <div class="viz-pitch">
                <div class="viz-title">音高準度</div>
                <div id="pitch-chart"></div>
            </div>
        </div>
    `;
    
    // 繪製視覺化
    import('./practice-ui.js').then(ui => {
        // 繪製分數儀表
        const scoreGaugeContainer = document.getElementById('score-gauge');
        if (scoreGaugeContainer) {
            ui.drawScoreGauge(score, scoreGaugeContainer);
        }
        
        // 繪製音高偏差圖表
        const pitchChartContainer = document.getElementById('pitch-chart');
        if (pitchChartContainer && normalizedNoteComparison.length > 0) {
            ui.drawPitchDeviationChart(normalizedNoteComparison, pitchChartContainer);
        }
    });
    
    // 更新詳細結果面板 - 顯示兩種評估結果
    const resultsPanel = document.getElementById('results-panel');
    if (resultsPanel) {
        resultsPanel.innerHTML = `
            <div class="assessment-comparison">
                <h3 style="color:#fff;margin:0 0 12px 0;font-size:16px;">練習評估結果</h3>
                <div class="results-grid-two">
                    ${renderAssessmentCard(vowelNorm, '母音過濾', normalizedNoteComparison)}
                    ${renderAssessmentCard(basicNorm, '基本比對', basicNorm ? normalizeNoteComparison(basicNorm.noteComparison) : [])}
                </div>
            </div>
        `;
        resultsPanel.classList.add('visible');
        
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
                <h4>音符比對</h4>
                <div id="note-table-${label === '母音過濾' ? 'vowel' : 'basic'}"></div>
            </div>
        </div>
    `;
}

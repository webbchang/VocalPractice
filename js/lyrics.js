// === Lyrics Module ===
import { state } from './state.js';
import { api } from './api.js';
import { escapeHtml } from './utils.js';

export async function loadLyrics() {
    const trackId = state.selectedVocalTrack;
    if (!trackId || !state.selectedSong) {
        state.lyricsLines = [];
        return;
    }

    try {
        // Fetch structures with lyrics for the selected vocal track
        const result = await api(`/songs/${state.selectedSong.id}/structures?track_id=${trackId}`);
        const structures = result.structures || [];

        // Flatten to get all PHRASE nodes with lyrics
        const lines = [];
        function extractLyrics(nodes) {
            for (const node of nodes) {
                if (node.type === 'PHRASE' && node.lyrics) {
                    lines.push({
                        structureId: node.id,
                        lyrics: node.lyrics,
                        startTime: node.start_time ?? 0,
                        endTime: node.end_time ?? 0
                    });
                }
                if (node.phrases && node.phrases.length) {
                    extractLyrics(node.phrases);
                }
            }
        }
        extractLyrics(structures);

        // Filter by selected range if any
        if (state.rangeStartId && state.rangeEndId) {
            const rangeStart = state.selectedRangeStart?.startTime ?? 0;
            const rangeEnd = state.selectedRangeEnd?.endTime ?? Infinity;
            state.lyricsLines = lines.filter(l => l.startTime >= rangeStart && l.endTime <= rangeEnd);
        } else {
            state.lyricsLines = lines;
        }
    } catch (err) {
        console.error('Failed to load lyrics:', err);
        state.lyricsLines = [];
    }
}

export function renderLyrics() {
    const container = document.getElementById('lyrics-lines');
    const wrapper = document.getElementById('lyrics-container');
    if (!container) return;

    if (state.lyricsLines.length === 0) {
        if (wrapper) wrapper.style.display = 'none';
        return;
    }

    if (wrapper) wrapper.style.display = 'block';

    container.innerHTML = state.lyricsLines.map((line, idx) => `
        <div class="lyrics-line" data-lyrics-idx="${idx}" data-start="${line.startTime}" data-end="${line.endTime}">
            ${escapeHtml(line.lyrics)}
        </div>
    `).join('');
}

export function startLyricsSync() {
    stopLyricsSync();
    if (state.lyricsLines.length === 0) return;

    const audioCtx = state.audioCtx;
    if (!audioCtx) return;

    function update() {
        if (!state.isRecording && !state.playbackStartTime) {
            state.lyricsAnimFrame = requestAnimationFrame(update);
            return;
        }

        const currentTime = audioCtx.currentTime - state.playbackStartTime + 
            (state.selectedRangeStart?.startTime ?? 0);

        let activeIndex = -1;
        for (let i = 0; i < state.lyricsLines.length; i++) {
            const line = state.lyricsLines[i];
            const isActive = currentTime >= line.startTime && currentTime < line.endTime;
            const isPassed = currentTime >= line.endTime;

            const el = document.querySelector(`.lyrics-line[data-lyrics-idx="${i}"]`);
            if (el) {
                el.classList.toggle('active', isActive);
                el.classList.toggle('passed', isPassed && !isActive);
                if (isActive) {
                    activeIndex = i;
                }
            }
        }

        // Auto-scroll to active lyric
        if (activeIndex >= 0) {
            const activeEl = document.querySelector(`.lyrics-line[data-lyrics-idx="${activeIndex}"]`);
            if (activeEl) {
                activeEl.scrollIntoView({ behavior: 'smooth', block: 'center' });
            }
        }

        state.lyricsAnimFrame = requestAnimationFrame(update);
    }

    state.lyricsAnimFrame = requestAnimationFrame(update);
}

export function stopLyricsSync() {
    if (state.lyricsAnimFrame) {
        cancelAnimationFrame(state.lyricsAnimFrame);
        state.lyricsAnimFrame = null;
    }
}
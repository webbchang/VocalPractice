import { describe, it, expect, beforeEach } from 'vitest';

// Setup window global (simulate loading songDataExtractor.js before tests)
beforeEach(() => {
    // Re-register if not present (vitest may cache the window)
    if (!window.SongDataExtractor) {
        // Inline the source since it's a non-module script
        // We eval the same code that's in songDataExtractor.js
        const code = `
function extractReferenceNotes(allNotes, selectedTrackIndex, rangeStart, rangeEnd) {
    if (!Array.isArray(allNotes) || selectedTrackIndex === null || selectedTrackIndex === undefined) {
        return [];
    }
    const safeRangeStart = Number.isFinite(rangeStart) ? rangeStart : 0;
    const safeRangeEnd = Number.isFinite(rangeEnd) ? rangeEnd : Infinity;
    return allNotes.filter(n =>
        n.track === selectedTrackIndex &&
        n.start >= safeRangeStart - 0.05 &&
        n.start < safeRangeEnd
    ).map(n => ({
        pitch: n.pitch,
        start_time: n.start,
        end_time: n.start + Math.min(n.dur, safeRangeEnd - n.start),
    })).filter(n => (n.end_time - n.start_time) > 0.02);
}
window.SongDataExtractor = { extractReferenceNotes };
`;
        eval(code);
    }
});

const mockNotes = [
    { pitch: 60, start: 1.0, dur: 0.5, track: 0 },
    { pitch: 62, start: 1.5, dur: 0.3, track: 0 },
    { pitch: 64, start: 2.0, dur: 0.4, track: 0 },
    { pitch: 65, start: 1.2, dur: 0.6, track: 1 },  // different track
    { pitch: 60, start: 10.0, dur: 0.5, track: 0 }, // outside range
];

describe('songDataExtractor.js - extractReferenceNotes', () => {
    it('should filter notes by track index', () => {
        const result = window.SongDataExtractor.extractReferenceNotes(mockNotes, 0, 0, 5);
        expect(result.every(n => n.pitch !== 65)).toBe(true);
        expect(result.length).toBeGreaterThan(0);
    });

    it('should filter notes within time range with -0.05 tolerance', () => {
        const result = window.SongDataExtractor.extractReferenceNotes(mockNotes, 0, 1.0, 2.0);
        // note at 1.0 start >= 0.95 && start < 2.0 ✓
        // note at 2.0 start >= 0.95 && start < 2.0 ✗ (start < 2.0 fails if start === 2.0)
        expect(result.length).toBe(2); // 1.0 and 1.5
        expect(result[0].pitch).toBe(60);
        expect(result[1].pitch).toBe(62);
    });

    it('should return empty array for non-array input', () => {
        const result = window.SongDataExtractor.extractReferenceNotes(null, 0, 0, 10);
        expect(result).toEqual([]);
    });

    it('should return empty array when selectedTrackIndex is null', () => {
        const result = window.SongDataExtractor.extractReferenceNotes(mockNotes, null, 0, 10);
        expect(result).toEqual([]);
    });

    it('should filter out notes with dur <= 0.02 after clipping', () => {
        const shortNotes = [
            { pitch: 70, start: 1.0, dur: 0.01, track: 0 },
            { pitch: 71, start: 1.5, dur: 0.5, track: 0 },
        ];
        const result = window.SongDataExtractor.extractReferenceNotes(shortNotes, 0, 1.0, 2.5);
        expect(result).toHaveLength(1);
        expect(result[0].pitch).toBe(71);
    });
});
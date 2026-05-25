import { describe, it, expect, beforeEach } from 'vitest';
import { SongDataConverter } from '../utils/songdata-converter.js';

describe('SongDataConverter', () => {
    beforeEach(() => {
        // Reset to defaults before each test
        SongDataConverter.updateContext({ tempo_map: [], ticks_per_quarter: null });
    });

    describe('updateContext', () => {
        it('should set tempo map and PPQ from song data', () => {
            const song = {
                tempo_map: [{ tick: 0, time_sec: 0, tempo_usec_per_qn: 500000 }],
                ticks_per_quarter: 480
            };
            SongDataConverter.updateContext(song);
            expect(SongDataConverter.internalTempoMap).toEqual(song.tempo_map);
            expect(SongDataConverter.internalPPQ).toBe(480);
        });

        it('should use defaults when song data is missing fields', () => {
            SongDataConverter.updateContext({});
            expect(SongDataConverter.internalTempoMap).toEqual([]);
            expect(SongDataConverter.internalPPQ).toBe(480);
        });
    });

    describe('toTick', () => {
        it('should convert seconds to ticks with default tempo (120 BPM, PPQ=480)', () => {
            // At 120 BPM, each quarter note = 0.5 sec, PPQ=480 => each tick = 0.5/480 ≈ 0.0010417 sec
            // 1 sec ≈ 960 ticks
            SongDataConverter.updateContext({ tempo_map: [], ticks_per_quarter: 480 });
            const tick = SongDataConverter.toTick(1);
            expect(tick).toBe(960);
        });

        it('should convert using tempo map', () => {
            const song = {
                tempo_map: [{ tick: 0, time_sec: 0, tempo_usec_per_qn: 500000 }],
                ticks_per_quarter: 480
            };
            SongDataConverter.updateContext(song);
            const tick = SongDataConverter.toTick(0.5);
            // 0.5 sec / (500000 / (480 * 1000000)) = 0.5 / 0.00104167 ≈ 480
            expect(tick).toBe(480);
        });

        it('should handle empty tempo map correctly', () => {
            SongDataConverter.updateContext({ tempo_map: [], ticks_per_quarter: 480 });
            expect(SongDataConverter.toTick(0)).toBe(0);
        });

        it('should never return negative ticks', () => {
            SongDataConverter.updateContext({ tempo_map: [], ticks_per_quarter: 480 });
            expect(SongDataConverter.toTick(-1)).toBe(0);
        });
    });

    describe('toSec', () => {
        it('should convert ticks to seconds with default tempo', () => {
            SongDataConverter.updateContext({ tempo_map: [], ticks_per_quarter: 480 });
            const sec = SongDataConverter.toSec(960);
            expect(sec).toBeCloseTo(1.0, 1);
        });

        it('should convert using tempo map', () => {
            const song = {
                tempo_map: [{ tick: 0, time_sec: 0, tempo_usec_per_qn: 500000 }],
                ticks_per_quarter: 480
            };
            SongDataConverter.updateContext(song);
            const sec = SongDataConverter.toSec(480);
            expect(sec).toBeCloseTo(0.5, 1);
        });
    });

    describe('formatTime', () => {
        it('should format seconds as m:ss', () => {
            expect(SongDataConverter.formatTime(0)).toBe('0:00');
            expect(SongDataConverter.formatTime(30)).toBe('0:30');
            expect(SongDataConverter.formatTime(60)).toBe('1:00');
            expect(SongDataConverter.formatTime(90)).toBe('1:30');
            expect(SongDataConverter.formatTime(3661)).toBe('61:01');
        });

        it('should handle null/undefined', () => {
            expect(SongDataConverter.formatTime(null)).toBe('0:00');
            expect(SongDataConverter.formatTime(undefined)).toBe('0:00');
        });
    });

    describe('formatDuration', () => {
        it('should format durations less than 60 seconds', () => {
            expect(SongDataConverter.formatDuration(0, 30)).toBe('30秒');
            expect(SongDataConverter.formatDuration(10, 15)).toBe('5秒');
        });

        it('should format durations of 60 seconds or more', () => {
            expect(SongDataConverter.formatDuration(0, 120)).toBe('2.0分');
            expect(SongDataConverter.formatDuration(30, 150)).toBe('2.0分');
        });

        it('should handle null/undefined', () => {
            expect(SongDataConverter.formatDuration(null, null)).toBe('');
            expect(SongDataConverter.formatDuration(0, null)).toBe('');
        });
    });

    describe('normalizeStructures', () => {
        it('should separate sections and nest phrases by parent_id', () => {
            const raw = [
                { id: 's1', type: 'SECTION', title: 'Verse', start_time: 0, end_time: 10, order_index: 1 },
                { id: 'p1', type: 'PHRASE', title: 'Phrase 1', start_time: 0, end_time: 5, parent_id: 's1', order_index: 1 },
                { id: 'p2', type: 'PHRASE', title: 'Phrase 2', start_time: 5, end_time: 10, parent_id: 's1', order_index: 2 }
            ];
            const result = SongDataConverter.normalizeStructures(raw);
            expect(result).toHaveLength(1);
            expect(result[0].id).toBe('s1');
            expect(result[0].phrases).toHaveLength(2);
            expect(result[0].phrases[0].id).toBe('p1');
            expect(result[0].phrases[1].id).toBe('p2');
        });

        it('should sort sections by start_time', () => {
            const raw = [
                { id: 's2', type: 'SECTION', title: 'Chorus', start_time: 20, end_time: 40, order_index: 2 },
                { id: 's1', type: 'SECTION', title: 'Verse', start_time: 0, end_time: 20, order_index: 1 }
            ];
            const result = SongDataConverter.normalizeStructures(raw);
            expect(result[0].id).toBe('s1');
            expect(result[1].id).toBe('s2');
        });

        it('should sort phrases by start_time within each section', () => {
            const raw = [
                { id: 's1', type: 'SECTION', title: 'Verse', start_time: 0, end_time: 10, order_index: 1 },
                { id: 'p2', type: 'PHRASE', title: 'B', start_time: 5, end_time: 10, parent_id: 's1', order_index: 2 },
                { id: 'p1', type: 'PHRASE', title: 'A', start_time: 0, end_time: 5, parent_id: 's1', order_index: 1 }
            ];
            const result = SongDataConverter.normalizeStructures(raw);
            expect(result[0].phrases[0].id).toBe('p1');
            expect(result[0].phrases[1].id).toBe('p2');
        });

        it('should return empty array for invalid input', () => {
            expect(SongDataConverter.normalizeStructures(null)).toEqual([]);
            expect(SongDataConverter.normalizeStructures(undefined)).toEqual([]);
        });

        it('should filter out phrases without matching parent section', () => {
            const raw = [
                { id: 's1', type: 'SECTION', title: 'Verse', start_time: 0, end_time: 10, order_index: 1 },
                { id: 'p1', type: 'PHRASE', title: 'Orphan', start_time: 0, end_time: 5, parent_id: 'nonexistent', order_index: 1 }
            ];
            const result = SongDataConverter.normalizeStructures(raw);
            expect(result[0].phrases).toHaveLength(0);
        });
    });
});
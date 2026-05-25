import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Mock window.MidiParser
beforeEach(() => {
    window.MidiParser = {
        parseMIDINotes: vi.fn((buf) => [
            { pitch: 60, start: 1.0, dur: 0.5, track: 0 },
            { pitch: 62, start: 1.5, dur: 0.3, track: 0 },
            { pitch: 64, start: 2.0, dur: 0.4, track: 0 },
        ]),
        midiPitchToFreq: (pitch) => 440 * Math.pow(2, (pitch - 69) / 12),
    };
    vi.useFakeTimers();

    function MockAudioContext() {
        return {
            currentTime: 10,
            createOscillator: () => ({
                type: '',
                frequency: { value: 0 },
                connect: vi.fn(function() { return this; }),
                start: vi.fn(),
                stop: vi.fn(),
                disconnect: vi.fn(),
            }),
            createGain: () => ({
                gain: { value: 0, setValueAtTime: vi.fn(), linearRampToValueAtTime: vi.fn() },
                connect: vi.fn(function() { return this; }),
                disconnect: vi.fn(),
            }),
            createBiquadFilter: () => ({
                type: '',
                frequency: { value: 0 },
                Q: { value: 0 },
                connect: vi.fn(function() { return this; }),
                disconnect: vi.fn(),
            }),
            destination: 'dest',
            state: 'running',
            resume: vi.fn().mockResolvedValue(),
        };
    }
    window.AudioContext = MockAudioContext;
    window.webkitAudioContext = undefined;
});

afterEach(() => {
    vi.useRealTimers();
});

describe('WebAudioAdapter', () => {
    let adapter;

    beforeEach(async () => {
        vi.clearAllMocks();
        const mod = await import('../adapters/audio-adapter.js');
        adapter = mod.WebAudioAdapter;
        adapter.audioCtx = null;
        adapter.playbackNodes = [];
        adapter.playbackTimer = null;
        adapter.internalParsedNotes = null;
        adapter.onStopCallback = null;
    });

    describe('getInternalCtx', () => {
        it('should create AudioContext on first call', () => {
            const ctx = adapter.getInternalCtx();
            expect(ctx).toBeDefined();
            expect(ctx.currentTime).toBe(10);
        });

        it('should reuse existing AudioContext', () => {
            const ctx1 = adapter.getInternalCtx();
            const ctx2 = adapter.getInternalCtx();
            expect(ctx1).toBe(ctx2);
        });
    });

    describe('loadData', () => {
        it('should parse MIDI buffer and store notes', () => {
            const buf = new ArrayBuffer(8);
            adapter.loadData(buf);
            expect(window.MidiParser.parseMIDINotes).toHaveBeenCalledWith(buf);
            expect(adapter.internalParsedNotes).toHaveLength(3);
        });
    });

    describe('getNoteCount', () => {
        it('should return 0 when no data loaded', () => {
            expect(adapter.getNoteCount()).toBe(0);
        });

        it('should return note count after loadData', () => {
            adapter.loadData(new ArrayBuffer(8));
            expect(adapter.getNoteCount()).toBe(3);
        });
    });

    describe('play', () => {
        it('should alert when MIDI not loaded', () => {
            const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {});
            adapter.play(0, 5);
            expect(alertSpy).toHaveBeenCalledWith('MIDI 尚未載入');
            alertSpy.mockRestore();
        });

        it('should create oscillators for notes in range', () => {
            adapter.loadData(new ArrayBuffer(8));
            adapter.play(0.5, 2.5);

            expect(adapter.playbackNodes.length).toBeGreaterThan(0);
            // Each note creates osc + gain = 2, 3 notes = 6, plus masterFilter + masterGain = 8
            expect(adapter.playbackNodes.length).toBe(8);
        });

        it('should not create nodes when no notes in range', () => {
            adapter.loadData(new ArrayBuffer(8));
            adapter.play(10, 20);
            expect(adapter.playbackNodes.length).toBe(0);
        });

        it('should set auto-stop timer', () => {
            adapter.loadData(new ArrayBuffer(8));
            adapter.play(0, 2);
            expect(adapter.playbackTimer).not.toBeNull();
        });
    });

    describe('stop', () => {
        it('should clear timer and disconnect nodes', () => {
            adapter.loadData(new ArrayBuffer(8));
            adapter.play(0.5, 2.5);
            expect(adapter.playbackNodes.length).toBeGreaterThan(0);

            adapter.stop();
            expect(adapter.playbackNodes.length).toBe(0);
            expect(adapter.playbackTimer).toBeNull();
        });

        it('should call onStopCallback when set', () => {
            const callback = vi.fn();
            adapter.onStopCallback = callback;
            adapter.stop();
            expect(callback).toHaveBeenCalled();
        });

        it('should handle stop when nothing is playing', () => {
            expect(() => adapter.stop()).not.toThrow();
        });
    });
});
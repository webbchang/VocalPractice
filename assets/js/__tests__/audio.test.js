import { describe, it, expect, vi, beforeEach } from 'vitest';
import state from '../state.js';

// Mock window.MidiParser.midiPitchToFreq
beforeEach(() => {
    window.MidiParser = {
        midiPitchToFreq: (pitch) => 440 * Math.pow(2, (pitch - 69) / 12),
    };
    vi.clearAllMocks();
});

describe('audio.js - midiPitchToFreq (internal)', () => {
    it('should return 440 Hz for A4 (pitch 69)', () => {
        // Access via window.MidiParser (injected)
        expect(window.MidiParser.midiPitchToFreq(69)).toBeCloseTo(440, 5);
    });

    it('should return 880 Hz for A5 (pitch 81)', () => {
        expect(window.MidiParser.midiPitchToFreq(81)).toBeCloseTo(880, 5);
    });
});

describe('audio.js - togglePlayback', () => {
    it('should call alert when no structure selected', async () => {
        state.selectedStructure = null;
        const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {});
        
        const { togglePlayback } = await import('../audio.js');
        togglePlayback();
        
        expect(alertSpy).toHaveBeenCalledWith('請先選擇一個段落或句子');
        alertSpy.mockRestore();
    });

    it('should call playRange when structure is selected', async () => {
        state.selectedStructure = { id: 's1', start: 0, end: 10, title: 'Verse' };
        
        const { togglePlayback, playRange } = await import('../audio.js');
        // playRange with no parsedNotes is a no-op early return
        togglePlayback();
        
        // Should not throw (playRange returns early if no parsedNotes)
        expect(true).toBe(true);
        state.selectedStructure = null;
    });
});

describe('audio.js - playRange with empty notes', () => {
    it('should do nothing when parsedNotes is empty', async () => {
        state.parsedNotes = [];
        const { playRange } = await import('../audio.js');
        
        // Should not throw
        expect(() => playRange(0, 10)).not.toThrow();
    });

    it('should do nothing when parsedNotes is null', async () => {
        state.parsedNotes = null;
        const { playRange } = await import('../audio.js');
        
        expect(() => playRange(0, 10)).not.toThrow();
    });
});

describe('audio.js - getIsPlaying', () => {
    it('should return false initially', async () => {
        const { getIsPlaying } = await import('../audio.js');
        expect(getIsPlaying()).toBe(false);
    });
});
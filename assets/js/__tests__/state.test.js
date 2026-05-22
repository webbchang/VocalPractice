import { describe, it, expect } from 'vitest';
import state from '../state.js';

describe('state.js', () => {
    it('should have correct initial values for all properties', () => {
        expect(state.currentSongId).toBeNull();
        expect(state.currentSongFull).toBeNull();
        expect(state.structures).toEqual([]);
        expect(state.selectedTrackId).toBeNull();
        expect(state.selectedAccompanimentTrackIds).toEqual([]);
        expect(state.selectedStructure).toBeNull();
        expect(state.midiData).toBeNull();
        expect(state.parsedNotes).toBeNull();
        expect(state.referenceNotes).toBeNull();
        expect(state.allTracksMeta).toEqual([]);
        expect(state.parsedTrackIndex).toBeNull();
        expect(state.parsedMIDITracks).toEqual([]);
    });

    it('should correctly distinguish null vs array defaults', () => {
        expect(state.structures).not.toBeNull();
        expect(state.selectedAccompanimentTrackIds).not.toBeNull();
        expect(state.allTracksMeta).not.toBeNull();
        expect(state.parsedMIDITracks).not.toBeNull();
        expect(state.currentSongId).toBeNull();
        expect(state.currentSongFull).toBeNull();
    });

    it('should allow property mutation', () => {
        state.currentSongId = 'song-123';
        expect(state.currentSongId).toBe('song-123');

        state.structures = [{ id: 's1', title: 'Verse' }];
        expect(state.structures).toHaveLength(1);
        expect(state.structures[0].title).toBe('Verse');

        state.selectedAccompanimentTrackIds = ['track-a', 'track-b'];
        expect(state.selectedAccompanimentTrackIds).toEqual(['track-a', 'track-b']);

        // Cleanup
        state.currentSongId = null;
        state.structures = [];
        state.selectedAccompanimentTrackIds = [];
    });
});
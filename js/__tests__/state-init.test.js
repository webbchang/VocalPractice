import { describe, it, expect } from 'vitest';
import { state } from '../state.js';

describe('state initial values', () => {
  it('should have correct default initial values', () => {
    expect(state.token).toBeNull();
    expect(state.user).toBeNull();
    expect(state.songs).toEqual([]);
    expect(state.selectedSong).toBeNull();
    expect(state.selectedVocalTrack).toBeNull();
    expect(state.selectedAccompanimentTracks).toEqual([]);
    expect(state.selectedStructure).toBeNull();
    expect(state.structures).toEqual([]);
    expect(state.isRecording).toBe(false);
    expect(state.isCountIn).toBe(false);
    expect(state.assessmentResult).toBeNull();
    expect(state.rangeStartId).toBeNull();
    expect(state.rangeEndId).toBeNull();
  });
});
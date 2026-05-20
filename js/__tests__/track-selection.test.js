import { describe, it, expect, vi, beforeEach } from 'vitest';
import { state } from '../state.js';

// Import the functions that manipulate state
import { clearRangeSelection, confirmRange, selectVocalTrack, toggleAccompaniment, updateStartButton } from '../track-selection.js';

describe('clearRangeSelection', () => {
  beforeEach(() => {
    state.rangeStartId = 'start-id';
    state.rangeEndId = 'end-id';
    state.selectedRangeStart = { startTime: 0 };
    state.selectedRangeEnd = { endTime: 10 };
  });

  it('should clear range state values', () => {
    document.body.innerHTML = `
      <div id="range-info"></div>
      <div id="range-actions"></div>
      <div class="structure-option selected"></div>
      <div class="structure-option in-range"></div>
    `;

    clearRangeSelection();

    expect(state.rangeStartId).toBeNull();
    expect(state.rangeEndId).toBeNull();
    expect(state.selectedRangeStart).toBeNull();
    expect(state.selectedRangeEnd).toBeNull();
  });

  it('should remove selected and in-range classes from DOM elements', () => {
    document.body.innerHTML = `
      <div id="range-info"></div>
      <div id="range-actions"></div>
      <div class="structure-option selected"></div>
      <div class="structure-option in-range"></div>
    `;

    clearRangeSelection();

    const elements = document.querySelectorAll('.structure-option');
    elements.forEach(el => {
      expect(el.classList.contains('selected')).toBe(false);
      expect(el.classList.contains('in-range')).toBe(false);
    });
  });
});

describe('toggleAccompaniment', () => {
  beforeEach(() => {
    state.selectedAccompanimentTracks = [];
  });

  it('should add track id if not already selected', () => {
    toggleAccompaniment('track-1');
    expect(state.selectedAccompanimentTracks).toEqual(['track-1']);
  });

  it('should remove track id if already selected', () => {
    state.selectedAccompanimentTracks = ['track-1', 'track-2'];
    toggleAccompaniment('track-1');
    expect(state.selectedAccompanimentTracks).toEqual(['track-2']);
  });

  it('should handle multiple toggles', () => {
    toggleAccompaniment('track-1');
    toggleAccompaniment('track-2');
    expect(state.selectedAccompanimentTracks).toEqual(['track-1', 'track-2']);

    toggleAccompaniment('track-1');
    expect(state.selectedAccompanimentTracks).toEqual(['track-2']);
  });
});

describe('selectVocalTrack', () => {
  it('should set selectedVocalTrack', () => {
    document.body.innerHTML = '<button id="start-practice-btn" disabled></button>';
    state.selectedVocalTrack = null;

    selectVocalTrack('track-1');
    expect(state.selectedVocalTrack).toBe('track-1');
  });
});

describe('updateStartButton', () => {
  it('should enable button when vocal track is selected', () => {
    document.body.innerHTML = '<button id="start-practice-btn" disabled></button>';
    state.selectedVocalTrack = 'track-1';

    updateStartButton();
    expect(document.getElementById('start-practice-btn').disabled).toBe(false);
  });

  it('should disable button when no vocal track is selected', () => {
    document.body.innerHTML = '<button id="start-practice-btn"></button>';
    state.selectedVocalTrack = null;

    updateStartButton();
    expect(document.getElementById('start-practice-btn').disabled).toBe(true);
  });
});

describe('confirmRange', () => {
  beforeEach(() => {
    state.selectedVocalTrack = null;
    state.rangeStartId = 'start-id';
    state.rangeEndId = 'end-id';
    state.structures = [
      { id: 'start-id', title: 'Verse 1' },
      { id: 'end-id', title: 'Chorus' },
    ];
  });

  it('should update DOM elements with confirmed range info', () => {
    document.body.innerHTML = `
      <div id="range-info"></div>
      <div id="range-actions"></div>
      <button id="start-practice-btn" disabled></button>
    `;

    confirmRange();

    const rangeInfo = document.getElementById('range-info');
    expect(rangeInfo.innerHTML).toContain('Verse 1');
    expect(rangeInfo.innerHTML).toContain('Chorus');
  });
});



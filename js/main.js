// === Main Entry Point ===
import { state } from './state.js';
import { api } from './api.js';
import { showScreen } from './navigation.js';
import { loadSongs, selectSong, goBackToSongs } from './songs.js';
import { onStructureClick, clearRangeSelection, confirmRange, selectVocalTrack, toggleAccompaniment } from './track-selection.js';
import { startPractice, goBackToTracks } from './practice.js';
import { toggleRecording } from './recording.js';
import { submitAssessment } from './results.js';
import { login, logout } from '../auth.js';

function initEventDelegation() {
    // Global click delegation for data-action elements
    document.addEventListener('click', (e) => {
        const target = e.target.closest('[data-action]');
        if (!target) return;

        const action = target.dataset.action;
        const id = target.dataset.id;

        switch (action) {
            case 'selectSong':
                selectSong(id);
                break;
            case 'selectVocalTrack':
                selectVocalTrack(id);
                // Also check the radio
                const radio = document.getElementById(`vocal-${id}`);
                if (radio) radio.checked = true;
                break;
            case 'toggleAccompaniment':
                toggleAccompaniment(id);
                break;
            case 'onStructureClick':
                onStructureClick(
                    target.dataset.id,
                    target.dataset.type,
                    parseFloat(target.dataset.start),
                    parseFloat(target.dataset.end),
                    target
                );
                break;
            case 'clearRangeSelection':
                clearRangeSelection();
                break;
            case 'confirmRange':
                confirmRange();
                break;
            case 'goBackToSongs':
                goBackToSongs();
                break;
            case 'goBackToTracks':
                goBackToTracks();
                break;
        }
    });
}

function initStaticBindings() {
    // Login
    document.getElementById('login-btn').addEventListener('click', () => {
        login(state, api, loadSongs);
    });

    // Logout - use dynamic import to avoid circular dependency
    document.getElementById('logout-btn').addEventListener('click', () => {
        import('./practice.js').then(mod => {
            logout(state, mod.stopAccompaniment, showScreen);
        });
    });

    // Start practice
    const startPracticeBtn = document.getElementById('start-practice-btn');
    if (startPracticeBtn) {
        startPracticeBtn.addEventListener('click', startPractice);
    }

    // Record button
    const recordBtn = document.getElementById('record-btn');
    if (recordBtn) {
        recordBtn.addEventListener('click', toggleRecording);
    }

    // Submit assessment button
    const submitBtn = document.getElementById('submit-btn');
    if (submitBtn) {
        submitBtn.addEventListener('click', submitAssessment);
    }
}

document.addEventListener('DOMContentLoaded', () => {
    initEventDelegation();
    initStaticBindings();
    showScreen('login-screen');
});
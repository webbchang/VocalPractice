import { SongService } from '../services/song-service.js';
import { UIStatus } from '../adapters/ui-renderer.js';
import { StructureRenderer } from '../adapters/ui-renderer.js';
import { UIErrorHandler } from '../utils/error-handler.js';
import { SongDataConverter } from '../utils/time-converter.js';
import { WebAudioAdapter } from '../adapters/audio-adapter.js';

let currentSongId = null;
let currentTrackId = null;
async function init() {
    try {
        const songs = await SongService.getSongs();
        StructureRenderer.renderSongsDropdown(songs);

        // 處理 URL 參數中的初始歌曲
        const urlParams = new URLSearchParams(window.location.search);
        const initialId = urlParams.get('song');
        if (initialId) {
            document.getElementById('song-select').value = initialId;
            await onSongChange();
        }
    } catch (err) {
        UIErrorHandler.notify(err);
    }
}
function setupTickAutoCalc() { };

async function onSongChange(songId) {
    WebAudioAdapter.stop(); // 🚩 切換歌曲前先停止播放，確保不會有殘留的音訊在背景播放
    currentSongId = document.getElementById('song-select').value;
    if (!currentSongId) {
        UIStatus.resetDashboard(); // 清空統計與 UI
        return;
    }

    SongDataConverter.updateContext(null); // 🚩 更新時間轉換器的上下文，確保新的歌曲資料能正確轉換時間
    try {
        UIStatus.showLoading('正在載入歌曲資訊...');
        
        // 1. 同時載入歌曲詳細資料與 MIDI
        const [songDetail, midiBuffer] = await Promise.all([
            SongService.getSongDetail(currentSongId),
            SongService.getMIDIData(currentSongId)
        ]);

        // 2. 更新轉換器與播放器的內部狀態
        SongDataConverter.updateContext(songDetail);
        WebAudioAdapter.loadData(midiBuffer);
        
        // 3. 更新 UI 組件（下拉選單與 MIDI 狀態）
        StructureRenderer.renderTrackSelector(songDetail.tracks);
        UIStatus.updateWaveformStatus(`MIDI 已載入 (${WebAudioAdapter.getNoteCount()} 音符)`, false);
        
        // 4. 若有預設聲部則自動載入結構
        await onTrackChange();
        
    } catch (err) {
        UIErrorHandler.notify(err);
    }
}
async function loadMIDIData(songId) {
    try {
        const midiBuffer = await SongService.getMidi(songId); // 透過服務層取得資料
        WebAudioAdapter.loadData(midiBuffer); // 🚩 呼叫適配器載入並解析資料

        // 更新 UI 狀態
        UIStatus.updateWaveformStatus(`📊 MIDI 已載入`);
    } catch (err) {
        UIErrorHandler.notify(err); // 🚩 統一錯誤處理 
    }
}
/** 當使用者切換聲部或需重新整理結構時 */
export async function onTrackChange() {
    currentTrackId = document.getElementById('lyrics-track-select').value;
    try {
        const data = await SongService.getStructures(currentSongId, currentTrackId);
        
        // 使用 Converter 正規化資料，再交給 Renderer 渲染 [2, 4]
        const normalized = SongDataConverter.normalizeStructures(data.structures);
        
        StructureRenderer.renderTimeline(normalized);
        StructureRenderer.renderTree(normalized, currentTrackId);
        
        // 更新上方統計數字
        UIStatus.updateStatCounters({
            sections: normalized.length,
            phrases: normalized.reduce((acc, s) => acc + s.phrases.length, 0),
            duration: SongDataConverter.calculateTotalDuration(normalized)
        });
    } catch (err) {
        UIErrorHandler.notify(err);
    }
}
async function loadTracksForLyrics(songId) { };
async function loadStructures(songId, trackId) {
    try {
        const structures = await SongService.getStructures(songId, trackId); // 透過服務層取得資料
        StructureRenderer.renderTimeLine(structures); // 🚩 呼叫適配器渲染時間軸
        StructureRenderer.renderTree(structures, trackId); // 🚩 呼叫適配器渲染樹狀圖
        UIStatus.updateStateCounters({ structureCount: structures.length }); // 🚩 更新 UI 統計數字
    } catch (err) {
        UIErrorHandler.notify(err); // 🚩 統一錯誤處理
    }
}
async function handlePlayButtonClick(startTime, endTime, btnId) {
    const btn = document.getElementById(btnId);

    if (!btn) return;

    // 1. 透過 UIStatus 判斷當前是否正在播放
    const isPlaying = btn.dataset.playing === 'true';

    try {
        if (isPlaying) {
            // 2. 停止邏輯：交由音訊適配器處理
            WebAudioAdapter.stop();
            UIStatus.setPlayButton(btn, false);
        } else {
            // 4. 播放邏輯：傳入範圍，內部會處理 AudioContext 恢復與資料篩選
            await WebAudioAdapter.play(startTime, endTime);
            UIStatus.setPlayButton(btn, true);
        }
    } catch (err) {
        // 5. 錯誤處理：統一交由 UIErrorHandler
        UIErrorHandler.notify(err);
    }
}
async function handleEdit(id, type) { };
async function handleSaveEdit() {
    const payload = UIStatus.getEditPanelData(); // 從 UI 蒐集資料
    try {
        if (payload.isNew) {
            await SongService.createStructures(currentSongId, payload);
        } else {
            await SongService.updateStructure(currentSongId, payload.id, payload);
        }

        UIStatus.hideEditPanel();
        await onTrackChange(); // 重新整理清單
    } catch (err) {
        UIErrorHandler.notify(err);
    }
};//songService to save, cancelEdit, loadStructures to refresh
async function handleDelete(id, title) { };//songService to delete, refresh 
async function handleSaveLyrics(sectionId) { };//songService.saveLyrics()
async function handleExport() { };//songService
async function handleImport(file) {
    try {
        const text = await fileInput.files.text();
        const res = await SongService.importCSV(currentSongId, text);

        UIStatus.setImportResult(`已匯入 ${res.imported} 筆`, 'success');
        await onTrackChange();
    } catch (err) {
        // UIErrorHandler 會處理 409 衝突並顯示詳細清單 [2]
        UIErrorHandler.notify(err);
    }
};//songService
async function handleCopyStruectures() { }
function cancelEdit() {
    StructureRenderer.hideEditPanel();
};
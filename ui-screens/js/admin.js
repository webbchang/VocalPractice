import { SongService } from './services/api-service.js';
import { UIStatus, StructureRenderer } from './adapters/ui-renderer.js';
import { UIErrorHandler } from './utils/error-handler.js';
import { SongDataConverter } from './utils/songdata-converter.js';
import { WebAudioAdapter } from './adapters/audio-adapter.js';

// --- State ---
let currentSongId = null;
let currentTrackId = null;
let structures = [];          // normalized structures (sections with nested phrases)
let editingNode = null;
let editingIsNew = false;
let activePlayBtn = null;

// --- Init ---
async function init() {
    // 檢查管理員身分
    const token = localStorage.getItem('token');
    const user = JSON.parse(localStorage.getItem('user') || 'null');
    if (!token || !user || user.role !== 'admin') {
        window.location.href = '../index.html';
        return;
    }

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

// --- 事件監聽綁定 ---
function bindEventListeners() {
    // 歌曲選擇
    document.getElementById('song-select').addEventListener('change', onSongChange);

    // 聲部選擇
    document.getElementById('lyrics-track-select').addEventListener('change', onTrackChange);

    // 頂部按鈕
    document.getElementById('btn-add-section').addEventListener('click', showAddSection);
    document.getElementById('btn-copy-structures').addEventListener('click', handleCopyStructures);
    document.getElementById('btn-export-csv').addEventListener('click', handleExport);
    document.getElementById('btn-import-csv-trigger').addEventListener('click', () => {
        document.getElementById('import-file').click();
    });
    document.getElementById('import-file').addEventListener('change', (e) => {
        handleImport(e.target);
    });

    // 編輯面板按鈕
    document.getElementById('btn-save-edit').addEventListener('click', saveEdit);
    document.getElementById('btn-cancel-edit').addEventListener('click', cancelEdit);

    // edit-type 切換
    document.getElementById('edit-type').addEventListener('change', function() {
        const parentSelect = document.getElementById('edit-parent');
        const parentGroup = parentSelect.closest('.form-group');
        if (this.value === 'PHRASE') {
            parentSelect.style.display = 'block';
            parentGroup.style.display = 'block';
        } else {
            parentSelect.style.display = 'none';
            parentGroup.style.display = 'none';
        }
        updateLyricsEditorVisibility(this.value);
    });

    // Event delegation: 結構樹中動態產生的按鈕
    document.getElementById('structure-tree').addEventListener('click', (e) => {
        const btn = e.target.closest('button');
        if (!btn) return;
        const action = btn.dataset.action;
        switch (action) {
            case 'play':
                handlePlayButtonClick(
                    parseFloat(btn.dataset.start),
                    parseFloat(btn.dataset.end),
                    btn
                );
                break;
            case 'edit':
                handleEdit(btn.dataset.id, btn.dataset.type);
                break;
            case 'delete':
                handleDelete(btn.dataset.id, btn.dataset.title);
                break;
            case 'add-phrase':
                showAddPhrase(btn.dataset.sectionId);
                break;
        }
    });

    // Event delegation: 時間軸區塊點擊
    document.getElementById('timeline').addEventListener('click', (e) => {
        const block = e.target.closest('.section-block');
        if (!block) return;
        handleEdit(block.dataset.structureId, block.dataset.structureType);
    });

    // 註冊播放停止回呼（自動重置按鈕狀態）
    WebAudioAdapter.onStopCallback = () => {
        if (activePlayBtn) {
            UIStatus.setPlayButton(activePlayBtn, false);
            activePlayBtn = null;
        }
    };
}

// --- Tick 自動計算 ---
function setupTickAutoCalc() {
    const startInput = document.getElementById('edit-start');
    const endInput = document.getElementById('edit-end');
    const startTickInput = document.getElementById('edit-start-tick');
    const endTickInput = document.getElementById('edit-end-tick');

    function recalcTicks() {
        const startTime = parseFloat(startInput.value) || 0;
        const endTime = parseFloat(endInput.value) || 0;
        startTickInput.value = SongDataConverter.toTick(startTime);
        endTickInput.value = SongDataConverter.toTick(endTime);
    }

    startInput.addEventListener('input', recalcTicks);
    endInput.addEventListener('input', recalcTicks);
}

// --- 歌曲切換 ---
async function onSongChange() {
    WebAudioAdapter.stop();
    currentSongId = document.getElementById('song-select').value;

    if (!currentSongId) {
        UIStatus.resetDashboard();
        return;
    }

    SongDataConverter.updateContext({ tempo_map: [], ticks_per_quarter: null });

    try {
        UIStatus.showLoading('正在載入歌曲資訊...');

        // 載入歌曲詳細資訊（含 tempo map、tracks）
        const songDetail = await SongService.getSongDetail(currentSongId);
        SongDataConverter.updateContext(songDetail);
        StructureRenderer.renderTrackSelector(songDetail.tracks);

        // 非同步載入 MIDI（失敗不影響結構操作）
        SongService.getMIDIData(currentSongId).then(midiBuffer => {
            WebAudioAdapter.loadData(midiBuffer);
            UIStatus.updateWaveformStatus(`MIDI 已載入 (${WebAudioAdapter.getNoteCount()} 音符)`, false);
        }).catch(() => {
            UIStatus.updateWaveformStatus('⚠️ MIDI 載入失敗，不影響結構編輯', true);
        });

        await onTrackChange();
    } catch (err) {
        UIErrorHandler.notify(err);
    }
}

// --- 聲部切換 → 載入結構 ---
async function onTrackChange() {
    currentTrackId = document.getElementById('lyrics-track-select').value;
    if (!currentSongId) return;

    try {
        const data = await SongService.getStructures(currentSongId, currentTrackId);
        const normalized = SongDataConverter.normalizeStructures(data.structures);
        structures = normalized;

        StructureRenderer.renderTimeline(normalized);
        StructureRenderer.renderTree(normalized, currentTrackId);

        // 更新統計
        let maxTime = 0;
        let phraseCount = 0;
        for (const s of normalized) {
            if (s.end_time > maxTime) maxTime = s.end_time;
            if (s.phrases) {
                phraseCount += s.phrases.length;
                for (const p of s.phrases) {
                    if (p.end_time > maxTime) maxTime = p.end_time;
                }
            }
        }
        UIStatus.updateStatCounters({
            sections: normalized.length,
            phrases: phraseCount,
            duration: SongDataConverter.formatTime(maxTime)
        });
    } catch (err) {
        UIErrorHandler.notify(err);
    }
}

// --- MIDI 載入（獨立使用）---
async function loadMIDIData(songId) {
    try {
        const midiBuffer = await SongService.getMIDIData(songId);
        WebAudioAdapter.loadData(midiBuffer);
        UIStatus.updateWaveformStatus(`📊 MIDI 已載入`, false);
    } catch (err) {
        UIErrorHandler.notify(err);
    }
}

// --- 播放/停止 ---
function handlePlayButtonClick(startTime, endTime, btn) {
    if (!btn) return;

    const isPlaying = btn.dataset.playing === 'true';

    if (isPlaying) {
        WebAudioAdapter.stop();
        UIStatus.setPlayButton(btn, false);
        if (activePlayBtn === btn) activePlayBtn = null;
    } else {
        WebAudioAdapter.play(startTime, endTime);
        UIStatus.setPlayButton(btn, true);
        activePlayBtn = btn;
    }
}

// --- 查詢結構節點（依 id）---
function findStructureById(id) {
    for (const s of structures) {
        if (s.id === id) return s;
        if (s.phrases) {
            for (const p of s.phrases) {
                if (p.id === id) return p;
            }
        }
    }
    return null;
}

// --- 編輯結構（開啟編輯面板）---
function handleEdit(id, type) {
    const node = findStructureById(id);
    if (!node) {
        alert('找不到此結構');
        return;
    }

    editingNode = node;
    editingIsNew = false;

    const panel = document.getElementById('edit-panel');
    document.getElementById('edit-panel-title').textContent = '✏️ 編輯：' + node.title;
    document.getElementById('edit-title').value = node.title || '';
    document.getElementById('edit-type').value = type || node.type || 'SECTION';
    document.getElementById('edit-start').value = node.start_time || 0;
    document.getElementById('edit-end').value = node.end_time || 0;
    document.getElementById('edit-start-tick').value = node.start_tick || 0;
    document.getElementById('edit-end-tick').value = node.end_tick || 0;
    document.getElementById('edit-start').dispatchEvent(new Event('input'));
    document.getElementById('edit-order').value = node.order_index || 0;
    document.getElementById('edit-lyrics').value = node.lyrics || '';

    // 父段落下拉
    const parentSelect = document.getElementById('edit-parent');
    parentSelect.innerHTML = '<option value="">無（段落）</option>';
    for (const s of structures) {
        if (s.id !== node.id) {
            const opt = document.createElement('option');
            opt.value = s.id;
            opt.textContent = s.title;
            if (s.id === node.parent_id) opt.selected = true;
            parentSelect.appendChild(opt);
        }
    }
    const isPhrase = (type === 'PHRASE' || node.type === 'PHRASE');
    parentSelect.style.display = isPhrase ? 'block' : 'none';
    parentSelect.closest('.form-group').style.display = isPhrase ? 'block' : 'none';
    updateLyricsEditorVisibility(isPhrase ? 'PHRASE' : 'SECTION');

    panel.style.display = 'block';
    panel.scrollIntoView({ behavior: 'smooth' });
}

// --- 新增段落 ---
function showAddSection() {
    if (!currentSongId) {
        alert('請先選擇歌曲');
        return;
    }

    editingNode = null;
    editingIsNew = true;

    const panel = document.getElementById('edit-panel');
    document.getElementById('edit-panel-title').textContent = '✏️ 新增段落';
    document.getElementById('edit-title').value = '';
    document.getElementById('edit-type').value = 'SECTION';
    document.getElementById('edit-start').value = 0;
    document.getElementById('edit-end').value = 0;
    document.getElementById('edit-start-tick').value = 0;
    document.getElementById('edit-end-tick').value = 0;
    document.getElementById('edit-start').dispatchEvent(new Event('input'));
    document.getElementById('edit-order').value = structures.length + 1;

    const parentSelect = document.getElementById('edit-parent');
    parentSelect.innerHTML = '<option value="">無（段落）</option>';
    for (const s of structures) {
        const opt = document.createElement('option');
        opt.value = s.id;
        opt.textContent = s.title;
        parentSelect.appendChild(opt);
    }
    parentSelect.style.display = 'none';
    parentSelect.closest('.form-group').style.display = 'none';
    updateLyricsEditorVisibility('SECTION');

    panel.style.display = 'block';
    panel.scrollIntoView({ behavior: 'smooth' });
}

// --- 新增句子 ---
function showAddPhrase(sectionId) {
    if (!currentSongId) {
        alert('請先選擇歌曲');
        return;
    }

    const section = structures.find(s => s.id === sectionId);
    if (!section) {
        alert('找不到段落');
        return;
    }

    editingNode = null;
    editingIsNew = true;

    const sortedPhrases = (section.phrases || []).slice().sort((a, b) => a.start_time - b.start_time);
    const phraseCount = sortedPhrases.length;
    const nextNum = phraseCount + 1;
    const defaultTitle = section.title + '-' + nextNum;
    const defaultStart = phraseCount > 0 ? sortedPhrases[phraseCount - 1].end_time : section.start_time;
    const defaultEnd = section.end_time;

    const panel = document.getElementById('edit-panel');
    document.getElementById('edit-panel-title').textContent = '✏️ 新增句子：' + defaultTitle;
    document.getElementById('edit-title').value = defaultTitle;
    document.getElementById('edit-type').value = 'PHRASE';
    document.getElementById('edit-start').value = defaultStart || 0;
    document.getElementById('edit-end').value = defaultEnd || 0;
    document.getElementById('edit-start-tick').value = 0;
    document.getElementById('edit-end-tick').value = 0;
    document.getElementById('edit-start').dispatchEvent(new Event('input'));
    document.getElementById('edit-order').value = nextNum;

    const parentSelect = document.getElementById('edit-parent');
    parentSelect.innerHTML = '<option value="">無（段落）</option>';
    for (const s of structures) {
        const opt = document.createElement('option');
        opt.value = s.id;
        opt.textContent = s.title;
        if (s.id === sectionId) opt.selected = true;
        parentSelect.appendChild(opt);
    }
    parentSelect.style.display = 'block';
    parentSelect.closest('.form-group').style.display = 'block';
    updateLyricsEditorVisibility('PHRASE');

    const hint = document.getElementById('section-lyrics-hint');
    if (hint) {
        hint.style.display = 'block';
        hint.textContent = '⏱ 開始時間以樂曲開頭 0:00 起算';
    }

    panel.style.display = 'block';
    panel.scrollIntoView({ behavior: 'smooth' });
}

// --- 編輯面板：歌詞可見性切換 ---
function updateLyricsEditorVisibility(type) {
    const isPhrase = (type || '').toUpperCase() === 'PHRASE';
    const lyricsRow = document.getElementById('edit-lyrics-row');
    const sectionHint = document.getElementById('section-lyrics-hint');
    if (lyricsRow) lyricsRow.style.display = isPhrase ? 'flex' : 'none';
    if (sectionHint) sectionHint.style.display = isPhrase ? 'none' : 'block';
}

// --- 儲存編輯 ---
async function saveEdit() {
    const title = document.getElementById('edit-title').value.trim();
    const type = document.getElementById('edit-type').value;
    const startTime = parseFloat(document.getElementById('edit-start').value) || 0;
    const endTime = parseFloat(document.getElementById('edit-end').value) || 0;
    const startTick = parseInt(document.getElementById('edit-start-tick').value) || 0;
    const endTick = parseInt(document.getElementById('edit-end-tick').value) || 0;
    const orderIdx = parseInt(document.getElementById('edit-order').value) || 0;
    const parentId = document.getElementById('edit-parent').value;
    const editLyrics = document.getElementById('edit-lyrics').value.trim();
    const trackId = document.getElementById('lyrics-track-select').value;

    // 如果未選擇聲部，fallback 到第一個非空的聲部
    let effectiveTrackId = trackId;
    if (!effectiveTrackId) {
        const select = document.getElementById('lyrics-track-select');
        for (let i = 0; i < select.options.length; i++) {
            if (select.options[i].value) {
                effectiveTrackId = select.options[i].value;
                break;
            }
        }
    }

    if (!title) { alert('請輸入標題'); return; }
    if (endTime <= startTime) { alert('結束時間必須大於開始時間'); return; }

    let parentSection = null;
    if (type === 'PHRASE') {
        if (!parentId) { alert('PHRASE 必須選擇父段落'); return; }
        parentSection = structures.find(s => s.id === parentId);
        if (!parentSection) { alert('找不到父段落'); return; }
        const parentStartTick = parseInt(parentSection.start_tick || 0);
        const parentEndTick = parseInt(parentSection.end_tick || 0);
        if (!(startTick >= parentStartTick && endTick <= parentEndTick)) {
            alert('PHRASE 的起訖 tick 必須落在父段落範圍內');
            return;
        }
    }

    try {
        if (editingIsNew) {
            // --- 新增 ---
            if (type === 'SECTION') {
                await SongService.createStructures(currentSongId, {
                    structures: [{
                        type: 'SECTION',
                        title,
                        start_time: startTime,
                        end_time: endTime,
                        start_tick: startTick,
                        end_tick: endTick,
                        order_index: orderIdx,
                        track_id: effectiveTrackId,
                        phrases: []
                    }]
                });
            } else if (type === 'PHRASE' && parentId) {
                if (parentSection) {
                    const existingPhrases = (parentSection.phrases || []).map(p => ({
                        type: p.type,
                        title: p.title,
                        start_time: p.start_time,
                        end_time: p.end_time,
                        start_tick: p.start_tick || 0,
                        end_tick: p.end_tick || 0,
                        order_index: p.order_index
                    }));
                    // DELETE old parent, POST new parent with phrases
                    await SongService.deleteStructure(currentSongId, parentId);
                    await SongService.createStructures(currentSongId, {
                        structures: [{
                            type: 'SECTION',
                            title: parentSection.title,
                            start_time: parentSection.start_time,
                            end_time: parentSection.end_time,
                            start_tick: parentSection.start_tick || 0,
                            end_tick: parentSection.end_tick || 0,
                            order_index: parentSection.order_index,
                            phrases: [
                                ...existingPhrases,
                                {
                                    type: 'PHRASE',
                                    title,
                                    start_time: startTime,
                                    end_time: endTime,
                                    start_tick: startTick,
                                    end_tick: endTick,
                                    order_index: orderIdx
                                }
                            ]
                        }]
                    });
                }
            }

            // 儲存新 PHRASE 的歌詞
            if (type === 'PHRASE' && editLyrics && trackId) {
                await onTrackChange(); // 重新取得結構，取得新 ID
                const updatedParent = structures.find(s => s.id === parentId);
                if (updatedParent && updatedParent.phrases) {
                    const matched = updatedParent.phrases.find(p =>
                        p.title === title &&
                        Number(p.start_time) === Number(startTime) &&
                        Number(p.end_time) === Number(endTime)
                    );
                    if (matched) {
                        await SongService.saveLyrics(currentSongId, [{
                            track_id: trackId,
                            structure_id: matched.id,
                            lyrics: editLyrics
                        }]);
                    }
                }
            }
        } else {
            // --- 編輯現有 ---
            const payload = {};
            if (editingNode.title !== title) payload.title = title;
            if (editingNode.start_time !== startTime) payload.start_time = startTime;
            if (editingNode.end_time !== endTime) payload.end_time = endTime;
            if ((editingNode.start_tick || 0) !== startTick) payload.start_tick = startTick;
            if ((editingNode.end_tick || 0) !== endTick) payload.end_tick = endTick;
            if (editingNode.order_index !== orderIdx) payload.order_index = orderIdx;
            if ((editingNode.type || '').toUpperCase() !== type) payload.type = type;
            const oldParentId = editingNode.parent_id || '';
            if (oldParentId !== parentId) payload.parent_id = parentId;

            if (Object.keys(payload).length > 0) {
                await SongService.updateStructure(currentSongId, editingNode.id, payload);
            }

            if (type === 'PHRASE' && editLyrics && trackId) {
                await SongService.saveLyrics(currentSongId, [{
                    track_id: trackId,
                    structure_id: editingNode.id,
                    lyrics: editLyrics
                }]);
            }
        }

        cancelEdit();
        await onTrackChange();
    } catch (err) {
        alert('儲存失敗：' + err.message);
    }
}

// --- 取消編輯 ---
function cancelEdit() {
    document.getElementById('edit-panel').style.display = 'none';
    editingNode = null;
    editingIsNew = false;
}

// --- 刪除結構 ---
async function handleDelete(id, title) {
    if (!confirm(`確定刪除「${title}」？\n此操作無法復原。`)) return;
    try {
        await SongService.deleteStructure(currentSongId, id);
        await onTrackChange();
    } catch (err) {
        alert('刪除失敗：' + err.message);
    }
}

// --- 匯出 CSV ---
async function handleExport() {
    if (!currentSongId) {
        alert('請先選擇歌曲');
        return;
    }
    try {
        const data = await SongService.exportCSV(currentSongId);
        const blob = new Blob([data.csv], { type: 'text/csv;charset=utf-8;' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = (data.song.title || 'song') + '-structures.csv';
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
        const trackNames = data.song.tracks.map(t => `${t.name} (${t.is_vocal ? '🎤' : '🎵'})`).join(', ');
        alert(`✅ 已匯出 ${data.csv.split('\n').length - 1} 行\n\n歌曲: ${data.song.title}\n聲部: ${trackNames}\n\nCSV 格式:\ntype,title,start,end,order,track_name,lyrics\nS=段落, P=樂句\n時間單位: 秒`);
    } catch (err) {
        alert('匯出失敗：' + err.message);
    }
}

// --- 匯入 CSV ---
async function handleImport(fileInput) {
    const file = fileInput.files[0];
    if (!file) return;
    const resultDiv = document.getElementById('import-result');
    resultDiv.textContent = '正在匯入...';
    resultDiv.style.color = '#888';

    try {
        const text = await file.text();
        const res = await SongService.importCSV(currentSongId, text);
        UIStatus.setImportResult(`✅ 已匯入 ${res.imported} 筆結構`, 'success');
        await onTrackChange();
    } catch (err) {
        // 處理 409 衝突等
        if (err.status === 409 && err.data) {
            let msg = '⚠️ 偵測到時間範圍衝突：\n';
            for (const c of (err.data.conflicts || [])) {
                msg += `\n- 「${c.existing_title}」(${c.existing_start}s-${c.existing_end}s) vs 「${c.incoming_title}」(${c.incoming_start}s-${c.incoming_end}s)`;
            }
            UIStatus.setImportResult('⚠️ 衝突', 'error');
            alert(msg);
        } else if (err.data && err.data.missing_tracks) {
            let msg = '❌ 找不到聲部：' + err.data.missing_tracks.join(', ');
            msg += '\n\n可用聲部：\n' + err.data.available_tracks.map(t => `- ${t.name}`).join('\n');
            UIStatus.setImportResult('❌ 聲部不符', 'error');
            alert(msg);
        } else {
            UIStatus.setImportResult('❌ 匯入失敗：' + err.message, 'error');
            alert('匯入失敗：' + err.message);
        }
    }
    fileInput.value = '';
}

// --- 複製結構（顯示對話框）---
async function handleCopyStructures() {
    const trackId = document.getElementById('lyrics-track-select').value;
    if (!trackId || !currentSongId) return;

    try {
        const song = await SongService.getSongDetail(currentSongId);
        const otherTracks = song.tracks.filter(t => t.id !== trackId);
        if (otherTracks.length === 0) {
            alert('沒有其他聲部可以複製');
            return;
        }

        let html = '<div style="position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,0.6);display:flex;align-items:center;justify-content:center;z-index:1000;" onclick="if(event.target===this)this.remove()">';
        html += '<div style="background:#16213e;border-radius:12px;padding:24px;max-width:500px;width:90%;max-height:80vh;overflow-y:auto;">';
        html += '<h3 style="margin-bottom:16px;">📋 從其他聲部複製段落結構</h3>';
        html += '<p style="color:#888;margin-bottom:16px;">注意：只複製段落框架，不含歌詞內容</p>';
        html += '<div style="display:flex;flex-direction:column;gap:8px;">';

        for (const t of otherTracks) {
            const safeName = t.name.replace(/'/g, "\\'");
            html += `<button class="btn btn-primary" style="text-align:left;padding:12px;" onclick="this.closest('[style*=\\'position:fixed\\']').remove(); window.__copyStructuresFromTrack('${t.id}', '${safeName}')">📥 從「${t.name}」複製</button>`;
        }

        html += '</div>';
        html += '<button class="btn btn-secondary" style="margin-top:16px;" onclick="this.closest(\'[style*=\\\'position:fixed\\\']\').remove()">取消</button>';
        html += '</div></div>';

        const div = document.createElement('div');
        div.innerHTML = html;
        document.body.appendChild(div.firstElementChild);
    } catch (err) {
        alert('無法載入聲部資訊：' + err.message);
    }
}

// --- 實際執行複製（透過 window 讓 dialog 按鈕能呼叫）---
window.__copyStructuresFromTrack = async function(sourceTrackId, sourceTrackName) {
    const targetTrackId = document.getElementById('lyrics-track-select').value;
    if (!targetTrackId || !currentSongId) return;

    document.querySelector('[style*="position:fixed"]')?.remove();

    if (!confirm(`確定要從「${sourceTrackName}」複製段落結構到目前聲部？`)) return;

    try {
        const sourceData = await SongService.getStructures(currentSongId, sourceTrackId);
        const sourceStructs = sourceData.structures || [];

        if (sourceStructs.length === 0) {
            alert('來源聲部沒有段落結構');
            return;
        }

        const payload = { structures: [] };
        for (const section of sourceStructs) {
            const sectionPayload = {
                type: 'SECTION',
                title: section.title,
                start_time: section.start_time,
                end_time: section.end_time,
                start_tick: section.start_tick || 0,
                end_tick: section.end_tick || 0,
                order_index: section.order_index || 0,
                track_id: targetTrackId,
                phrases: []
            };
            if (section.phrases) {
                for (const phrase of section.phrases) {
                    sectionPayload.phrases.push({
                        type: 'PHRASE',
                        title: phrase.title,
                        start_time: phrase.start_time,
                        end_time: phrase.end_time,
                        start_tick: phrase.start_tick || 0,
                        end_tick: phrase.end_tick || 0,
                        order_index: phrase.order_index || 0,
                        track_id: targetTrackId
                    });
                }
            }
            payload.structures.push(sectionPayload);
        }

        await SongService.createStructures(currentSongId, payload);
        await onTrackChange();
        alert('✅ 段落結構已複製完成');
    } catch (err) {
        alert('複製失敗：' + err.message);
    }
};

// --- Init ---
setupTickAutoCalc();
bindEventListeners();
init();
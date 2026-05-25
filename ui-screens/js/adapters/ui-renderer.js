// --- 通用工具函式（不依賴外部 import，供內部使用）---
function escapeHtml(str) {
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

export const StructureRenderer = {
    renderTimeline(structures) {
        const timeline = document.getElementById('timeline');
        const placeholder = document.getElementById('waveform-placeholder');

        if (!structures || structures.length === 0) {
            timeline.innerHTML = '<div style="text-align:center;color:#999;padding:20px;">尚無段落結構</div>';
            if (placeholder) placeholder.textContent = '📊 尚無段落結構，請先建立';
            return;
        }

        if (placeholder) placeholder.textContent = '📊 結構預覽 (' + structures.length + ' 個段落)';

        let maxTime = 0;
        for (const s of structures) {
            if (s.end_time > maxTime) maxTime = s.end_time;
            if (s.phrases) {
                for (const p of s.phrases) {
                    if (p.end_time > maxTime) maxTime = p.end_time;
                }
            }
        }
        if (maxTime === 0) maxTime = 1;

        timeline.innerHTML = '';
        for (const s of structures) {
            const left = (s.start_time / maxTime * 100).toFixed(1);
            const width = ((s.end_time - s.start_time) / maxTime * 100).toFixed(1);
            const block = document.createElement('div');
            block.className = 'section-block';
            block.style.cssText = `left: ${left}%; width: ${width}%; cursor: pointer;`;
            block.textContent = s.title;
            block.dataset.structureId = s.id;
            block.dataset.structureType = 'SECTION';
            timeline.appendChild(block);

            if (s.phrases) {
                for (const p of s.phrases) {
                    const pLeft = (p.start_time / maxTime * 100).toFixed(1);
                    const pWidth = ((p.end_time - p.start_time) / maxTime * 100).toFixed(1);
                    const pBlock = document.createElement('div');
                    pBlock.className = 'section-block phrase';
                    pBlock.style.cssText = `left: ${pLeft}%; width: ${pWidth}%; cursor: pointer;`;
                    pBlock.textContent = p.title;
                    pBlock.dataset.structureId = p.id;
                    pBlock.dataset.structureType = 'PHRASE';
                    timeline.appendChild(pBlock);
                }
            }
        }
    },

    renderSongsDropdown(songs) {
        const select = document.getElementById('song-select');
        if (!select) return;
        select.innerHTML = '<option value="">請選擇歌曲...</option>';
        for (const s of songs) {
            const opt = document.createElement('option');
            opt.value = s.id;
            opt.textContent = s.title + ' - ' + (s.artist || '未知');
            select.appendChild(opt);
        }
    },

    renderTrackSelector(tracks) {
        const select = document.getElementById('lyrics-track-select');
        if (!select) return;
        select.innerHTML = '<option value="">選擇聲部...</option>';
        for (const t of tracks) {
            const opt = document.createElement('option');
            opt.value = t.id;
            opt.textContent = t.name + (t.is_vocal ? ' 🎤' : '') + ' (' + (t.note_count || 0) + ' 音符)';
            select.appendChild(opt);
        }
        if (select.options.length > 1) {
            select.selectedIndex = 1;
        }
    },

    renderTree(structures, trackId) {
        const container = document.getElementById('structure-tree');
        const copyBtn = document.getElementById('copy-structures-btn');

        if (!structures || structures.length === 0) {
            container.innerHTML = '<div style="text-align: center; color: #999; padding: 20px;">尚無段落結構</div>';
            if (copyBtn) copyBtn.style.display = trackId ? 'inline-block' : 'none';
            return;
        }

        if (copyBtn) copyBtn.style.display = trackId ? 'inline-block' : 'none';

        let html = '';
        for (const s of structures) {
            html += `
                <div class="tree-item">
                    <div class="tree-section">
                        <div class="tree-section-header">
                            <span class="tree-section-title">🎵 ${escapeHtml(s.title)}</span>
                            <div>
                                <button class="btn btn-sm btn-play" data-action="play" data-start="${s.start_time}" data-end="${s.end_time}" title="試聽段落">▶️</button>
                                <button class="btn btn-secondary btn-sm" data-action="edit" data-id="${s.id}" data-type="SECTION">編輯</button>
                                <button class="btn btn-danger btn-sm" data-action="delete" data-id="${s.id}" data-title="${escapeHtml(s.title)}">刪除</button>
                            </div>
                        </div>
                        <div class="tree-section-time">${formatTime(s.start_time)} - ${formatTime(s.end_time)} (${formatDuration(s.start_time, s.end_time)})</div>
            `;

            const sortedPhrases = (s.phrases || []).slice().sort((a, b) => a.start_time - b.start_time);
            const phraseCount = sortedPhrases.length;

            if (phraseCount > 0) {
                for (let i = 0; i < phraseCount; i++) {
                    const p = sortedPhrases[i];
                    const isLast = (i === phraseCount - 1);
                    const lyricsVal = p.lyrics || '';
                    html += `
                        <div class="tree-phrase">
                            <div style="flex:1;">
                                <div>
                                    <span class="tree-phrase-title">📝 ${escapeHtml(p.title)}</span>
                                    <span class="tree-phrase-time"> ${formatTime(p.start_time)} - ${formatTime(p.end_time)}</span>
                                    <button class="btn btn-sm btn-play" data-action="play" data-start="${p.start_time}" data-end="${p.end_time}" title="試聽句子">▶️</button>
                                </div>
                                <div class="lyrics-row">
                                    <input class="lyrics-input" type="text" id="lyrics-${p.id}" placeholder="輸入歌詞..." value="${escapeHtml(lyricsVal)}">
                                </div>
                            </div>
                            <div><button class="btn btn-secondary btn-sm" data-action="edit" data-id="${p.id}" data-type="PHRASE">編輯</button></div>
                            ${isLast ? `<div><button class="btn btn-sm btn-primary" data-action="save-lyrics" data-section-id="${s.id}" title="儲存此段落所有歌詞">💾 儲存此段落歌詞</button></div>` : ''}
                        </div>
                    `;
                }
            }

            html += `
                        <div class="add-phrase-row">
                            <button class="btn btn-sm btn-secondary" data-action="add-phrase" data-section-id="${s.id}">+ 新增句子</button>
                        </div>
                    </div>
                </div>
            `;
        }
        container.innerHTML = html;
    },

    hideEditPanel() {
        document.getElementById('edit-panel').style.display = 'none';
    }
};

// --- UIStatus ---
export const UIStatus = {
    updateWaveformStatus(message, isError) {
        const statusEl = document.getElementById('waveform-placeholder');
        if (!statusEl) return;
        statusEl.textContent = message;
        statusEl.style.color = isError ? '#e74c3c' : '';
    },

    setPlayButton(btn, isPlaying) {
        if (!btn) return;
        if (isPlaying) {
            btn.textContent = '⏹';
            btn.dataset.playing = 'true';
        } else {
            btn.textContent = '▶️';
            btn.dataset.playing = 'false';
        }
    },

    updateStatCounters(stats) {
        if (stats.sections !== undefined) {
            document.getElementById('stat-sections').textContent = stats.sections;
        }
        if (stats.phrases !== undefined) {
            document.getElementById('stat-phrases').textContent = stats.phrases;
        }
        if (stats.duration !== undefined) {
            document.getElementById('stat-duration').textContent = stats.duration;
        }
    },

    setImportResult(message, type) {
        const resultEl = document.getElementById('import-result');
        if (!resultEl) return;
        resultEl.textContent = message;
        resultEl.style.color = type === 'success' ? '#27ae60' : '#e74c3c';
    },

    resetDashboard() {
        document.getElementById('structure-tree').innerHTML = '<div style="text-align: center; color: #999; padding: 20px;">請先選擇歌曲</div>';
        document.getElementById('timeline').innerHTML = '';
        document.getElementById('waveform-placeholder').textContent = '📊 請先選擇歌曲';
        document.getElementById('stat-sections').textContent = '0';
        document.getElementById('stat-phrases').textContent = '0';
        document.getElementById('stat-duration').textContent = '0:00';
    },

    showLoading(message) {
        const placeholder = document.getElementById('waveform-placeholder');
        if (placeholder) placeholder.textContent = '⏳ ' + message;
    },

    getEditPanelData() {
        const title = document.getElementById('edit-title').value.trim();
        const type = document.getElementById('edit-type').value;
        const startTime = parseFloat(document.getElementById('edit-start').value) || 0;
        const endTime = parseFloat(document.getElementById('edit-end').value) || 0;
        const startTick = parseInt(document.getElementById('edit-start-tick').value) || 0;
        const endTick = parseInt(document.getElementById('edit-end-tick').value) || 0;
        const orderIdx = parseInt(document.getElementById('edit-order').value) || 0;
        const parentId = document.getElementById('edit-parent').value;
        const lyrics = document.getElementById('edit-lyrics').value.trim();
        const trackId = document.getElementById('lyrics-track-select').value;
        return { title, type, startTime, endTime, startTick, endTick, orderIdx, parentId, lyrics, trackId };
    }
};

// --- 內部工具函式（不 export）---
function formatTime(sec) {
    if (sec == null) return '0:00';
    const m = Math.floor(sec / 60);
    const s = Math.floor(sec % 60);
    return m + ':' + s.toString().padStart(2, '0');
}

function formatDuration(start, end) {
    if (start == null || end == null) return '';
    const d = end - start;
    if (d < 60) return d.toFixed(0) + '秒';
    return (d / 60).toFixed(1) + '分';
}
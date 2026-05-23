export const StructureRenderer ={
    renderTimeLine(structures){
        // Implementation for rendering the time line based on structure data
    },
    renderTree(structures,trackId){  //structures is normolized with phrases nested inside sections
       const container = document.getElementById('structure-tree');
        if (!structures || structures.length === 0) {
            container.innerHTML = '<div class="empty-msg">尚無段落結構</div>'; 
            return;
        }

        let html = '';
        structures.forEach(s => {
            // 1. 產生 Section HTML
            html += `<div class="section-item">
                        <span>${s.title}</span>
                        <button onclick="togglePlay(${s.start_time}, ${s.end_time}, 'btn-${s.id}')">▶️</button>
                     </div>`;

            // 2. 排序並產生 Phrases HTML
            const sortedPhrases = (s.phrases || []).sort((a, b) => a.start_time - b.start_time);
            sortedPhrases.forEach(p => {
                html += `<div class="phrase-item" style="margin-left: 20px;">
                            <span>${p.title}</span>
                            <input id="lyrics-${p.id}" value="${p.lyrics || ''}">
                         </div>`;
            });

            // 3. 加上新增按鈕
            html += `<button onclick="showAddPhrase('${s.id}')">+ 新增句子</button>`;
        });

        container.innerHTML = html; // 統一更新 DOM [10]
    
    },
    renderSongsDropdown(songs){
        // Implementation for loading songs into the dropdown menu
    },
    renderTracksForLyrics(song){
        // Implementation for loading tracks for lyrics editing interface
    },
    hideEditPanel(){
       document.getElementById('edit-panel').style.display = 'none'
    },
    renderTrackSelector(tracks){
        
    }
}
//負責管理單一元件的視覺狀態與即時反饋，確保主程式不需要直接操作style or textContent 
export const UIStatus = {
    updateWaveformStatus(message, isError){
        const statusEl = document.getElementById('waveform-status');
        statusEl.textContent = message;
        statusEl.style.color = isError ? '#e74c3c' : '#2ecc71';
    },
    setPlayButton(btn, isPlaying){
        if (isPlaying) {
            btn.textContent = '停止';
            btn.dataset.playing = 'true';
            btn.style.backgroundColor = '#e74c3c';
        } else {
            btn.textContent = '播放';
            btn.dataset.playing = 'false';
            btn.style.backgroundColor = '';
        }
    },
    updateStateCounters(stats){
        // Implementation for updating any counters or indicators in the UI based on the current state of the application 
    },
    setImportResult(message, isError){
        const resultEl = document.getElementById('import-result');
        resultEl.textContent = message;
        resultEl.style.color = isError ? '#e74c3c' : '#2ecc71';
    },
    resetDashboard(){}, //todo:   清空統計與 UI     
    updateState(data){}
}
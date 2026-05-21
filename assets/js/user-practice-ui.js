function updatePlayBtn() {
    const btn = document.getElementById("record-btn");
    if (!btn) return;
    if (isPlaying) {
        btn.innerHTML = 
            `<svg viewBox="0 0 24 24" width="30" height="30">
                <rect x="6" y="6" width="5" height="12" fill="white"/>
                <rect x="13" y="6" width="5" height="12" fill="white"/>
            </svg>`;
    } else {
        btn.innerHTML = 
            `<svg viewBox="0 0 24 24" width="30" height="30">
                <polygon points="8,5 19,12 8,19" fill="white"/>
            </svg>`;
    }
}

function togglePlayback() {
    if (isPlaying) {
        stopPlayback();
        return;
    }
    if (selectedStructure) {
        playRange(selectedStructure.start, selectedStructure.end);
    } else {
        alert("請先選擇一個段落或句子");
    }
}

function renderVocalTracks(tracks) {
    const container = document.getElementById("vocal-tracks");
    if (!tracks || tracks.length === 0) {
        container.innerHTML = `<div class="no-structures">此歌曲沒有音軌</div>`;
        return;
    }

    let html = "";
    let hasVocal = false; // 💡 用來記錄是否有成功渲染任何一條音軌

    for (const t of tracks) {
        if (!t.is_vocal) continue; // 只顯示標記為主聲部的音軌選項
        hasVocal = true; // 只要有一條是主聲部，就算成功渲染了主聲部選項
        const isSelected = t.id === selectedTrackId;
        const badgeText = t.is_vocal ? "🎤 主聲部" : "🎵 伴唱聲部";
        html += `
            <div class="track-option ${isSelected ? "selected" : ""}" onclick="selectTrack("${t.id}")">
                <input type="radio" name="vocal-track" ${isSelected ? "checked" : ""}>
                <span class="track-name">${t.name}</span>
                <span class="track-badge has-structure">${badgeText}</span>
            </div>
        `;
    }
    if (!hasVocal) {
        container.innerHTML = `<div class="no-structures">此歌曲沒有主聲部音軌</div>`;
        return;
    }
    container.innerHTML = html;
}

function renderAccompanimentTracks(tracks) {
    const container = document.getElementById("accompaniment-tracks");
    if (!tracks || tracks.length === 0) {
        container.innerHTML = `<div class="no-structures">此歌曲沒有音軌</div>`;
        return;
    }

    let html = "";
    for (const t of tracks) {
        const isChecked = selectedAccompanimentTrackIds.includes(t.id);
        const badgeText = t.is_vocal ? "🎤 主聲部" : "🎵 伴唱聲部";
        // Exclude the currently selected vocal track from accompaniment options
        const isDisabled = t.id === selectedTrackId;
        html += `
            <div class="track-option ${isChecked ? "selected" : ""}" onclick="${isDisabled ? "" : `toggleAccompanimentTrack("${t.id}")`}" style="${isDisabled ? "opacity:0.5;cursor:not-allowed;" : ""}">
                <input type="checkbox" ${isChecked ? "checked" : ""} ${isDisabled ? "disabled" : ""} style="margin-right:12px;">
                <span class="track-name">${t.name}</span>
                <span class="track-badge has-structure">${badgeText}</span>
            </div>
        `;
    }
    container.innerHTML = html;
}

function renderStructureList() {
    const container = document.getElementById("structure-list");
    if (!structures || structures.length === 0) {
        container.innerHTML = `<div class="no-structures">此音軌尚無段落結構<br><small>請管理員在後台設定段落</small></div>`;
        return;
    }

    let html = "";
    for (const s of structures) {
        const isSelected = selectedStructure && selectedStructure.id === s.id;
        html += `
            <div class="structure-option ${isSelected ? "selected" : ""}" onclick="selectStructure("${s.id}")">
                <span class="structure-icon">📂</span>
                <span class="structure-name">${s.title}</span>
                <span class="structure-time">${formatTime(s.start_time)} - ${formatTime(s.end_time)}</span>
                <button class="btn-play" onclick="event.stopPropagation();playRange(${s.start_time},${s.end_time})">▶️</button>
            </div>
        `;
        if (s.phrases) {
            for (const p of s.phrases) {
                const isPhraseSelected = selectedStructure && selectedStructure.id === p.id;
                html += `
                    <div class="structure-option phrased ${isPhraseSelected ? "selected" : ""}" onclick="selectStructure("${p.id}")">
                        <span class="structure-icon">📄</span>
                        <span class="structure-name">${p.title} ${p.lyrics ? "🎤" : ""}</span>
                        <span class="structure-time">${formatTime(p.start_time)} - ${p.lyrics ? "" : formatTime(p.end_time)}</span>
                        <button class="btn-play" onclick="event.stopPropagation();playRange(${p.start_time},${p.end_time})">▶️</button>
                    </div>
                `;
            }
        }
    }
    container.innerHTML = html;
}

function updateSelectionUI() {
    if (selectedStructure) {
        document.getElementById("range-info").textContent =
            `🎯 ${selectedStructure.title}：${formatTime(selectedStructure.start)} - ${formatTime(selectedStructure.end)}`;
        document.getElementById("range-actions").style.display = "flex";
        document.getElementById("current-range-label").textContent =
            `${selectedStructure.title} (${formatTime(selectedStructure.start)})`;
        renderStructureList(); // re-render to show selection
    }
}

function clearSelection() {
    selectedStructure = null;
    document.getElementById("range-info").textContent = "選擇一個段落或句子開始練習";
    document.getElementById("range-actions").style.display = "none";
    document.getElementById("current-range-label").textContent = "未選擇";
    renderStructureList();
}

function switchTab(el, tab) {
    document.querySelectorAll(".viz-tab").forEach(t => t.classList.remove("active"));
    el.classList.add("active");
    // Placeholder - would switch visualization types
}

function logout() {
    document.cookie = "token=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;";
    window.location.href = "/index.html";
}

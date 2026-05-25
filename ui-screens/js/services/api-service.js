const API_BASE = '/api/v1/admin';
const USER_API_BASE = '/api/v1';

export const SongService={
    //api(path,options){},
// 基礎 API 呼叫器 (改進自原始 api helper [1])
    async _request(path, options = {}) {
        const token = localStorage.getItem('token');
        const headers = options.body instanceof FormData ? {} : { 
            'Content-Type': 'application/json', 
            ...options.headers 
        };
        if (token) headers['Authorization'] = `Bearer ${token}`;

        // 自動判斷是否為管理端路徑 [1]
        const base = path.startsWith('/admin') ? API_BASE : USER_API_BASE;
        const finalPath = path.startsWith('/admin') ? path.replace('/admin', '') : path;

        const res = await fetch(base + finalPath, { 
            credentials: 'same-origin', 
            headers, 
            ...options 
        });

        if (!res.ok) {
            const err = await res.json().catch(() => ({ error: res.statusText }));
            const error = new Error(err.error || `HTTP ${res.status}`);
            error.status = res.status;
            error.data = err;
            throw error; // 丟給 UIErrorHandler 處理
        }
        return res.json();
    },

    // --- 讀取操作 ---
    
    /** 獲取所有歌曲清單 */
    getSongs() {
        return this._request('/songs');
    },

    /** 獲取歌曲詳細資訊 (包含聲部、速度映射與 PPQ) */
    getSongDetail(songId) {
        return this._request(`/songs/${songId}`);
    },

    /** 獲取特定的段落結構 */
    getStructures(songId, trackId) {
        let path = `/songs/${songId}/structures`;
        if (trackId) path += `?track_id=${encodeURIComponent(trackId)}`;
        return this._request(path);
    },

    /** getMidi — 別名，與 getMIDIData 相同 */
    getMidi(songId) {
        return this.getMIDIData(songId);
    },

    /** 獲取 MIDI 二進制資料 */
    async getMIDIData(songId) {
        const token = localStorage.getItem('token');
        const headers = token ? { 'Authorization': `Bearer ${token}` } : {};
        const res = await fetch(`/api/v1/songs/${songId}/midi`, { 
            credentials: 'same-origin',
            headers 
        });
        if (!res.ok) throw new Error('MIDI 載入失敗');
        return res.arrayBuffer();
    },

    // --- 寫入/更新/刪除操作 (CRUD) ---

    /** 建立新的結構 (支援批量或從其他聲部複製)  */
    createStructures(songId, payload) {
        return this._request(`/admin/songs/${songId}/structures`, {
            method: 'POST',
            body: JSON.stringify(payload)
        });
    },

    /** 更新特定結構內容 */
    updateStructure(songId, structureId, payload) {
        return this._request(`/admin/songs/${songId}/structures/${structureId}`, {
            method: 'PUT',
            body: JSON.stringify(payload)
        });
    },

    /** 刪除結構節點 */
    deleteStructure(songId, structureId) {
        return this._request(`/admin/songs/${songId}/structures/${structureId}`, {
            method: 'DELETE'
        });
    },

    /** 儲存歌詞 (支援單句或段落批次) */
    saveLyrics(songId, lyricsPayload) {
        return this._request(`/admin/songs/${songId}/lyrics`, {
            method: 'POST',
            body: JSON.stringify({ lyrics: lyricsPayload })
        });
    },

    // --- 匯入/匯出操作 ---

    /** 匯出 CSV 資料 */
    exportCSV(songId) {
        return this._request(`/admin/songs/${songId}/structures/export`);
    },

    /** 匯入 CSV 並處理衝突  */
    importCSV(songId, csvText, force = true) {
        return this._request(`/admin/songs/${songId}/structures/import?force=${force}`, {
            method: 'POST',
            headers: { 'Content-Type': 'text/csv' },
            body: csvText
        });
    },

    // --- 版本管理 ---

    /** 上傳 MIDI 建立新歌曲（可帶 source_song_id 做為新增版本） */
    uploadSong(title, artist, file, sourceSongId = null) {
        const formData = new FormData();
        formData.append('title', title);
        formData.append('artist', artist);
        formData.append('midi_file', file);
        if (sourceSongId) {
            formData.append('source_song_id', sourceSongId);
        }
        return this._request('/songs', {
            method: 'POST',
            body: formData,
        });
    },

    /** 上傳新版本 MIDI（基於現有歌曲） */
    createNewVersion(songId, file) {
        const formData = new FormData();
        formData.append('midi_file', file);
        return this._request(`/admin/songs/${songId}/new-version`, {
            method: 'POST',
            body: formData,
        });
    },

    /** 設為活躍版本（user 看到的就是這個版本） */
    setActiveVersion(songId) {
        return this._request(`/admin/songs/${songId}/set-active`, {
            method: 'PUT',
        });
    },

    /** 取得同一首歌的所有版本 */
    getVersions(songId) {
        return this._request(`/admin/songs/${songId}/versions`);
    }
}

# Function Map — Vocal Practice 前端模組 (JS)

> Review 時只需查閱對應模組的函式清單。
> 每個函式標註：所屬模組、參數、回傳值、是否對外暴露、依賴關係。

---

## A. 使用者練習模組 (`assets/js/`)

---

## 1. `midiParser.js`（window 全域，非 ES Module）

| 暴露名稱 | 實際函式 | 參數 | 回傳值 | 說明 |
|----------|----------|------|--------|------|
| `window.MidiParser.parseMIDINotes` | `parseMIDINotes` | `midiArrayBuf: ArrayBuffer` | `Array<{pitch, start, dur, track, velocity?}>` | 解析 MIDI 二進位資料，回傳所有音符（含時間、音高、軌道、力度） |
| `window.MidiParser.midiPitchToFreq` | `midiPitchToFreq` | `pitch: number` | `number` | MIDI pitch → 頻率 (Hz) |

**依賴：** 無  
**使用端：** `audio.js`, `practice-business.js`, `ui-screens/js/adapters/audio-adapter.js`

---

## 2. `songDataExtractor.js`（window 全域，非 ES Module）

| 暴露名稱 | 實際函式 | 參數 | 回傳值 | 說明 |
|----------|----------|------|--------|------|
| `window.SongDataExtractor.extractReferenceNotes` | `extractReferenceNotes` | `allNotes, selectedTrackIndex, rangeStart, rangeEnd` | `Array<{pitch, start, dur}>` | 從 parsed notes 中過濾出指定音軌 + 時間範圍的參考音符 |

**依賴：** 無  
**使用端：** `practice-business.js`

---

## 3. `state.js`（ES Module）

| 暴露名稱 | 型態 | 說明 |
|----------|------|------|
| `default (state)` | `Object` | 集中式狀態物件，所有模組直接讀寫此物件 |

**state 屬性一覽：**

| 屬性 | 型態 | 預設值 | 說明 |
|------|------|--------|------|
| `currentSongId` | `string \| null` | `null` | 當前選取的歌曲 ID |
| `currentSongFull` | `Object \| null` | `null` | 完整歌曲資料（含 tracks, tempo_map） |
| `structures` | `Array` | `[]` | 段落結構列表 |
| `selectedTrackId` | `string \| null` | `null` | 選取的聲部 UUID |
| `selectedAccompanimentTrackIds` | `string[]` | `[]` | 選取的伴唱聲部 UUID 列表 |
| `selectedStructure` | `Object \| null` | `null` | `{ id, start, end, title }` |
| `midiData` | `ArrayBuffer \| null` | `null` | MIDI 原始資料 |
| `parsedNotes` | `Array \| null` | `null` | MIDI 解析後的音符陣列 |
| `referenceNotes` | `Array \| null` | `null` | 過濾後的參考音符（僅選取音軌+區間） |
| `allTracksMeta` | `Array` | `[]` | 從後端取得的音軌 meta 資訊 |
| `parsedTrackIndex` | `number \| null` | `null` | 選取音軌在 parsedNotes 中的 index |
| `parsedMIDITracks` | `Array` | `[]` | MIDI 解析的軌道資訊 |

---

## 4. `api.js`（ES Module）

| 匯出名稱 | 參數 | 回傳值 | 說明 |
|----------|------|--------|------|
| `api` | `path: string, options?: Object` | `Promise<Object>` | 底層 fetch 封裝，自動帶 credentials + Content-Type |
| `formatTime` | `sec: number` | `string` | 秒數 → `"m:ss"` 格式 |
| `loadSongs` | 無 | `Promise<void>` | 從後端載入歌曲列表並填入 `<select>` |
| `loadMIDI` | `currentSongId: string` | `Promise<ArrayBuffer \| null>` | 下載 MIDI 檔 |
| `loadStructures` | `currentSongId: string, trackId: string` | `Promise<Array>` | 載入指定音軌的段落結構 |

**依賴：** 無（純 fetch）  
**使用端：** `practice-business.js`

---

## 5. `audio.js`（ES Module）

| 匯出名稱 | 參數 | 回傳值 | 說明 |
|----------|------|--------|------|
| `playRange` | `startTime: number, endTime: number` | `void` | 播放指定時間範圍的 MIDI 音符（僅選取音軌） |
| `stopPlayback` | 無 | `void` | 停止播放並清理節點 |
| `togglePlayback` | 無 | `void` | 切換播放/暫停 |
| `getIsPlaying` | 無 | `boolean` | 取得當前播放狀態 |
| `setUpdatePlayBtn` | `fn: Function` | `void` | 注入 UI 更新函式（迴避循環依賴） |

**內部狀態：** `audioCtx`, `playbackNodes`, `playbackTimer`, `isPlaying`（模組私有）  
**依賴：** `state.js`, `window.MidiParser.midiPitchToFreq`  
**使用端：** `practice-business.js`, `practice-ui.js`

---

## 6. `practice-business.js`（ES Module）

| 匯出名稱 | 參數 | 回傳值 | 說明 |
|----------|------|--------|------|
| `selectTrack` | `trackId: string` | `Promise<void>` | 選取聲部，載入對應結構 |
| `selectStructure` | `id: string` | `void` | 選取段落/句子，產生參考資料 |
| `onSongChange` | 無 | `Promise<void>` | 歌曲選擇變更處理（載入歌曲、MIDI、音軌、結構） |
| `toggleAccompanimentTrack` | `trackId: string` | `void` | 切換伴唱聲部選取 |
| `startPractice` | 無 | `void` | 開始練習（設定 UI + 自動播放） |

**內部函式（不匯出）：** `determineParsedTrackIndex()`, `generateReferenceData()`, `updateSelectionUI()`  
**依賴：** `state.js`, `api.js`, `audio.js`, `window.SongDataExtractor`, `practice-ui.js`（動態 import）  
**使用端：** `user-practice.html`（經 window 掛載）

---

## 7. `practice-ui.js`（ES Module）

| 匯出名稱 | 參數 | 回傳值 | 說明 |
|----------|------|--------|------|
| `updatePlayBtn` | 無 | `void` | 更新播放按鈕圖示（播放/暫停切換） |
| `renderVocalTracks` | `tracks: Array` | `void` | 渲染主聲部 radio list |
| `renderAccompanimentTracks` | `tracks: Array` | `void` | 渲染伴唱聲部 checkbox list |
| `renderStructureList` | 無 | `void` | 渲染段落結構列表 |
| `updateSelectionUI` | 無 | `void` | 更新選取範圍的 UI 顯示 |
| `clearSelection` | 無 | `void` | 清除當前選取 |
| `switchTab` | `el: Element, tab: string` | `void` | 切換視覺化分頁 |
| `logout` | 無 | `void` | 登出（清除 cookie + 跳轉） |

**副作用（module init）：** 呼叫 `setUpdatePlayBtn(updatePlayBtn)` 註冊播放按鈕更新  
**依賴：** `state.js`, `audio.js`  
**使用端：** `user-practice.html`（經 window 掛載），`practice-business.js`（動態 import）

---

## 8. `user-practice-business.js`（ES Module）

使用者練習擴充業務邏輯。

**依賴：** `state.js`, `api.js`  
**使用端：** `user-practice.html`

---

## 9. `user-practice-ui.js`（ES Module）

使用者練習擴充 UI 渲染。

**依賴：** `state.js`  
**使用端：** `user-practice.html`

---

## B. 管理者後台模組 (`ui-screens/js/`)

---

## 10. `admin.js`（ES Module，主入口）

| 函式 | 參數 | 回傳值 | 說明 |
|------|------|--------|------|
| `init()` | 無 | `Promise<void>` | 初始化：檢查 admin 角色、載入歌曲列表、處理 URL 參數 |
| `bindEventListeners()` | 無 | `void` | 綁定所有 DOM 事件監聽 |
| `setupTickAutoCalc()` | 無 | `void` | 設定 Tick 自動計算（start/end 時間輸入 → 自動換算 Tick） |
| `onSongChange()` | 無 | `Promise<void>` | 歌曲切換：載入詳細資訊、MIDI、聲部、結構 |
| `onTrackChange()` | 無 | `Promise<void>` | 聲部切換：載入該聲部的結構樹 |
| `loadMIDIData(songId)` | `songId: string` | `Promise<void>` | 獨立 MIDI 載入 |
| `handlePlayButtonClick(startTime, endTime, btn)` | `startTime, endTime, btnElement` | `void` | 播放/停止切換 |
| `findStructureById(id)` | `id: string` | `Object \| null` | 依 ID 查詢結構節點 |
| `handleEdit(id, type)` | `id, type` | `void` | 開啟編輯面板 |
| `showAddSection()` | 無 | `void` | 顯示新增段落面板 |
| `showAddPhrase(sectionId)` | `sectionId: string` | `void` | 顯示新增句子面板（自動算序號、時間） |
| `updateLyricsEditorVisibility(type)` | `type: string` | `void` | 切換歌詞編輯器可見性（僅 PHRASE 顯示） |
| `saveEdit()` | 無 | `Promise<void>` | 儲存編輯/新增（含 Tick 邊界驗證、歌詞儲存） |
| `cancelEdit()` | 無 | `void` | 取消編輯 |
| `handleDelete(id, title)` | `id, title` | `Promise<void>` | 刪除結構節點 |
| `handleSaveLyrics(sectionId)` | `sectionId: string` | `Promise<void>` | 批量儲存段落歌詞 |
| `handleExport()` | 無 | `Promise<void>` | 匯出 CSV |
| `handleImport(fileInput)` | `fileInput: Element` | `Promise<void>` | 匯入 CSV（含 409 衝突、聲部不符處理） |
| `handleCopyStructures()` | 無 | `Promise<void>` | 顯示跨聲部複製對話框 |
| `__copyStructuresFromTrack(sourceTrackId, sourceTrackName)` | `sourceTrackId, sourceTrackName` | `Promise<void>` | 執行跨聲部複製（window 全域供 dialog 按鈕呼叫） |

**內部狀態：** `currentSongId`, `currentTrackId`, `structures`, `editingNode`, `editingIsNew`, `activePlayBtn`  
**依賴：** `services/api-service.js`, `utils/error-handler.js`, `utils/songdata-converter.js`, `adapters/ui-renderer.js`, `adapters/audio-adapter.js`  
**使用端：** `admin-structures.html`（`<script type="module">`）

---

## 11. `services/api-service.js`（ES Module → SongService）

```javascript
export const SongService = { ... }
```

| 方法 | 參數 | 回傳值 | 說明 |
|------|------|--------|------|
| `_request(path, options)` | `path, options` | `Promise<Object>` | 底層 fetch 封裝，自動路徑判斷、Token 帶入、JSON/FormData 適配 |
| `getSongs()` | 無 | `Promise<Array>` | 獲取所有歌曲清單 |
| `getSongDetail(songId)` | `songId` | `Promise<Object>` | 獲取歌曲詳細資訊（含聲部、Tempo Map、PPQ） |
| `getStructures(songId, trackId)` | `songId, trackId?` | `Promise<Object>` | 獲取段落結構（可依聲部過濾） |
| `getMidi(songId)` | `songId` | `Promise<ArrayBuffer>` | getMIDIData 的別名 |
| `getMIDIData(songId)` | `songId` | `Promise<ArrayBuffer>` | 獲取 MIDI 二進制資料 |
| `createStructures(songId, payload)` | `songId, payload` | `Promise<Object>` | 建立新結構（支援批量或從其他聲部複製） |
| `updateStructure(songId, structureId, payload)` | `songId, structureId, payload` | `Promise<Object>` | 更新特定結構內容 |
| `deleteStructure(songId, structureId)` | `songId, structureId` | `Promise<Object>` | 刪除結構節點 |
| `saveLyrics(songId, lyricsPayload)` | `songId, lyricsPayload` | `Promise<Object>` | 儲存歌詞（支援單句或段落批次） |
| `exportCSV(songId)` | `songId` | `Promise<Object>` | 匯出 CSV 資料 |
| `importCSV(songId, csvText, force)` | `songId, csvText, force=true` | `Promise<Object>` | 匯入 CSV 並處理衝突 |
| `uploadSong(title, artist, file, sourceSongId?)` | `title, artist, file, sourceSongId?` | `Promise<Object>` | 上傳 MIDI 建立新歌曲（可帶 source_song_id 做為新增版本） |
| `createNewVersion(songId, file)` | `songId, file` | `Promise<Object>` | 上傳新版本 MIDI |
| `setActiveVersion(songId)` | `songId` | `Promise<Object>` | 設為活躍版本 |
| `getVersions(songId)` | `songId` | `Promise<Array>` | 取得同一首歌的所有版本 |

**依賴：** 無（純 fetch）  
**使用端：** `admin.js`

---

## 12. `utils/error-handler.js`（ES Module → UIErrorHandler）

```javascript
export const UIErrorHandler = { ... }
```

| 方法 | 參數 | 回傳值 | 說明 |
|------|------|--------|------|
| `notify(error)` | `error: Error` | `void` | 統一錯誤處理入口（401 → 跳轉登入, 409 → 詳細對話框, 其他 → Toast） |
| `showToast(message)` | `message: string` | `void` | 顯示簡短提示文字 |
| `handleConflict(data)` | `data: Object` | `void` | 處理 409 衝突 |
| `showDetailedModal(title, message)` | `title, message` | `void` | 顯示自訂詳細錯誤對話框 |
| `handleAuthError()` | 無 | `void` | 認證錯誤處理（跳轉登入頁） |

**依賴：** 無  
**使用端：** `admin.js`

---

## 13. `utils/songdata-converter.js`（ES Module → SongDataConverter）

```javascript
export const SongDataConverter = { ... }
```

| 方法 | 參數 | 回傳值 | 說明 |
|------|------|--------|------|
| `updateContext(song)` | `song: Object` | `void` | 更新 tempo map 與 PPQ 快取 |
| `toTick(timeSec)` | `timeSec: number` | `number` | 秒數 → MIDI Tick（支援 tempo map 變速） |
| `formatDuration(start, end)` | `start, end: number` | `string` | 時間差格式化（「30秒」/「2.5分」） |
| `formatTime(sec)` | `sec: number` | `string` | 秒數 → `m:ss` 格式 |
| `normalizeStructures(rawApiData)` | `rawApiData: Array` | `Array` | 將 API 結構資料正規化為巢狀樹狀結構 |
| `toSec(tick)` | `tick: number` | `number` | MIDI Tick → 秒數（支援 tempo map 變速） |

**內部狀態：** `internalTempoMap`, `internalPPQ`  
**依賴：** 無  
**使用端：** `admin.js`

---

## 14. `adapters/ui-renderer.js`（ES Module → StructureRenderer + UIStatus）

### StructureRenderer

| 方法 | 參數 | 回傳值 | 說明 |
|------|------|--------|------|
| `renderTimeline(structures)` | `structures: Array` | `void` | 時間軸視覺化（Section/Phrase 色塊） |
| `renderSongsDropdown(songs)` | `songs: Array` | `void` | 歌曲下拉選單 |
| `renderTrackSelector(tracks)` | `tracks: Array` | `void` | 聲部選擇器（含 is_vocal 標記與音符數） |
| `renderTree(structures, trackId)` | `structures, trackId` | `void` | 結構樹渲染（播放/編輯/刪除按鈕、歌詞輸入框） |
| `hideEditPanel()` | 無 | `void` | 隱藏編輯面板 |

### UIStatus

| 方法 | 參數 | 回傳值 | 說明 |
|------|------|--------|------|
| `updateWaveformStatus(message, isError)` | `message, isError` | `void` | 更新狀態文字 |
| `setPlayButton(btn, isPlaying)` | `btn, isPlaying` | `void` | 設定播放按鈕狀態 |
| `updateStatCounters(stats)` | `stats: Object` | `void` | 更新統計數字 |
| `setImportResult(message, type)` | `message, type` | `void` | 設定匯入結果顯示 |
| `resetDashboard()` | 無 | `void` | 重設儀表板 |
| `showLoading(message)` | `message: string` | `void` | 顯示載入中 |
| `getEditPanelData()` | 無 | `Object` | 取得編輯面板資料 |

**依賴：** 無  
**使用端：** `admin.js`

---

## 15. `adapters/audio-adapter.js`（ES Module → WebAudioAdapter）

```javascript
export const WebAudioAdapter = { ... }
```

| 方法 | 參數 | 回傳值 | 說明 |
|------|------|--------|------|
| `getInternalCtx()` | 無 | `AudioContext` | 取得或建立 AudioContext |
| `loadData(buffer)` | `buffer: ArrayBuffer` | `void` | 載入 MIDI 資料並解析音符 |
| `getNoteCount()` | 無 | `number` | 取得載入的音符數 |
| `play(startTime, endTime)` | `startTime, endTime: number` | `Promise<void>` | 播放指定時間範圍（三角波 + 低通濾波 + ADSR 封包） |
| `stop()` | 無 | `void` | 停止播放並清理所有節點 |

**內部屬性：** `audioCtx`, `playbackNodes`, `playbackTimer`, `internalParsedNotes`, `onStopCallback`  
**依賴：** `window.MidiParser.parseMIDINotes`, `window.MidiParser.midiPitchToFreq`  
**使用端：** `admin.js`

---

## 依賴關係圖

### 使用者練習 (`user-practice.html`)

```
user-practice.html (type="module" entry)
  ├── midiParser.js              (window global, 非 module)
  ├── songDataExtractor.js       (window global, 非 module)
  ├── state.js                   ─┐
  ├── api.js                     ─┤
  ├── audio.js ─── state.js ──────┤
  ├── practice-business.js        │
  │   ├── state.js ───────────────┘
  │   ├── api.js
  │   ├── audio.js
  │   └── practice-ui.js (dynamic import)
  ├── practice-ui.js
  │   ├── state.js
  │   └── audio.js
  ├── user-practice-business.js
  └── user-practice-ui.js
```

### 管理者後台 (`admin-structures.html`)

```
admin-structures.html (type="module" entry)
  └── admin.js
      ├── services/api-service.js
      ├── utils/error-handler.js
      ├── utils/songdata-converter.js
      ├── adapters/ui-renderer.js
      └── adapters/audio-adapter.js
          └── window.MidiParser (midiParser.js)
```

---

## Review 快速指南

當 review 某個功能的程式碼時：

| 想 review 的範圍 | 只需要看 |
|------------------|----------|
| **使用者練習** | |
| MIDI 解析邏輯 | `assets/js/midiParser.js` |
| API 請求處理 | `assets/js/api.js` |
| 音訊播放邏輯 | `assets/js/audio.js` |
| 業務流程（選歌、選段、練習） | `assets/js/practice-business.js` |
| DOM 渲染與 UI 更新 | `assets/js/practice-ui.js` |
| 狀態定義與初始值 | `assets/js/state.js` |
| **管理者後台** | |
| 結構 CRUD、CSV、播放、版本管理 | `ui-screens/js/admin.js` |
| API 服務層（所有端點封裝） | `ui-screens/js/services/api-service.js` |
| 統一錯誤處理 | `ui-screens/js/utils/error-handler.js` |
| Tick↔Sec 轉換、結構正規化 | `ui-screens/js/utils/songdata-converter.js` |
| 結構樹/時間軸/聲部選擇器渲染 | `ui-screens/js/adapters/ui-renderer.js` |
| MIDI 範圍播放音訊 | `ui-screens/js/adapters/audio-adapter.js` |
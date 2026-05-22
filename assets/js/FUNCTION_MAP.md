# Function Map — Vocal Practice 前端模組 (JS)

> Review 時只需查閱對應模組的函式清單。
> 每個函式標註：所屬模組、參數、回傳值、是否對外暴露、依賴關係。

---

## 1. `midiParser.js`（window 全域，非 ES Module）

| 暴露名稱 | 實際函式 | 參數 | 回傳值 | 說明 |
|----------|----------|------|--------|------|
| `window.MidiParser.parseMIDINotes` | `parseMIDINotes` | `midiArrayBuf: ArrayBuffer` | `Array<{pitch, start, dur, track}>` | 解析 MIDI 二進位資料，回傳所有音符（含時間、音高、軌道） |
| `window.MidiParser.midiPitchToFreq` | `midiPitchToFreq` | `pitch: number` | `number` | MIDI pitch → 頻率 (Hz) |

**依賴：** 無  
**使用端：** `audio.js`, `practice-business.js`

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

## 依賴關係圖

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
  └── practice-ui.js
      ├── state.js
      └── audio.js
```

---

## Review 快速指南

當 review 某個功能的程式碼時：

| 想 review 的範圍 | 只需要看 |
|------------------|----------|
| MIDI 解析邏輯 | `midiParser.js` |
| API 請求處理 | `api.js` |
| 音訊播放邏輯 | `audio.js` |
| 業務流程（選歌、選段、練習） | `practice-business.js` |
| DOM 渲染與 UI 更新 | `practice-ui.js` |
| 狀態定義與初始值 | `state.js` |
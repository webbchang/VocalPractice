# Vocal Practice App - UI Screens

本目錄包含 Vocal Practice App 的所有 UI 介面螢幕和範例結果模板。

## 目錄結構

```
ui-screens/
├── js/                          # JavaScript ES Modules
│   ├── admin.js                 # 管理者主入口（結構 CRUD、CSV、播放、版本管理）
│   ├── services/
│   │   └── api-service.js       # API 服務層（SongService，含管理/使用者路徑判斷）
│   ├── utils/
│   │   ├── error-handler.js     # 統一錯誤處理（UIErrorHandler，含 401/409 處理）
│   │   └── songdata-converter.js # 資料轉換（SongDataConverter，tick↔sec、時間格式化）
│   ├── adapters/
│   │   ├── ui-renderer.js       # UI 渲染（StructureRenderer、UIStatus）
│   │   └── audio-adapter.js     # WebAudio 適配器（WebAudioAdapter，MIDI 範圍播放）
│   └── __tests__/               # 單元測試
│       ├── api-service.test.js
│       ├── audio-adapter.test.js
│       ├── error-handler.test.js
│       ├── songdata-converter.test.js
│       └── ui-renderer.test.js
├── admin-common.css           # 管理者後台共用樣式
├── admin-dashboard.html       # 管理者儀表板（角色權限檢查）
├── admin-users.html           # 管理者後台 - 使用者管理
├── admin-users.css            # 使用者管理樣式
├── admin-songs.html           # 管理者後台 - 歌曲管理（含版本管理）
├── admin-songs.css            # 歌曲管理樣式
├── admin-structures.html      # 管理者後台 - 段落結構管理
├── admin-structures.css       # 段落結構管理樣式
├── admin-dashboard.html       # 管理者儀表板
├── user-practice.html         # 使用者練習介面
├── user-practice.css          # 使用者練習介面樣式
├── user-dashboard.html        # 使用者儀表板
├── sample-results/            # 評分結果範例模板
│   ├── assessment-excellent.json        # 優秀評分 (90+ 分)
│   ├── assessment-good.json             # 良好評分 (80-89 分)
│   ├── assessment-average.json          # 普通評分 (70-79 分)
│   └── assessment-needs-improvement.json # 需改進評分 (<70 分)
└── README.md                  # 本說明文件
```

## JavaScript 模組架構

### 模組依賴關係

```
admin-structures.html (type="module")
  └── admin.js
      ├── services/api-service.js  ── 所有 API 請求
      ├── utils/error-handler.js    ── 錯誤處理
      ├── utils/songdata-converter.js ── 資料轉換
      ├── adapters/ui-renderer.js   ── UI 渲染
      └── adapters/audio-adapter.js ── 音訊播放（依賴 window.MidiParser）
```

### 模組說明

#### 1. admin.js - 管理者主入口
- **結構 CRUD**：新增/編輯/刪除 Section 與 Phrase，含父子關係驗證
- **段落結構管理**：跨聲部複製結構、時間軸視覺化互動
- **CSV 匯入/匯出**：含衝突偵測與 409 處理、聲部不符提示
- **播放控制**：MIDI 範圍試聽，支援播放/停止切換
- **版本管理**：上傳新版本、設定活躍版本、檢視版本列表
- **歌詞管理**：僅 PHRASE 層級儲存歌詞，SECTION 隱藏歌詞編輯器

#### 2. services/api-service.js (SongService)
- 封裝所有 Admin/User API 呼叫
- 自動判斷管理端路徑（`/admin/songs` → `/api/v1/admin/songs`）
- 自動帶入 JWT Token（從 localStorage 讀取）
- FormData 與 JSON 請求自動適配
- MIDI 二進位資料下載（`getMIDIData` 回傳 ArrayBuffer）
- 支援錯誤拋出（含 HTTP status），供 UIErrorHandler 處理
- **重要端點**：
  - `getSongs()`, `getSongDetail(songId)`
  - `getStructures(songId, trackId)` — 可依聲部過濾
  - `getMIDIData(songId)`, `getMidi(songId)`
  - `createStructures(songId, payload)`
  - `updateStructure(songId, structureId, payload)`
  - `deleteStructure(songId, structureId)`
  - `saveLyrics(songId, lyricsPayload)`
  - `exportCSV(songId)`, `importCSV(songId, csvText, force)`
  - `uploadSong(title, artist, file, sourceSongId)` — 含版本管理
  - `createNewVersion(songId, file)`, `setActiveVersion(songId)`, `getVersions(songId)`

#### 3. utils/error-handler.js (UIErrorHandler)
- `notify(error)` — 統一錯誤處理入口
  - 401 → 自動重新導向登入頁
  - 409 → 顯示詳細衝突對話框
  - 其他 → Toast 訊息顯示
- `showDetailedModal(title, message)` — 自訂錯誤對話框
- `handleAuthError()` — 認證錯誤處理

#### 4. utils/songdata-converter.js (SongDataConverter)
- `updateContext(song)` — 更新 tempo map 與 PPQ 快取
- `toTick(timeSec)` — 秒數 → MIDI Tick（支援 tempo map 變速）
- `toSec(tick)` — MIDI Tick → 秒數
- `formatDuration(start, end)` — 時間差格式化（「30秒」/「2.5分」）
- `formatTime(sec)` — 秒數 → `m:ss` 格式
- `normalizeStructures(rawApiData)` — 將 API 結構資料正規化為巢狀樹狀結構（自動偵測 flat/nested 兩種格式）

#### 5. adapters/ui-renderer.js (StructureRenderer + UIStatus)
- `StructureRenderer`：
  - `renderTimeline(structures)` — 時間軸視覺化（Section/Phrase 色塊）
  - `renderSongsDropdown(songs)` — 歌曲下拉選單
  - `renderTrackSelector(tracks)` — 聲部選擇器（含 is_vocal 標記）
  - `renderTree(structures, trackId)` — 結構樹渲染（含播放/編輯/刪除按鈕、歌詞輸入框）
  - `hideEditPanel()` — 隱藏編輯面板
- `UIStatus`：
  - `updateWaveformStatus()` / `setPlayButton()` / `updateStatCounters()`
  - `setImportResult()` / `resetDashboard()` / `showLoading()`
  - `getEditPanelData()` — 取得編輯面板資料

#### 6. adapters/audio-adapter.js (WebAudioAdapter)
- `loadData(buffer)` — 載入 MIDI ArrayBuffer 並解析音符
- `getNoteCount()` — 取得載入的音符數
- `play(startTime, endTime)` — 播放指定時間範圍（三角波 + 低通濾波 + ADSR 封包）
- `stop()` — 停止播放並清理所有音訊節點
- `onStopCallback` — 播放停止回呼（用於按鈕狀態重置）

## 管理者後台模組 (Admin Modules)

### 1. 管理者儀表板 (admin-dashboard.html)
- **功能**: 角色權限檢查、模組入口導航
- **特色**:
  - 檢查 localStorage 中 token 與 user.role === 'admin'
  - 若無權限自動跳轉登入頁
  - 三張功能卡片導向使用者管理、歌曲管理、段落結構

### 2. 使用者管理 (admin-users.html)
- **功能**: 建立、查看、管理使用者帳號
- **特色**:
  - 新增使用者表單（使用者名稱、電子郵件、密碼）
  - 使用者列表表格顯示
  - 啟用/停用使用者切換
  - 刪除使用者功能

### 3. 歌曲管理 (admin-songs.html)
- **功能**: 上傳 MIDI 檔案、管理歌曲清單、版本管理
- **特色**:
  - 拖曳上傳 MIDI 檔案（支援版本管理，可基於現有歌曲上傳新版本）
  - 自動解析 MIDI 多聲部 Track，根據名稱自動標記 is_vocal
  - 歌曲卡片式展示
  - 顯示各聲部資訊（主旋律標記、音符數、時長）
  - 檢視歌曲詳情時，每個 Track 旁有「設為主旋律/設為伴奏」切換按鈕
  - 版本切換與活躍版本管理
  - 刪除歌曲功能
- **API 整合**: PATCH /api/v1/admin/songs/{song_id}/tracks/{track_id} (修改 is_vocal)

### 4. 段落結構管理 (admin-structures.html)
- **功能**: 定義和編輯歌曲段落結構
- **特色**:
  - 時間軸視覺化預覽（Section 與 Phrase 色塊點擊可編輯）
  - Section（段落）和 Phrase（樂句）階層管理
  - 樹狀結構清單（含播放/編輯/刪除按鈕）
  - 編輯面板（時間、Tick、排序、父段落選擇）
  - Tick 自動計算（根據 tempo map 即時換算）
  - 跨聲部複製結構（從其他聲部複製段落框架）
  - CSV 匯入/匯出（含衝突偵測、聲部驗證）
  - 歌詞管理（僅 PHRASE 層級顯示歌詞編輯器）
- **樣式**: admin-structures.css, admin-common.css

## 使用者模組 (User Modules)

### 練習介面 (user-practice.html)
- **功能**: 完整的練習流程介面
- **特色**:
  - 左側邊欄：歌曲資訊、聲部選擇、段落選擇（含歌詞）
  - 前端下載完整 MIDI 檔案後，從中萃取所選 Track 的音符作為參考資料
  - 視覺化區域：四種圖表切換（音高曲線、偏差圖、時長比較、鋼琴捲軸）
  - 控制區域：錄音按鈕、計時器、狀態顯示
  - 結果面板：分數圓環、統計數據、上傳/重試按鈕
  - 歷史記錄：最近練習記錄列表

### 使用者儀表板 (user-dashboard.html)
- **功能**: 使用者練習總覽與統計
- **特色**:
  - 練習次數與平均分數統計
  - 最近練習記錄
  - 個人進度追蹤

## 評分結果模板 (Sample Results)

### JSON 結構說明

每個評分結果 JSON 包含以下欄位：

```json
{
  "assessment_id": "UUID",           // 評估記錄 ID
  "user_id": "UUID",                 // 使用者 ID
  "song_id": "UUID",                 // 歌曲 ID
  "track_id": "UUID",                // 聲部 ID
  "structure_id": "UUID or null",    // 段落 ID（null 表示全曲）
  "score": 85.5,                     // 總分 (0-100)
  "total_notes": 50,                 // 總音符數
  "matched_notes": 42,               // 匹配音符數
  "average_pitch_deviation": 15.3,   // 平均音高偏差 (cents)
  "average_duration_deviation": 0.08, // 平均時長偏差 (秒)
  "pitch_deviation": [...],          // 逐音符音高偏差陣列
  "duration_deviation": [...],       // 逐音符時長偏差陣列
  "note_comparison": [...],          // 逐音符比較詳細資料
  "segment_scores": {...},           // 各段落分數
  "metadata": {...},                 // 歌曲和段落元資料
  "created_at": "2024-01-15T14:32:18Z" // 建立時間
}
```

### 評分等級

| 等級 | 分數範圍 | 顏色標記 | 檔案 |
|------|----------|----------|------|
| 優秀 (Excellent) | 90-100 | 綠色 | `assessment-excellent.json` |
| 良好 (Good) | 80-89 | 黃綠色 | `assessment-good.json` |
| 普通 (Average) | 70-79 | 橙色 | `assessment-average.json` |
| 需改進 (Needs Work) | <70 | 紅色 | `assessment-needs-improvement.json` |

## 使用方式

### 直接開啟 HTML 檔案
```bash
# 管理者儀表板（須為 admin 角色）
open ui-screens/admin-dashboard.html

# 其他管理頁面
open ui-screens/admin-users.html
open ui-screens/admin-songs.html
open ui-screens/admin-structures.html

# 使用者介面
open ui-screens/user-practice.html
open ui-screens/user-dashboard.html
```

### 查看範例結果
```bash
# 查看優秀評分範例
cat ui-screens/sample-results/assessment-excellent.json

# 使用 jq 格式化查看
cat ui-screens/sample-results/assessment-good.json | jq .
```

## 技術特點

- **純 HTML/CSS/JavaScript (ES Modules)**: 無需建構工具，直接在瀏覽器運行
- **模組化架構**: services/utils/adapters 三層分離，各模組職責明確
- **響應式設計**: 支援不同螢幕尺寸
- **現代 UI 風格**: 使用陰影、圓角、漸層等現代設計元素
- **互動式元件**: 按鈕、表單、切換分頁等互動功能
- **範例資料**: 所有頁面包含範例資料，可直接預覽效果
- **獨立樣式**: 每個管理頁面有專屬 CSS + 共用 admin-common.css
- **統一錯誤處理**: UIErrorHandler 集中管理所有錯誤顯示與導航
- **Tempo Map 支援**: SongDataConverter 支援變速 MIDI 的 Tick↔Sec 轉換

## API 整合

### 管理者 API
- `POST /api/v1/admin/users` - 建立使用者
- `GET /api/v1/admin/users` - 列出使用者
- `PUT /api/v1/admin/users/{user_id}/toggle-active` - 啟用/停用使用者
- `POST /api/v1/admin/songs` - 上傳 MIDI（支援版本管理）
- `GET /api/v1/admin/songs` - 列出歌曲
- `GET /api/v1/admin/songs/{song_id}` - 查詢單曲詳細資訊
- `PATCH /api/v1/admin/songs/{song_id}/tracks/{track_id}` - 修改 Track 屬性 (is_vocal)
- `DELETE /api/v1/admin/songs/{song_id}` - 刪除歌曲
- `POST /api/v1/admin/songs/{song_id}/new-version` - 上傳新版本
- `PUT /api/v1/admin/songs/{song_id}/set-active` - 設定活躍版本
- `GET /api/v1/admin/songs/{song_id}/versions` - 取得版本列表
- `POST /api/v1/admin/songs/{song_id}/structures` - 建立段落結構
- `POST /api/v1/admin/songs/{song_id}/structures/{section_id}/copy-phrases` - 複製段落 Phrase
- `PUT /api/v1/admin/songs/{song_id}/structures/{structure_id}` - 修改段落結構
- `DELETE /api/v1/admin/songs/{song_id}/structures/{structure_id}` - 刪除段落結構
- `GET /api/v1/admin/songs/{song_id}/structures/export` - 匯出 CSV
- `POST /api/v1/admin/songs/{song_id}/structures/import` - 匯入 CSV
- `POST /api/v1/admin/songs/{song_id}/lyrics` - 批量管理歌詞
- `GET /api/v1/admin/tracks/{track_id}/lyrics` - 查詢特定 Track 歌詞
- `DELETE /api/v1/admin/songs/{song_id}/lyrics` - 刪除歌詞

### 使用者 API
- `POST /api/v1/auth/login` - 登入
- `GET /api/v1/songs` - 列出歌曲
- `GET /api/v1/songs/{song_id}` - 取得單曲資訊
- `GET /api/v1/songs/{song_id}/structures?track_id=<uuid>` - 取得段落結構（含歌詞，可依聲部過濾）
- `GET /api/v1/songs/{song_id}/tracks/{track_id}/audio` - 取得特定 Track 參考音訊
- `POST /api/v1/assessments/submit` - 提交評分
- `GET /api/v1/assessments` - 取得個人評分歷史
- `GET /api/v1/assessments/stats` - 取得評分統計
- `GET /api/v1/assessments/{assessment_id}` - 查詢特定評分結果
- `GET /api/v1/assessments/{assessment_id}/download` - 下載評分結果
- `DELETE /api/v1/assessments/{assessment_id}` - 刪除評分
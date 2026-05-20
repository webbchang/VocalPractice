# Vocal Practice App - UI Screens

本目錄包含 Vocal Practice App 的所有 UI 介面螢幕和範例結果模板。

## 目錄結構

```
ui-screens/
├── admin-common.css           # 管理者後台共用樣式
├── admin-users.html           # 管理者後台 - 使用者管理
├── admin-users.css            # 使用者管理樣式
├── admin-songs.html           # 管理者後台 - 歌曲管理
├── admin-songs.css            # 歌曲管理樣式
├── admin-structures.html      # 管理者後台 - 段落結構管理
├── admin-structures.css       # 段落結構管理樣式
├── user-practice.html         # 使用者練習介面
├── user-dashboard.html        # 使用者儀表板
├── import_js.txt              # JavaScript 匯入參考
├── user-practice-html.txt     # 練習介面 HTML 暫存
├── sample-results/            # 評分結果範例模板
│   ├── assessment-excellent.json        # 優秀評分 (90+ 分)
│   ├── assessment-good.json             # 良好評分 (80-89 分)
│   ├── assessment-average.json          # 普通評分 (70-79 分)
│   └── assessment-needs-improvement.json # 需改進評分 (<70 分)
└── README.md                  # 本說明文件
```

## 管理者後台模組 (Admin Modules)

### 1. 使用者管理 (admin-users.html)
- **功能**: 建立、查看、管理使用者帳號
- **特色**:
  - 新增使用者表單（使用者名稱、電子郵件、密碼）
  - 使用者列表表格顯示
  - 啟用/停用使用者切換
  - 刪除使用者功能
- **樣式**: admin-users.css, admin-common.css

### 2. 歌曲管理 (admin-songs.html)
- **功能**: 上傳 MIDI 檔案、管理歌曲清單
- **特色**:
  - 拖曳上傳 MIDI 檔案
  - 自動解析 MIDI 多聲部 Track，根據名稱自動標記 is_vocal
  - 歌曲卡片式展示
  - 顯示各聲部資訊（主旋律標記、音符數、時長）
  - 檢視歌曲詳情時，每個 Track 旁有「設為主旋律/設為伴奏」切換按鈕
  - 刪除歌曲功能
- **樣式**: admin-songs.css, admin-common.css
- **API 整合**: PATCH /api/v1/admin/songs/{song_id}/tracks/{track_id} (修改 is_vocal)

### 3. 段落結構管理 (admin-structures.html)
- **功能**: 定義和編輯歌曲段落結構
- **特色**:
  - 時間軸視覺化預覽
  - Section（段落）和 Phrase（樂句）階層管理
  - 樹狀結構清單
  - 編輯面板（時間、Tick、排序）
  - 與歌詞管理整合
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
# 在瀏覽器中開啟
open ui-screens/admin-users.html
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

- **純 HTML/CSS/JavaScript**: 無需建構工具，直接在瀏覽器運行
- **響應式設計**: 支援不同螢幕尺寸
- **現代 UI 風格**: 使用陰影、圓角、漸層等現代設計元素
- **互動式元件**: 按鈕、表單、切換分頁等互動功能
- **範例資料**: 所有頁面包含範例資料，可直接預覽效果
- **獨立樣式**: 每個管理頁面有專屬 CSS + 共用 admin-common.css

## 與後端 API 整合

這些 UI 介面設計為與以下 API 端點整合：

### 管理者 API
- `POST /api/v1/admin/users` - 建立使用者
- `GET /api/v1/admin/users` - 列出使用者
- `PUT /api/v1/admin/users/{user_id}/toggle-active` - 啟用/停用使用者
- `POST /api/v1/admin/songs` - 上傳 MIDI
- `GET /api/v1/admin/songs` - 列出歌曲
- `GET /api/v1/admin/songs/{song_id}` - 查詢單曲詳細資訊
- `PATCH /api/v1/admin/songs/{song_id}/tracks/{track_id}` - 修改 Track 屬性 (is_vocal)
- `DELETE /api/v1/admin/songs/{song_id}` - 刪除歌曲
- `POST /api/v1/admin/songs/{song_id}/structures` - 建立段落結構
- `POST /api/v1/admin/songs/{song_id}/structures/{section_id}/copy-phrases` - 複製段落 Phrase
- `PUT /api/v1/admin/songs/{song_id}/structures/{structure_id}` - 修改段落結構
- `DELETE /api/v1/admin/songs/{song_id}/structures/{structure_id}` - 刪除段落結構
- `POST /api/v1/admin/songs/{song_id}/lyrics` - 批量管理歌詞
- `GET /api/v1/admin/tracks/{track_id}/lyrics` - 查詢特定 Track 歌詞
- `DELETE /api/v1/admin/songs/{song_id}/lyrics` - 刪除歌詞

### 使用者 API
- `POST /api/v1/auth/login` - 登入
- `GET /api/v1/songs` - 列出歌曲
- `GET /api/v1/songs/{song_id}` - 取得單曲資訊
- `GET /api/v1/songs/{song_id}/structures` - 取得段落結構（含歌詞）
- `GET /api/v1/songs/{song_id}/tracks/{track_id}/audio` - 取得特定 Track 參考音訊
- `POST /api/v1/assessments/submit` - 提交評分
- `GET /api/v1/assessments` - 取得個人評分歷史
- `GET /api/v1/assessments/stats` - 取得評分統計
- `GET /api/v1/assessments/{assessment_id}` - 查詢特定評分結果
- `GET /api/v1/assessments/{assessment_id}/download` - 下載評分結果
- `DELETE /api/v1/assessments/{assessment_id}` - 刪除評分
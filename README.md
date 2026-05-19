# Vocal Practice App

Vocal Practice App 是一個專為歌手和聲樂學生設計的練習工具。管理者上傳多聲部 MIDI 檔案並定義歌曲段落結構，使用者選擇特定聲部練習、選擇伴奏聲部跟著唱。錄音後在使用者端完成 pitch detection 與逐音符比對，並以視覺化圖表呈現音高偏差與時長差異，最終將評分結果儲存在伺服器上與帳號關聯。

## 技術架構

| 層級 | 技術 |
|------|------|
| 前端 | HTML, CSS, Vanilla JavaScript (Web Audio API 合成器伴奏播放, Client-side MIDI 比對) |
| 後端 | Go, chi router |
| 架構模式 | 領域驅動設計 (DDD) |
| 資料儲存 | In-memory (可擴展為 PostgreSQL) |
| 認證方式 | JWT Token |

## 目錄結構

```
├── cmd/
│   ├── server/                  # Go 伺服器入口
│   │   └── main.go
│   ├── midi_inspect/            # MIDI 解析檢查工具
│   │   └── main.go
│   ├── midi_vs_wav_report/      # MIDI vs WAV 比對報告工具 (生成 HTML)
│   │   └── main.go
│   └── snapshot/                # 專案快照生成工具
│       └── main.go
├── internal/
│   ├── domain/                  # 領域模型 (DDD Entity)
│   │   ├── user.go              # User 模型
│   │   ├── song.go              # Song (含 MIDITrack、MIDINote、TempoMapEntry)
│   │   ├── song_structure.go    # SongStructure (SECTION/PHRASE) + StructureNode
│   │   ├── track_lyrics.go      # TrackLyrics (各聲部段落歌詞)
│   │   └── user_assessment.go   # UserAssessment (評分結果)
│   ├── handler/                 # HTTP 處理器
│   │   ├── auth.go              # POST /api/v1/auth/login + JWT middleware
│   │   ├── common.go            # respondJSON / respondError 共用工具
│   │   ├── admin_users.go       # Admin 使用者 CRUD (含 toggle-active)
│   │   ├── admin_songs.go       # Admin MIDI 上傳/管理 (含 delete)
│   │   ├── admin_structures.go  # Admin 段落結構管理
│   │   ├── admin_lyrics.go      # Admin 歌詞批量管理
│   │   ├── songs.go             # User 歌曲查詢/MIDI 下載/結構樹
│   │   └── assessments.go       # User 評分提交、歷史、統計、下載、刪除
│   ├── service/                 # 核心服務
│   │   ├── midi_parser.go       # MIDI 檔案解析（多 Track、Note 萃取、Tempo Map）
│   │   ├── midi_parser_test.go
│   │   ├── assessment_test.go
│   │   └── assessment_unit_test.go
│   └── storage/                 # 資料持久層
│       ├── memory.go
│       └── memory_test.go
├── index.html / style.css / script.js  # 前台使用者頁面（練習、分析、歷史）
├── ui-screens/                  # UI 介面靜態模板
│   ├── admin-*.html / admin-*.css     # 管理者後台頁面與樣式
│   ├── user-practice.html       # 使用者練習介面
│   ├── user-dashboard.html      # 使用者儀表板
│   └── sample-results/          # 評分結果範例 JSON
├── snapshot/                    # 專案快照
│   ├── progress.md              # 進度追蹤文件
│   └── snapshot.html            # 自動生成的專案快照
├── Vocal Practice App.md        # 設計規格文件
└── README.md
```

## 快速開始

### 1. 啟動後端伺服器

```bash
go run cmd/server/main.go
```

伺服器將監聽 `http://localhost:8080`。

### 2. 工具命令

```bash
# MIDI 解析檢查
go run cmd/midi_inspect/main.go

# MIDI vs WAV 比對報告（生成 output/comparison_report.html）
go run cmd/midi_vs_wav_report/main.go

# 生成專案快照（輸出 snapshot/snapshot.html）
go run cmd/snapshot/main.go
```

### 3. 開啟前台

直接在瀏覽器中開啟 `index.html`，或使用 Live Server。

### 4. 開啟管理後台

```bash
open ui-screens/admin-users.html
```

### 5. 執行測試

```bash
go test ./... -v
```

## API 文件

### Admin API

#### 使用者管理

```
POST /api/v1/admin/users
Content-Type: application/json

{
  "username": "singer01",
  "email": "singer01@example.com",
  "password": "securepassword"
}
```

```
GET /api/v1/admin/users
```

回應：使用者列表（不含密碼）

```
PUT /api/v1/admin/users/{user_id}/toggle-active
```

切換使用者啟用/停用狀態。

#### 上傳歌曲 MIDI 檔案

```
POST /api/v1/admin/songs
Content-Type: multipart/form-data

Parameters:
  title: "Song Title"
  artist: "Artist Name"
  midi_file: (binary .mid file)
```

- 伺服器解析 MIDI 檔案中的多個 Track，自動萃取各聲部音符序列及 Tempo Map，並根據 Track 名稱自動標記 `is_vocal`
- 回應包含 song_id 與解析出的 Track 列表（名稱、音符數、時長、is_vocal 標記）

#### 查詢歌曲列表

```
GET /api/v1/admin/songs
```

#### 修改 Track 屬性（is_vocal 標記）

```
PATCH /api/v1/admin/songs/{song_id}/tracks/{track_id}
Content-Type: application/json

{"is_vocal": true}
```

Admin 可在上傳 MIDI 後手動調整各 Track 的 `is_vocal` 標記，覆蓋自動偵測結果。

#### 查詢單曲詳細資訊（含 Track 列表）

```
GET /api/v1/admin/songs/{song_id}
```

#### 刪除歌曲

```
DELETE /api/v1/admin/songs/{song_id}
```

#### 批量建立段落結構

```
POST /api/v1/admin/songs/{song_id}/structures
Content-Type: application/json

{
  "structures": [
    {
      "type": "SECTION",
      "title": "Chorus A",
      "start_time": 45.2,
      "end_time": 75.0,
      "order_index": 1,
      "phrases": [
        {
          "type": "PHRASE",
          "title": "Line 1",
          "start_time": 45.2,
          "end_time": 60.0,
          "order_index": 1
        }
      ]
    }
  ]
}
```

#### 修改段落結構

```
PUT /api/v1/admin/songs/{song_id}/structures/{structure_id}
Content-Type: application/json

{"title": "Line 1 Edited", "start_time": 45.5}
```

#### 刪除段落結構

```
DELETE /api/v1/admin/songs/{song_id}/structures/{structure_id}
```

> SECTION 刪除時會級聯刪除巢狀的 PHRASE。

#### 歌詞批量管理

```
POST /api/v1/admin/songs/{song_id}/lyrics
Content-Type: application/json

{
  "lyrics": [
    {
      "track_id": "uuid",
      "structure_id": "uuid",
      "lyrics": "Some lyrics text"
    }
  ]
}
```

```
GET /api/v1/admin/tracks/{track_id}/lyrics
```

查詢特定 Track 的所有歌詞。

```
DELETE /api/v1/admin/songs/{song_id}/lyrics?track_id=xxx&structure_id=xxx
```

刪除特定 Track 與 Structure 關聯的歌詞。

---

### User API

#### 使用者登入

```
POST /api/v1/auth/login
Content-Type: application/json

{
  "email": "singer01@example.com",
  "password": "securepassword"
}

Response:
{
  "token": "jwt_token_string",
  "user": { "id": "uuid", "username": "singer01", "email": "singer01@example.com" }
}
```

後續請求需在 Header 帶入 `Authorization: Bearer <token>`。

#### 列出可用歌曲

```
GET /api/v1/songs
```

回應包含每首歌的 title、artist、可用 Track 列表。

#### 取得單曲資訊（含 Track 列表與段落結構概覽）

```
GET /api/v1/songs/{song_id}
```

#### 下載 MIDI 檔案

```
GET /api/v1/songs/{song_id}/midi
```

回應為 binary `.mid` 檔案。

#### 取得歌曲段落結構樹

```
GET /api/v1/songs/{song_id}/structures
```

回應為巢狀結構樹（SECTION → PHRASE），含 lyrics 欄位。

#### 取得特定 Track 參考音訊（若已生成）

```
GET /api/v1/songs/{song_id}/tracks/{track_id}/audio
```

#### 提交評分結果

```
POST /api/v1/assessments/submit
Content-Type: application/json
Authorization: Bearer <token>

{
  "song_id": "uuid",
  "structure_id": "uuid (optional, null = full song)",
  "track_id": "uuid (the vocal track being assessed)",
  "score": 85.5,
  "pitch_deviation": [10, -5, 30, 120, -8, ...],
  "duration_deviation": [0.05, -0.1, 0.3, ...],
  "note_comparison": [
    {
      "ref_pitch": 60,
      "user_pitch": 61,
      "ref_start": 0.0,
      "ref_end": 1.0,
      "user_start": 0.05,
      "user_end": 1.1,
      "pitch_deviation_cents": 100,
      "duration_deviation_sec": 0.05,
      "match_status": "matched"
    }
  ],
  "total_notes": 50,
  "matched_notes": 42,
  "average_pitch_deviation": 15.3,
  "average_duration_deviation": 0.08
}

Response:
{
  "assessment_id": "uuid",
  "status": "saved"
}
```

#### 查詢自己的評分歷史

```
GET /api/v1/assessments
Authorization: Bearer <token>
```

可選參數：`?song_id=xxx` `?limit=20` `?offset=0`

#### 查詢評分統計

```
GET /api/v1/assessments/stats
Authorization: Bearer <token>
```

#### 查詢特定評分結果詳細資料

```
GET /api/v1/assessments/{assessment_id}
Authorization: Bearer <token>
```

#### 下載評分結果（JSON 檔案）

```
GET /api/v1/assessments/{assessment_id}/download
Authorization: Bearer <token>
```

#### 刪除評分記錄

```
DELETE /api/v1/assessments/{assessment_id}
Authorization: Bearer <token>
```

---

## 使用者流程

```
1. 登入帳號
2. 選擇歌曲
3. 選擇要練習的 Vocal Track（評估基準、單選）
4. 選擇要跟著唱的伴奏 Track（可多選）
5. 結構樹顯示 SECTION/PHRASE 層級與關聯歌詞，點擊兩端設定連續練習範圍
   （或跳過此步驟直接練全曲）
6. 前端下載完整 MIDI 檔案，從中萃取所選 Track 的音符作為參考資料
7. 耳機偵測 → 有線/藍牙/無耳機對應提示（藍牙延遲警告）
8. 點擊「開始練習」→ 倒數 3 拍（3, 2, 1 視覺 + 節拍器 click）
9. Web Audio API 合成器（三角波 + 低通濾波 + ADSR 封包）播放伴奏
10. 同時 MediaRecorder 錄製使用者演唱
11. 錄音結束 → 停止伴奏 → 自動執行 pitch detection（autocorrelation）
12. 前端比對參考 MIDI vs 使用者音高/時長
12. 顯示視覺化分析結果：
    ├── 音高曲線疊加圖 (Pitch Contour Overlay)
    ├── 逐音符偏差條狀圖 (Per-Note Deviation Chart)
    ├── 時長比對圖 (Duration Comparison)
    ├── 段落分數儀表板 (Segment Score Dashboard)
    └── 錄音波形 + MIDI Piano Roll
13. 評分結果上傳伺服器，與帳號關聯
```

## 前端視覺化分析

### 1. 音高曲線疊加圖 (Pitch Contour Overlay)

X 軸為時間線（秒），Y 軸為 MIDI note number。藍色實線為參考 MIDI 音高軌跡，橘色虛線為使用者錄音辨識結果，偏差明顯處以紅色區塊標示。

### 2. 逐音符偏差條狀圖 (Per-Note Deviation Bar Chart)

每個參考音符一根柱子，柱高 = 音高偏差（cents）。顏色標示：
- 🟢 綠色：偏差 < 50 cents（良好）
- 🟡 黃色：50-100 cents（尚可）
- 🔴 紅色：> 100 cents（需改善）

### 3. 時長比對圖 (Duration Comparison)

橫條圖並排顯示參考音符與使用者音符的時長，直觀看出唱太長或太短。

### 4. 段落分數儀表板 (Segment Score Dashboard)

每個 Section/Phrase 顯示獨立分數（0-100），以進度條或圓餅圖呈現，點擊可展開逐音符細節。

### 5. 錄音波形 + MIDI Piano Roll

顯示錄音波形，上方疊加 MIDI Piano Roll 標示正確音符位置，讓使用者視覺比對「什麼時候該唱什麼音」。

## 練習輔助功能

### 1. 耳機/藍牙偵測
- 透過 `navigator.mediaDevices.enumerateDevices()` 枚舉音訊裝置
- 自動標記有線耳機、藍牙耳機（100-300ms 延遲警告）、無耳機（回授風險提示）
- 錄音時優先選擇偵測到的耳機麥克風

### 2. 結構樹與連續範圍選取
- SECTION（📂）與 PHRASE（📄）階層樹狀顯示，含 lyrics 歌詞
- 點擊兩個項目設定範圍起訖（start marker → end marker）
- 選取範圍後顯示時間區間，伴奏僅播放範圍內音符

### 3. 倒數拍 (Count-in)
- 從 MIDI 音符 onset 計算 BPM（median inter-onset interval）
- 3 拍倒數：3, 2, 1 視覺文字 + oscillator click
- 倒數結束後伴奏與錄音同時開始

### 4. 合成器伴奏播放
- Web Audio API `OscillatorNode`（三角波）
- `BiquadFilterNode`（低通濾波器）模擬柔和音色
- ADSR 封包控制每個音符的起音/衰減/釋放
- 僅播放選取時間範圍內的音符
- 所有 `OscillatorNode` 集中管理，可一鍵停止

### 4. Client-side 參考資料生成
- 前端下載完整 MIDI 檔案後，由 Client-side MIDI parser 解析所有 Track 音符
- 根據使用者選擇的 Track UUID，自動對應到 MIDI 檔案中的 Track 索引（0-indexed，假設順序與 API 回傳一致）
- 選擇段落後，過濾出該 Track 在時間範圍 `[start-0.05, end)` 內的所有音符作為參考資料
- 無需從伺服器額外下載參考資料，降低伺服器負擔
- 此方式可提高練習流程的反應速度

## 核心服務

### MIDI 檔案解析器 (MIDI Parser)

解析 Standard MIDI File (.mid)，萃取多個 Track 的名稱、樂器、音符序列（pitch, velocity, start_time, end_time）。支援多聲部辨識、Tempo Map 萃取、與段落時間計算。

### 使用者認證服務

基於 JWT Token 的登入/註冊機制，保護使用者資料與評分歷史。

### MIDI 檔案儲存與串流

管理 MIDI 檔案的上傳、儲存與下載，支援特定 Track 的參考音訊生成與串流。

### 歌詞管理

管理者可為每個 Track 的每個段落（Structure）關聯歌詞，支援批量新增/更新、查詢與刪除。

## License

MIT
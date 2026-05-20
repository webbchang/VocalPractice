# Vocal Practice App 測試進度

## 測試時間
2026-05-17 01:58

## 測試結果摘要

### ✅ 所有測試通過

| 測試項目 | 狀態 | 說明 |
|----------|------|------|
| 伺服器啟動 | ✅ 通過 | Go 伺服器成功啟動於 port 8080 |
| 健康檢查 | ✅ 通過 | GET /health 回傳 `{"status":"ok"}` |
| 管理員建立使用者 | ✅ 通過 | POST /api/v1/admin/users 成功建立使用者 |
| 管理員上傳 MIDI | ✅ 通過 | POST /api/v1/admin/songs 成功上傳 reference.mid |
| 管理員查詢歌曲 | ✅ 通過 | GET /api/v1/admin/songs 回傳歌曲列表 |
| 管理員建立段落結構 | ✅ 通過 | POST /api/v1/admin/songs/{id}/structures 成功建立 Verse 1 和 Chorus |
| 使用者登入 | ✅ 通過 | POST /api/v1/auth/login 成功取得 JWT token |
| 使用者查詢歌曲 | ✅ 通過 | GET /api/v1/songs 回傳可用歌曲 |
| 使用者查詢段落結構 | ✅ 通過 | GET /api/v1/songs/{id}/structures 回傳完整結構樹 |
| 使用者提交評分 | ✅ 通過 | POST /api/v1/assessments/submit 成功儲存評分 |
| 管理員查詢所有評分 | ✅ 通過 | GET /api/v1/admin/assessments 回傳所有評分 |
| 使用者查詢個人評分 | ✅ 通過 | GET /api/v1/assessments (需 JWT) 回傳個人評分 |

## Bug 修復

### JWT Token 生成錯誤
**問題**: `internal/handler/auth.go` 中的 `generateToken` 函數使用 `string(rune(...))` 來轉換 Unix 時間戳記，導致 token 無效。

**修復**: 改用 `strconv.FormatInt()` 正確轉換 int64 為字串。

```go
// 修復前 (錯誤)
"exp":` + string(rune(time.Now().Add(24*time.Hour).Unix())) + `

// 修復後 (正確)
exp := strconv.FormatInt(time.Now().Add(24*time.Hour).Unix(), 10)
"exp":` + exp + `
```

## API 測試詳細結果

### 1. 建立使用者
```json
{
  "id": "8ee791c4-ab91-4a1c-97fc-daaa2af76458",
  "username": "test_singer",
  "email": "testsinger@example.com"
}
```

### 2. 上傳 MIDI 歌曲
```json
{
  "artist": "Test Artist",
  "song_id": "8671ced5-965b-47fe-a805-86174f8296af",
  "title": "Test Song",
  "tracks": [
    {
      "id": "f17520d8-0537-4f9a-bf51-6444506b2aaa",
      "name": "中音長笛, Alto 2",
      "note_count": 140,
      "duration_sec": 258.14,
      "is_vocal": false
    }
  ]
}
```

### 3. 建立段落結構
成功建立:
- Verse 1 (0-60秒) 包含 Line 1 (0-15秒) 和 Line 2 (15-30秒)
- Chorus (60-120秒) 包含 Chorus Line 1 (60-90秒)

### 4. 提交評分
```json
{
  "assessment_id": "ebd37d01-f076-4966-aec8-c945dbd8833b",
  "status": "saved"
}
```

## 新功能（2026-05-19 更新 #2）

### Client-side Reference Data 自動化測試
- **cmd/midi_ref_test/ref_filter.cjs** — 獨立的 Node.js 腳本，實作了 user-practice.html 中的完整 client-side MIDI 解析邏輯（parseMIDINotes + generateReferenceData）
- 支援兩種操作模式：
  - **Stdin 模式**：接收 JSON 輸入（midiPath + trackIndex + start/end），輸出 JSON 結果
  - **HTTP 伺服器模式**：啟動 HTTP server，接受 multipart POST（支援 curl），可用於外部驗證
- **internal/service/ref_filter_test.go** — 8 項自動化測試：
  - `TestJSRefFilter_Stdin_BasicFilter` — 基本範圍過濾驗證（時間邊界檢查）
  - `TestJSRefFilter_Stdin_EmptyRange` — 空範圍回傳空陣列
  - `TestJSRefFilter_Stdin_MultiTrack` — 多聲部 MIDI 解析與過濾
  - `TestJSRefFilter_Stdin_CompareWithGoParser` — 比對 Go server-side vs JS client-side parser 輸出（結果完全一致：14 音符，0 差異）
  - `TestJSRefFilter_HTTPServer_Basic` — HTTP 伺服器端到端測試（multipart POST + GET）
  - `TestJSRefFilter_Stdin_ToleranceStart` — 驗證 -0.05s tolerance 邊界行為
  - `TestGoParser_ReferenceDataFiltering` — Go parser 四種範圍測試（含邊界案例）
  - `TestMIDIParserNoteTiming` — 驗證 Go parser 產生的時間戳合理性
- 所有測試通過：共 22 項（原本 14 項 + 新增 8 項）

## 新功能（2026-05-19 更新 #1）

以下功能已完成實作並整合至前台：

### Admin 可手動指派 isVocal
- **PATCH /api/v1/admin/songs/{song_id}/tracks/{track_id}**：Admin 在上傳 MIDI 後可手動調整各 Track 的 `is_vocal` 標記，覆蓋自動偵測結果
- **admin-songs.html**：檢視歌曲詳情時，每個 Track 旁顯示「設為主旋律/設為伴奏」切換按鈕
- **internal/storage/memory.go**：新增 `UpdateTrack()` 方法，支援部分欄位更新
- **internal/handler/admin_songs.go**：新增 `UpdateTrack` handler 處理 PATCH 請求

### Client-side Reference Data 生成
- **user-practice.html**：前端下載完整 MIDI 檔案後，使用者選擇 Track + 段落時，從已解析的 MIDI 音符中過濾出對應 Track 在時間範圍內的音符作為參考資料
- 不再依賴伺服器額外提供參考資料端點，降低伺服器負載與延遲
- 對應邏輯：根據 `currentSongFull.tracks[i].id === selectedTrackId` 決定 MIDI Track 索引，再以 `parsedNotes.filter(n => n.track === trackIndex && n.start >= start-0.05 && n.start < end)` 產生參考資料

## 新功能（2026-05-17 更新）

以下功能已完成實作並整合至前台：

### 前端練習流程強化
- **結構樹與連續範圍選取**：SECTION（📂）/ PHRASE（📄）階層樹狀顯示，點擊兩端設定練習範圍
- **耳機/藍牙偵測**：枚舉音訊裝置，自動標記有線/藍牙/無耳機並顯示對應警告
- **倒數拍**：從 MIDI 音符計算 BPM，3 拍倒數（3, 2, 1 視覺 + click）
- **合成器伴奏播放**：Web Audio API 三角波 + 低通濾波 + ADSR 封包
- **同時錄音**：伴奏播放與 MediaRecorder 錄音同步進行

### 後端擴展
- **歌詞管理**：TrackLyrics 領域模型、AdminLyricsHandler (CRUD)
- **使用者啟用/停用**：PUT /api/v1/admin/users/{id}/toggle-active
- **刪除歌曲**：DELETE /api/v1/admin/songs/{id}
- **評分統計 API**：GET /api/v1/assessments/stats
- **評分下載 API**：GET /api/v1/assessments/{id}/download
- **評分刪除 API**：DELETE /api/v1/assessments/{id}
- **Tempo Map 支援**：Song 模型新增 TempoMapEntry 與 ticks_per_quarter
- **StructureNode**：巢狀結構回應含 lyrics 欄位

### 工具命令
- **cmd/midi_inspect**：MIDI 解析檢查工具
- **cmd/midi_vs_wav_report**：MIDI vs WAV 比對報告，生成 HTML 視覺化報表
- **cmd/snapshot**：專案快照生成工具（輸出 snapshot/snapshot.html）

### UI 頁面
- **user-dashboard.html**：使用者儀表板（練習次數、平均分數、最近記錄）
- 每個管理頁面新增獨立 CSS 檔案（admin-*.css）+ 共用 admin-common.css

### 文件更新
- `README.md`：更新目錄結構、技術架構、使用者流程、新增練習輔助功能章節、新 API 端點與工具命令
- `Vocal Practice App.md`：更新 Data Models（TempoMapEntry、StructureNode、TrackLyrics）、API 端點（lyrics CRUD、stats/download/delete）、工具命令、使用方式
- `ui-screens/README.md`：新增 UI 螢幕說明文件，含 admin CSS、user-dashboard、API 端點清單

## 測試資料
- MIDI 檔案: `test_data/reference.mid` (3.8KB, 140 音符, 258秒)
- 錄音檔案: `test_data/test_recording_alto.wav`, `test_data/test_recording_sinwave.wav`

## 專案快照
- 已生成: `snapshot/snapshot.html`
- 檔案數: 50 | Dirs: 12 | 總行數: ~16108 | 大小: ~516.4 KB
- 測試套件：**22 項測試全部通過**（service 18 項、storage 4 項）
- go build 編譯成功無錯誤
- 新增 `cmd/midi_ref_test/` 目錄：包含 ref_filter.cjs（Node.js client-side MIDI 解析器，支援 stdin / HTTP 雙模式）
- 新增 `internal/service/ref_filter_test.go`：8 項 JS-bridge 自動化測試（驗證 client-side reference data 生成的時間範圍正確性）

- [2026-05-19 23:45:28] 完成前端 MIDI 單一來源重構：
  - assets/js/midiParser.js 作為單一 MIDI parser（window.MidiParser）
  - assets/js/songDataExtractor.js 作為 reference extractor（window.SongDataExtractor）
  - ui-screens/user-practice.html、ui-screens/admin-structures.html 移除重複解析邏輯並改為共用模組
  - script.js 已建立時間戳備份

- [2026-05-20 00:26:19] 新增獨立測試伺服器 `cmd/test_server/main.go`（不影響正式 `cmd/server/main.go`）：
  - 預設啟動於 `:18080`
  - 內建 seed 使用者：`webbchang@gmail.com / test1234`
  - 啟動時自動嘗試載入 `test_data/reference2.MID`（或 `reference2.mid`）
  - 成功時建立預設歌曲：`song1 / artist1`
  - 驗證結果：
    - `GET /health` => `{"status":"ok","server":"test"}`
    - `POST /api/v1/auth/login` 可成功取得 token
    - `GET /api/v1/songs` 回傳 `song1 / artist1`

- [2026-05-20 01:12:59] admin-structures UI 修正（方案 1，最小改動）：
  - `ui-screens/admin-structures.html`：lyrics 編輯欄改為僅 `PHRASE` 顯示
  - 新增 `updateLyricsEditorVisibility(type)` 統一控制顯示行為
  - 編輯/新增/切換類型時同步套用規則：`SECTION` 隱藏 lyrics、顯示提示文字
  - 提示文案：`SECTION 不儲存歌詞，請使用 PHRASE 編輯歌詞。`

- [2026-05-20 01:35:56] admin-structures 後端新增「複製 section phrases + 時間排序」：
  - 新增 API：`POST /api/v1/admin/songs/{song_id}/structures/{section_id}/copy-phrases`
  - 可將來源 section 下所有 phrase 複製到目標 section，並複製對應 track lyrics
  - `internal/storage/memory.go`：`BuildStructureTree` 新增穩定排序（`StartTick` -> `StartTime` -> `OrderIdx` -> `ID`）
  - 測試新增：
    - `internal/handler/admin_structures_test.go`：`TestCopySectionPhrasesSuccess`
    - `internal/storage/memory_test.go`：`TestBuildStructureTreePhraseSortedByTime`
  - 驗證：`go test ./internal/storage ./internal/handler` 全部通過

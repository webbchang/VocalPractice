# Vocal Practice App 測試進度

## 測試時間
2026-05-28 00:50

## 最新實作摘要

### 管理者後台 ES Modules 重構（2026-05-23 ~ 2026-05-24）

`ui-screens/js/` 目錄全面重構為 ES Modules 架構：

#### 新增模組

| 模組 | 檔案 | 說明 |
|------|------|------|
| 主入口 | `admin.js` | 管理者頁面主邏輯（結構 CRUD、CSV、播放、版本管理） |
| API 服務 | `services/api-service.js` | SongService 封裝所有 Admin/User API 呼叫 |
| 錯誤處理 | `utils/error-handler.js` | UIErrorHandler 統一錯誤處理（401/409 處理） |
| 資料轉換 | `utils/songdata-converter.js` | SongDataConverter（tick↔sec、時間格式化、結構正規化） |
| UI 渲染 | `adapters/ui-renderer.js` | StructureRenderer（結構樹、時間軸、聲部選擇器）+ UIStatus |
| 音訊 | `adapters/audio-adapter.js` | WebAudioAdapter（MIDI 範圍播放、三角波+低通濾波+ADSR） |

#### 對應測試檔案

| 測試 | 說明 |
|------|------|
| `ui-screens/js/__tests__/api-service.test.js` | SongService API 服務層測試 |
| `ui-screens/js/__tests__/audio-adapter.test.js` | WebAudioAdapter 音訊適配器測試 |
| `ui-screens/js/__tests__/error-handler.test.js` | UIErrorHandler 錯誤處理測試 |
| `ui-screens/js/__tests__/songdata-converter.test.js` | SongDataConverter 資料轉換測試 |
| `ui-screens/js/__tests__/ui-renderer.test.js` | StructureRenderer/UIStatus 渲染測試 |

### 管理者儀表板（2026-05-23）
- `ui-screens/admin-dashboard.html`：角色權限檢查（admin role required），三張功能卡片
- 若無權限自動跳轉登入頁

### 跨聲部複製結構（2026-05-20）
- `POST /api/v1/admin/songs/{song_id}/structures/{section_id}/copy-phrases`
- 可將來源 section 下所有 phrase 複製到目標 section，並複製對應 track lyrics
- `internal/storage/memory.go`：`BuildStructureTree` 新增穩定排序（`StartTick` -> `StartTime` -> `OrderIdx` -> `ID`）

### 歌曲版本管理（2026-06-01）
- `POST /api/v1/admin/songs/{song_id}/new-version`：上傳新 MIDI 作為新版本
- `PUT /api/v1/admin/songs/{song_id}/set-active`：設定活躍版本
- `GET /api/v1/admin/songs/{song_id}/versions`：取得版本列表
- `uploadSong(title, artist, file, sourceSongId)`：上傳時可指定關聯歌曲

## API 測試歷史結果

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

## 已修復 Bug

### JWT Token 生成錯誤
**問題**: `internal/handler/auth.go` 中的 `generateToken` 函數使用 `string(rune(...))` 來轉換 Unix 時間戳記，導致 token 無效。
**修復**: 改用 `strconv.FormatInt()` 正確轉換 int64 為字串。

## 測試統計

| 測試套件 | 數量 | 狀態 |
|----------|------|------|
| `internal/service/` | 20+ 項 | ✅ |
| `internal/storage/` | 4 項 | ✅ |
| `internal/handler/` | 2 項 | ✅ |
| `ui-screens/js/__tests__/` | 5 項 | ✅ |
| `assets/js/__tests__/` | 7 項 | ✅ |

## 專案快照
- 已生成: `snapshot/snapshot.html`
- 檔案數: ~50+ | Dirs: ~12+ | 總行數: ~18000+
- go build 編譯成功無錯誤
- 新增 `ui-screens/js/` 目錄：6 個 ES 模組 + 5 個測試檔
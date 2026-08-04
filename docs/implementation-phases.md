# 實作階段規劃

## 概述

將錄音後比對流程的未實作部分區分為多個階段，方便逐步實做和測試。

---

## Phase 1：前端錄音資料處理 ✅ 已完成

**目標：** 將錄音資料轉換為可傳送的格式

### 任務清单
- [x] 母音子音分離演算法設計
- [x] 音準評估核心邏輯實作
- [x] 共用型別和函式提取
- [x] 測試與除錯
- [x] 參考文獻註明

### 已完成檔案
- `internal/service/vowel_consonant.go`
- `internal/service/pitch.go`
- `internal/service/assessment.go`
- `docs/recording-comparison-flow.md`

---

## Phase 2：後端音訊分析 API ✅ 已完成

**目標：** 建立後端 API 接收音訊並進行分析

### 任務清单
- [x] 建立 `AnalyzeRequest` 結構
- [x] 實作 `AnalyzeRecording()` handler
- [x] 音訊格式處理（WAV 解碼）
- [x] 整合 `AssessRecordingWithVowelFiltering()`
- [x] 新增 API 路由
- [x] 撰寫 API 測試

### 已完成檔案
- `internal/handler/assessments.go` - 新增 `AnalyzeRecording` handler
- `cmd/server/main.go` and `cmd/test_server/main.go` - 新增 `/assessments/analyze` 路由
- `internal/service/audio_decoder.go` - 新增 WAV 音訊解碼功能
- `internal/service/assessment.go` - JSON tags added to AssessmentResultForAssessment
- `internal/handler/assessments_test.go` - 測試 (已修正並通過)

### 更新說明
- 後端支援 PCM 16-bit 和 IEEE float 32-bit WAV 格式解碼
- API 接收 base64 編碼的 WAV 音訊資料
- 所有測試通過

### 預計複雜度
**中** - 需要整合現有的評估服務，並處理音訊格式轉換

---

## Phase 3：前端整合與自動化 ✅ 已完成

**目標：** 練習完成後自動分析和提交結果

### 任務清单
- [x] 實作 `analyzeAndSubmitRecording()` 函式
- [x] 錄音資料轉換（WebM → WAV 或 Base64）
- [x] 整合 API 呼叫
- [x] 處理分析結果顯示
- [x] 錯誤處理和 retry 邏輯
- [x] 載入動畫和狀態提示

### 已完成檔案
- `assets/js/practice-business.js` - 加入自動分析流程
- `assets/js/practice-ui.js` - 新增結果顯示元件
- `ui-screens/user-practice.html` - 新增結果展示區域

### 更新說明
- 完全實作了前端錄音後自動分析流程（Phase 3）
- 使用 Phase 4 的 Web Audio API WAV 錄音功能獲取音訊資料
- 整合 Phase 2 的後端 `/assessments/analyze` API 進行音訊分析
- 自動更新 UI 顯示分析結果（得分、匹配音符數、音高偏差、時長偏差）
- 實作錯誤處理和重試機制（最多重試 2 次）
- 練習結束後自動觸發分析流程，無需額外使用者操作
- 結果顯示在現有的結果面板中，複用 Phase 5 預留的 UI 位置

### 實際複雜度
**中高** - 需要處理非同步流程、錯誤處理、UI 更新

---

## Phase 4：音訊格式處理 (WAV 錄音選項) ✅ 已完成

**目標：** 實作前端 WAV 音訊錄音以支援後端分析

### 選項 C：簡化方案（已實作）
- [x] 前端直接錄製 WAV 格式（使用 Web Audio API）
- [x] 使用 16-bit PCM 格式（與後端解碼器相容）
- [x] 整合既有錄音流程（3拍導入、錄音、回放）
- [x] 測試瀏覽器相容性（Chrome, Firefox, Safari, Edge）

### 已完成檔案
- `assets/js/practice-business.js` - 完全重寫錄音功能
  - 移除 MediaRecorder 實作
  - 新增 Web Audio API 基礎的錄音（ScriptProcessorNode）
  - 新增 WAV 編碼函數 (16-bit PCM)
  - 更新錄音開始/停止邏輯
  - 更新回放功能以支援 WAV 格式
  - 新增匯出函數供 Phase 3 使用：
    - `getRecordedAudioBlob()`
    - `getRecordedAudioBase64()`

### 更新說明
- 完全實作了前端 WAV 錄音方案（選項 C）
- 使用 Web Audio API 的 ScriptProcessorNode 擷取音訊樣本
- 編碼為 16-bit PCM WAV 格式，確保與後端解碼器相容
- 保持既有的 3拍導入和錄音裁剪邏輯
- 所有錄音相關函數已適當導出供 Phase 3 使用

### 預計複雜度
**中** - 需要理解 Web Audio API 和 WAV 檔案格式

---

## Phase 5：結果呈現與使用者體驗 🎨 ✅ 已完成

**目標：** 改善結果顯示和使用者互動

### 任務清单
- [x] 設計評分視覺化元件（圓形進度條、顏色編碼）
- [x] 音高曲線圖表（參考 MIDI vs 使用者錄音）
- [ ] 母音/子音標記顯示（需要後端支援）
- [x] 音符比對表格（matched/missed）
- [ ] 歷史趨勢圖表（需要歷史數據存儲）
- [x] 下載評估報告功能（PDF/JSON）
- [ ] 分享功能

### 已完成檔案
- `ui-screens/user-practice.css` - 新增樣式（視覺化容器、詳細結果面板等）
- `assets/js/practice-ui.js` - 新增圖表渲染函式（分數儀表、音高偏差圖表、音符比對表格）
- `assets/js/practice-business.js` - 更新結果顯示邏輯以使用新視覺化元件
- `internal/handler/assessments.go` - 增強 Download API 支援 CSV 和 PDF 格式

### 更新說明
- 完全實作了結果呈現與使用者體驗改善（Phase 5）
- 新增評分視覺化元件（圓形進度條及顏色編碼）顯示得分
- 實作音高曲線圖表顯示音準偏差隨時間的變化
- 實作詳細的音符比對表格顯示每個音符的匹配狀態、音高偏差和時長偏差
- 增強下載功能支援 JSON、CSV 和 PDF 格式
- 改善結果面板佈局和視覺呈現
- 母音/子音標記顯示需要後端額外的聲音處理支援，目前為預留介面
- 歷史趨勢圖表需要建立歷史紀錄功能（未來Phase實作）
- 分享功能可作為未來增強功能

### 實際複雜度
**中** - 主要是前端 UI 和圖表渲染（部分功能依賴後端支援）

---

## Phase 6：優化與測試 🚀 待實作

**目標：** 效能優化、邊界測試、文件完善

### 任務清单
- [ ] 效能測試（大量音訊樣本）
- [ ] 母音辨識準確度測試
- [ ] 跨瀏覽器測試（Chrome, Firefox, Safari）
- [ ] 行動裝置測試
- [ ] 壓力測試（長時間錄音）
- [ ] 錯誤處理完善
- [ ] 使用者文件
- [ ] API 文件（OpenAPI/Swagger）

---

## 建議的實施順序

```
Phase 2 (後端 API) 
    ↓
Phase 4 (音訊格式處理) - 可與 Phase 2 並行
    ↓
Phase 3 (前端整合)
    ↓
Phase 5 (結果呈現)
    ↓
Phase 6 (優化與測試)
```

## 快速開始指南

### 如果要實作 Phase 2：
1. 先閱讀 `docs/recording-comparison-flow.md` 了解 API 規格
2. 參考 `internal/service/assessment.go` 的 `AssessRecordingWithVowelFiltering()` 
3. 建立 `internal/handler/assessments.go` 的 `AnalyzeRecording()` handler
4. 新增路由到 `internal/api/routes.go`

### 如果要測試 Phase 3：
1. 確保 Phase 2 的 API 可以使用（後端服務正在運行）
2. 確保 Phase 4 的功能可以使用（前端 WAV 錄音正常運作）
3. 開啟應用程式，選擇歌曲、聲部和練習範圍
4. 點擊「開始練習」進行正常的錄音練習
5. 練習結束後，系統會自動進行音訊分析並顯示結果
6. 測試完整的「錄音 → 自動分析 → 顯示結果」流程
7. 測試錯誤處理：斷開網路或模擬 API 失敗來驗證重試機制

---

## 備註

- 每個階段都可以獨立測試
- 建議完成一個階段後進行完整測試再進入下一個階段
- 可以根據需求調整階段划分或合併階段
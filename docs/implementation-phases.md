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

## Phase 2：後端音訊分析 API ⏳ 待實作

**目標：** 建立後端 API 接收音訊並進行分析

### 任務清单
- [ ] 建立 `AnalyzeRequest` 結構
- [ ] 實作 `AnalyzeRecording()` handler
- [ ] 音訊格式處理（WebM/Opus 解碼）
- [ ] 整合 `AssessRecordingWithVowelFiltering()`
- [ ] 新增 API 路由
- [ ] 撰寫 API 測試

### 需要建立/修改的檔案
- `internal/handler/assessments.go` - 新增 `AnalyzeRecording` handler
- `internal/api/routes.go` - 新增 `/assessments/analyze` 路由
- `internal/service/audio_decoder.go` - 新增音訊解碼功能

### 預計複雜度
**中** - 需要整合現有的評估服務，並處理音訊格式轉換

---

## Phase 3：前端整合與自動化 ⌨️ 待實作

**目標：** 練習完成後自動分析和提交結果

### 任務清单
- [ ] 實作 `analyzeAndSubmitRecording()` 函式
- [ ] 錄音資料轉換（WebM → WAV 或 Base64）
- [ ] 整合 API 呼叫
- [ ] 處理分析結果顯示
- [ ] 錯誤處理和 retry 邏輯
- [ ] 載入動畫和狀態提示

### 需要修改的檔案
- `assets/js/practice-business.js` - 加入自動分析流程
- `assets/js/practice-ui.js` - 新增結果顯示元件
- `ui-screens/user-practice.html` - 新增結果展示區域

### 預計複雜度
**中高** - 需要處理非同步流程、錯誤處理、UI 更新

---

## Phase 4：音訊格式處理 🔧 待實作

**目標：** 處理前端錄音格式與後端分析格式的轉換

### 選項 A：前端轉換
- [ ] 實作 WebM → WAV 轉換（使用 Web Audio API）
- [ ] 或實作 WebM → PCM samples 轉換
- [ ] 測試不同瀏覽器的相容性

### 選項 B：後端解碼
- [ ] 整合 ffmpeg 或 libav（需要系統相依套件）
- [ ] 或使用纯 Go 的 WebM 解碼器（如 `github.com/atotto/webaudio`）
- [ ] 處理音訊格式偵測

### 選項 C：簡化方案
- [ ] 前端直接錄製 WAV 格式（犧牲一些品質）
- [ ] 使用 IEEE float 32-bit 格式

### 推薦方案
**選項 C** - 最簡單快速，MediaRecorder 支援 `audio/wav` 格式

---

## Phase 5：結果呈現與使用者體驗 🎨 待實作

**目標：** 改善結果顯示和使用者互動

### 任務清单
- [ ] 設計評分視覺化元件（圓形進度條、顏色編碼）
- [ ] 音高曲線圖表（參考 MIDI vs 使用者錄音）
- [ ] 母音/子音標記顯示
- [ ] 音符比對表格（matched/missed）
- [ ] 歷史趨勢圖表
- [ ] 下載評估報告功能（PDF/JSON）
- [ ] 分享功能

### 需要修改的檔案
- `ui-screens/user-practice.css` - 新增樣式
- `assets/js/practice-ui.js` - 圖表渲染
- `internal/handler/assessments.go` - 增強 Download API

### 預計複雜度
**中** - 主要是前端 UI 和圖表渲染

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

### 如果要實作 Phase 3：
1. 確保 Phase 2 的 API 可以使用
2. 修改 `assets/js/practice-business.js` 的 `cleanupPractice()` 函式
3. 在練習結束後加入 `analyzeAndSubmitRecording()` 呼叫
4. 測試完整的「錄音 → 分析 → 顯示結果」流程

---

## 備註

- 每個階段都可以獨立測試
- 建議完成一個階段後進行完整測試再進入下一個階段
- 可以根據需求調整階段划分或合併階段
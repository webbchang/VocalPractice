# 錄音後比對流程文件

## 概述

本文檔描述使用者錄音後，系統進行音準分析和比對的完整流程，包括函數呼叫和 API 呼叫。

## 流程圖

```
使用者錄音完成
    ↓
[前端] 準備錄音資料
    ↓
[前端] 呼叫 API 提交評估結果
    ↓
[後端] 儲存評估結果到資料庫
    ↓
[前端] 顯示結果給使用者
```

## 詳細流程

### 1. 錄音完成後處理

**位置：** `assets/js/practice-business.js`

```javascript
// cleanupPractice() 完成後，顯示回放按鈕
function showReplayButton() {
    // 建立回放按鈕和停止按鈕
    // 使用者可以回放錄音
}
```

### 2. 音訊分析和評估流程

**主要函數：** `internal/service/assessment.go`

#### 2.1 只分析母音（新功能）

```go
// 函數簽名
func AssessRecordingWithVowelFiltering(
    samples []float64,        // 音訊樣本
    sampleRate int,           // 取樣率
    referenceNotes []domain.MIDINote  // 參考音符（從 MIDI 取得）
) (*AssessmentResultForAssessment, error)
```

**執行步驟：**

1. **音高偵測** (`detectPitchGo`)
   - 使用自相關法 (Autocorrelation) 偵測所有音符
   - 參數：window size = 2048 samples, hop size = 50ms
   - 參考："A Comparative Study of Pitch Detection Algorithms"

2. **母音子音分離** (`VowelConsonantSeparator.GetVowelOnlyNotes`)
   - 從偵測到的音符中識別母音部分
   - 使用頻譜特徵：
     - 頻譜質心 (Spectral Centroid) < 2000 Hz
     - 過零率 (Zero-Crossing Rate) < 0.3
     - 能量穩定性 (Energy Stability) > 0.6
   - 參考："Vowel/Consonant Discrimination in Speech"

3. **音符合併** (`mergeSamePitchNotesForAssessment`)
   - 合併相同音高的連續音符（間隔 < 50ms）
   - 參考："Music Transcription" by A. Klapuri, 2004

4. **音符比對** (`compareMergedNotesForAssessment`)
   - 比較參考音符和偵測到的母音音符
   - 計算音高偏差（cents）和時長偏差（seconds）
   - 重疊率門檻：80%

5. **計算分數**
   - 音高分數 (70%) + 時長分數 (30%)
   - 公式：`pitchScore * 0.7 + durationScore * 0.3`

#### 2.2 分析所有音符（原有方法）

```go
// 函數簽名
func AssessRecording(
    samples []float64,
    sampleRate int,
    referenceNotes []domain.MIDINote
) (*AssessmentResultForAssessment, error)
```

與母音過濾版本相同，但不執行步驟 2（母音子音分離）。

### 3. API 呼叫流程

#### 3.1 提交評估結果

**前端呼叫：**

```javascript
// assets/js/practice-business.js
async function submitAssessment(result) {
    const response = await api('/assessments', {
        method: 'POST',
        body: JSON.stringify({
            song_id: state.currentSongId,
            structure_id: state.selectedStructure?.id,
            track_id: state.selectedTrackId,
            score: result.Score,
            total_notes: result.TotalNotes,
            matched_notes: result.MatchedNotes,
            average_pitch_deviation: result.AveragePitchDeviation,
            average_duration_deviation: result.AverageDurationDeviation,
            pitch_deviation: result.PitchDeviation,
            duration_deviation: result.DurationDeviation,
            note_comparison: result.NoteComparison
        })
    });
    
    return response;
}
```

**後端處理：**

```go
// internal/handler/assessments.go
func (h *UserAssessmentsHandler) Submit(w http.ResponseWriter, r *http.Request) {
    // 1. 解析請求
    var req submitAssessmentRequest
    json.NewDecoder(r.Body).Decode(&req)
    
    // 2. 建立評估記錄
    assessment := domain.NewUserAssessment(
        userID, songID, trackID, structureID,
        req.Score, req.TotalNotes, req.MatchedNotes,
        req.AveragePitchDeviation, req.AverageDurationDeviation,
        req.PitchDeviation, req.DurationDeviation,
        req.NoteComparison
    )
    
    // 3. 儲存到資料庫
    h.store.CreateAssessment(assessment)
    
    // 4. 回傳評估 ID
    respondJSON(w, http.StatusCreated, map[string]interface{}{
        "assessment_id": assessment.ID,
        "status": "saved"
    })
}
```

**API 規格：**

- **URL：** `POST /assessments`
- **認證：** 需要 JWT token
- **請求參數：**
  ```json
  {
    "song_id": "uuid",
    "structure_id": "uuid (可選)",
    "track_id": "uuid",
    "score": 85.5,
    "total_notes": 50,
    "matched_notes": 42,
    "average_pitch_deviation": 15.3,
    "average_duration_deviation": 0.08,
    "pitch_deviation": [10, -5, ...],
    "duration_deviation": [0.05, -0.02, ...],
    "note_comparison": [
      {
        "ref_pitch": 60,
        "user_pitch": 62,
        "ref_start": 1.0,
        "ref_end": 2.0,
        "user_start": 1.0,
        "user_end": 2.0,
        "pitch_deviation_cents": 35,
        "duration_deviation_sec": 0.0,
        "match_status": "matched"
      }
    ]
  }
  ```

- **回應：**
  ```json
  {
    "assessment_id": "uuid",
    "status": "saved"
  }
  ```

#### 3.2 取得評估歷史

**前端呼叫：**

```javascript
// 取得使用者的評估歷史
async function loadAssessmentHistory(songId) {
    const assessments = await api(`/assessments?song_id=${songId}&limit=20`);
    return assessments;
}
```

**API 規格：**

- **URL：** `GET /assessments`
- **查詢參數：**
  - `song_id` (可選)：過濾特定歌曲
  - `limit` (可選)：每頁筆數，預設 20
  - `offset` (可選)：偏移量，預設 0
- **回應：** 評估記錄陣列

#### 3.3 下載評估報告

**前端呼叫：**

```javascript
// 下載評估報告 JSON
function downloadAssessmentReport(assessmentId) {
    window.open(`/assessments/${assessmentId}/download`);
}
```

**API 規格：**

- **URL：** `GET /assessments/{assessment_id}/download`
- **回應：** JSON 檔案附件

## 資料結構

### AssessmentResultForAssessment

```go
type AssessmentResultForAssessment struct {
    Score                    int
    TotalNotes               int
    MatchedNotes             int
    AveragePitchDeviation    float64
    AverageDurationDeviation float64
    PitchDeviation           []float64
    DurationDeviation        []float64
    NoteComparison           []domain.NoteComparison
}
```

### domain.NoteComparison

```go
type NoteComparison struct {
    RefPitch             int
    UserPitch            int
    RefStart             float64
    RefEnd               float64
    UserStart            float64
    UserEnd              float64
    PitchDeviationCents  float64
    DurationDeviationSec float64
    MatchStatus          string  // "matched" 或 "missed"
}
```

## 使用範例

### 後端使用範例

```go
package main

import (
    "vocal-practice-app/internal/service"
    "vocal-practice-app/internal/domain"
)

func analyzeRecording(samples []float64, sampleRate int, referenceNotes []domain.MIDINote) {
    // 方法 1：只分析母音（推薦）
    result, err := service.AssessRecordingWithVowelFiltering(
        samples, 
        sampleRate, 
        referenceNotes
    )
    
    if err != nil {
        log.Fatal(err)
    }
    
    // 使用結果
    score := result.Score
    matchedNotes := result.MatchedNotes
    totalNotes := result.TotalNotes
    
    // 方法 2：分析所有音符
    result2, err := service.AssessRecording(
        samples,
        sampleRate,
        referenceNotes
    )
}
```

### 前端使用範例

```javascript
// 在練習完成後提交評估
async function onPracticeCompleted(audioBlob) {
    // 1. 讀取音訊檔案
    const arrayBuffer = await audioBlob.arrayBuffer();
    const audioContext = new AudioContext();
    const audioBuffer = await audioContext.decodeAudioData(arrayBuffer);
    
    // 2. 取得參考音符
    const referenceNotes = state.referenceNotes;
    
    // 3. 送出到後端進行分析
    const result = await api('/assessments/analyze', {
        method: 'POST',
        body: JSON.stringify({
            audio_data: arrayBufferToBase64(arrayBuffer),
            reference_notes: referenceNotes,
            sample_rate: audioBuffer.sampleRate
        })
    });
    
    // 4. 顯示結果
    displayAssessmentResult(result);
}
```

## 參考文獻

1. **音高偵測：** "A Comparative Study of Pitch Detection Algorithms" by N. H. B. M. et al.
2. **音符合併：** "Music Transcription" by A. Klapuri, 2004
3. **母音子音分離：** "Vowel/Consonant Discrimination in Speech" by T. F. Y. et al.
4. **頻譜分析：** "Spectral Audio Signal Processing" by J. O. Smith, 2020

## 備註

- 母音子音分離功能為簡化實作，使用頻譜特徵進行估算
- 實際應用中可考慮使用機器學習模型提升準確度
- 目前只支援單聲道音訊分析
# Vocal Practice App

## 簡介

Vocal Practice App 是一個專為歌手和聲樂學生設計的練習工具。管理者上傳多聲部 MIDI 檔案至伺服器，定義歌曲段落結構，管理使用者帳號與評分資料。使用者登入後選擇歌曲、選擇要練習的聲部（Vocal Track）以及要跟著唱的伴奏聲部（Accompaniment Tracks），錄音後於前端執行 pitch detection 並與參考 MIDI 進行逐音符比對，最後以五種視覺化圖表呈現音高與時長偏差，並將評分結果上傳至伺服器與帳號關聯。

## 技術架構

- **版本**: 0.1
- **語言**: Go (後端), HTML/CSS/JavaScript (前端)
- **架構模式**: 領域驅動設計 (DDD)
- **認證方式**: JWT Token
- **主要組件**:
  - 後端：使用者管理、MIDI 檔案上傳/解析/儲存/串流、段落結構管理、歌詞管理、評分結果儲存與查詢
  - 前端：MIDI 多聲部播放與選擇、錄音 → pitch detection → MIDI 比對、五種視覺化分析圖表

## Backend Architecture & Design Specifications

## 1. Data Models

### User
Stores user account information for authentication and personal assessment history.

| Field | Type | Attributes | Description |
|-------|------|------------|-------------|
| id | UUID | Primary Key | Unique user ID |
| username | String | Not Null, Unique | Display name |
| email | String | Not Null, Unique | Login email |
| password_hash | String | Not Null | BCrypt hashed password |
| created_at | Timestamp | Not Null | Registration time |

### SongModel
Stores song metadata and associated MIDI file path. The MIDI file contains multiple tracks (vocal parts, accompaniment, etc.).

| Field | Type | Attributes | Description |
|-------|------|------------|-------------|
| id | UUID | Primary Key | Unique song ID |
| title | String | Not Null | Song title |
| artist | String | Not Null | Artist name |
| midi_file_path | String | Not Null | Path to uploaded MIDI file |
| tracks | Array of MIDITrack | Not Null | Parsed tracks from MIDI file |
| tempo_map | Array of TempoMapEntry | Nullable | Parsed tempo changes from MIDI |
| ticks_per_quarter | Integer | Nullable | MIDI resolution (TPQN) |
| created_at | Timestamp | Not Null | Creation time |

### MIDITrack
Represents a single track (voice/instrument part) extracted from a Standard MIDI File.

| Field | Type | Description |
|-------|------|-------------|
| id | UUID | Unique track ID |
| name | String | Track name (e.g., "Lead Vocal", "Piano", "Bass") |
| instrument | String | Instrument name (e.g., "Acoustic Grand Piano") |
| channel | Integer | MIDI channel number |
| notes | Array of MIDINote | Note sequence for this track |
| is_vocal | Boolean | Whether this track is designated as a vocal part |

### MIDINote
Represents a single MIDI note with pitch, velocity and timing.

| Field | Type | Description |
|-------|------|-------------|
| pitch | Integer | MIDI note number (0-127) |
| velocity | Integer | Note velocity / dynamics (0-127) |
| start_time | Float | Start time in seconds |
| end_time | Float | End time in seconds |

### TempoMapEntry
Represents a tempo change event within the MIDI file.

| Field | Type | Description |
|-------|------|-------------|
| tick | Integer | MIDI tick position |
| time_sec | Float | Absolute time in seconds |
| tempo_usec_per_qn | Integer | Microseconds per quarter note |

### SongStructureModel
Stores section and phrase cuts on the MIDI timeline defined by the admin. Used by users to select specific segments for practice.

| Field | Type | Attributes | Description |
|-------|------|------------|-------------|
| id | UUID | Primary Key | Unique structure ID |
| song_id | UUID | Foreign Key | Links to Songs table |
| parent_id | UUID | Foreign Key, Nullable | Parent section ID for hierarchical nesting |
| type | Enum | SECTION, PHRASE | Block type classification |
| title | String | Not Null | Display name (e.g., "Chorus A", "Line 1") |
| start_time | Float | Not Null | Start timestamp in seconds |
| end_time | Float | Not Null | End timestamp in seconds |
| start_tick | Integer | Not Null | Precise MIDI start point in Ticks |
| end_tick | Integer | Not Null | Precise MIDI end point in Ticks |
| order_index | Integer | Not Null | Sequential playback order |

### StructureNode
Represents a hierarchical structure node used in API responses, containing nested phrases and lyrics.

| Field | Type | Description |
|-------|------|-------------|
| (inherits SongStructure) | - | All SongStructure fields |
| phrases | Array of StructureNode | Nested phrase nodes |
| lyrics | String | Associated lyrics text |

### TrackLyrics
Stores lyrics text associated with a specific track and structure (section/phrase).

| Field | Type | Attributes | Description |
|-------|------|------------|-------------|
| id | UUID | Primary Key | Unique lyrics record ID |
| track_id | UUID | Foreign Key | Links to MIDI track |
| structure_id | UUID | Foreign Key | Links to SongStructure |
| lyrics | Text | Not Null | Lyrics text content |
| created_at | Timestamp | Not Null | Creation time |
| updated_at | Timestamp | Not Null | Last update time |

### UserAssessmentModel
Stores assessment results submitted by users after practicing. Each record is associated with a user account.

| Field | Type | Attributes | Description |
|-------|------|------------|-------------|
| id | UUID | Primary Key | Evaluation record ID |
| user_id | UUID | Foreign Key | Links to Users table |
| song_id | UUID | Foreign Key | Links to Songs table |
| track_id | UUID | Foreign Key | The vocal track being assessed |
| structure_id | UUID | Foreign Key, Nullable | Target segment (Null = Full song) |
| score | Float | Not Null | Evaluation score (0-100) |
| total_notes | Integer | Not Null | Total reference notes count |
| matched_notes | Integer | Not Null | Successfully matched notes count |
| average_pitch_deviation | Float | Nullable | Average pitch deviation in cents |
| average_duration_deviation | Float | Nullable | Average duration deviation in seconds |
| pitch_deviation | JSON / Array | Nullable | Per-note pitch deviation series in cents |
| duration_deviation | JSON / Array | Nullable | Per-note duration deviation series in seconds |
| note_comparison | JSON / Array | Nullable | Detailed note-to-note comparison data |
| segment_scores | JSON / Map | Nullable | Per-segment score breakdown |
| metadata | JSON / Map | Nullable | Song and structure metadata context |
| created_at | Timestamp | Not Null | Submission time |

---

## 2. API Endpoints

## 2.1 Admin Endpoints

### POST /api/v1/admin/users
* Action: Creates a new user account.
* Payload: `{"username": "singer01", "email": "singer01@example.com", "password": "securepassword"}`
* Response: Created user object (without password hash).

### GET /api/v1/admin/users
* Action: Lists all registered users (without password hashes).

### PUT /api/v1/admin/users/{user_id}/toggle-active
* Action: Toggles user active/inactive status.
* Response: Updated user object.

### POST /api/v1/admin/songs
* Action: Uploads a Standard MIDI File (.mid). Server parses the file and extracts multi-track note sequences plus tempo map. Auto-detects vocal tracks by track name keywords (vocal/voice/lead/singer).
* Payload: `multipart/form-data` with fields `title`, `artist` and file `midi_file`.
* Response: `{"song_id": "uuid", "tracks": [{"id": "uuid", "name": "Lead Vocal", "note_count": 120, "duration_sec": 180.0, "is_vocal": false}, ...]}`

### GET /api/v1/admin/songs
* Action: Lists all uploaded songs.

### GET /api/v1/admin/songs/{song_id}
* Action: Returns song metadata and parsed track list.

### PATCH /api/v1/admin/songs/{song_id}/tracks/{track_id}
* Action: Modifies track properties (e.g., `is_vocal` flag). Admin can manually override the auto-detected vocal designation.
* Payload: `{"is_vocal": true}`
* Response: Updated track object.

### DELETE /api/v1/admin/songs/{song_id}
* Action: Deletes a song and its associated MIDI file from the server.

### POST /api/v1/admin/songs/{song_id}/structures

* Action: Bulk creates sections and phrases for a song.
* Payload:

```json
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

### PUT /api/v1/admin/songs/{song_id}/structures/{structure_id}

* Action: Modifies timestamps or titles of an existing segment.
* Payload: `{"title": "Line 1 Edited", "start_time": 45.5}`

### DELETE /api/v1/admin/songs/{song_id}/structures/{structure_id}

* Action: Removes a segment. SECTION deletion cascade-deletes nested PHRASE units.

### POST /api/v1/admin/songs/{song_id}/lyrics

* Action: Batch creates or updates lyrics for multiple track/structure combinations.
* Payload:

```json
{
  "lyrics": [
    {
      "track_id": "uuid",
      "structure_id": "uuid",
      "lyrics": "Lyrics text for this section/track"
    }
  ]
}
```

### GET /api/v1/admin/tracks/{track_id}/lyrics

* Action: Lists all lyrics associated with a specific track.

### DELETE /api/v1/admin/songs/{song_id}/lyrics

* Action: Removes lyrics for a specific track/structure combination.
* Query params: `?track_id=xxx&structure_id=xxx`

## 2.2 User Endpoints

### POST /api/v1/auth/login
* Action: Authenticates user and returns JWT token.
* Payload: `{"email": "singer01@example.com", "password": "securepassword"}`
* Response: `{"token": "jwt_string", "user": {"id": "uuid", "username": "singer01"}}`
* Authorization: Subsequent requests require `Authorization: Bearer <token>` header.

### GET /api/v1/songs
* Action: Lists all available songs with track summaries.

### GET /api/v1/songs/{song_id}
* Action: Returns full song metadata, track list, and structure overview.

### GET /api/v1/songs/{song_id}/midi
* Action: Downloads the full MIDI file as binary `.mid`.

### GET /api/v1/songs/{song_id}/structures
* Action: Fetches nested structural data tree for frontend segment selection (includes lyrics).

```json
{
  "song_id": "song_uuid_123",
  "structures": [
    {
      "structure_id": "sec_uuid_01",
      "type": "SECTION",
      "title": "Verse",
      "start_time": 0.0,
      "end_time": 30.5,
      "lyrics": "Verse lyrics...",
      "phrases": [
        {
          "structure_id": "phr_uuid_01",
          "type": "PHRASE",
          "title": "Line 1",
          "start_time": 0.0,
          "end_time": 15.2,
          "lyrics": "Line 1 lyrics..."
        }
      ]
    }
  ]
}
```

### GET /api/v1/songs/{song_id}/tracks/{track_id}/audio
* Action: Streams audio for a specific track (if pre-generated).

### POST /api/v1/assessments/submit
* Action: Uploads practice assessment results (computed client-side) to the server, associated with the authenticated user's account.

```json
{
  "song_id": "uuid",
  "structure_id": "uuid_or_null",
  "track_id": "uuid",
  "score": 85.5,
  "pitch_deviation": [10, -5, 30, 120, -8],
  "duration_deviation": [0.05, -0.1, 0.3],
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
  "average_duration_deviation": 0.08,
  "segment_scores": {"section_uuid_01": 90, "section_uuid_02": 75},
  "metadata": {"song_title": "Test Song", "track_name": "Lead Vocal"}
}
```

* Response: `{"assessment_id": "uuid", "status": "saved"}`

### GET /api/v1/assessments
* Action: Lists the authenticated user's assessment history.
* Query params: `?song_id=xxx` `?limit=20` `?offset=0`

### GET /api/v1/assessments/stats
* Action: Returns aggregated statistics about the user's assessments.
* Response: `{"total_assessments": 10, "average_score": 85.5, "best_score": 95, "worst_score": 60}`

### GET /api/v1/assessments/{assessment_id}
* Action: Returns full details of a specific assessment including note comparison data.

### GET /api/v1/assessments/{assessment_id}/download
* Action: Downloads the assessment result as a JSON file.

### DELETE /api/v1/assessments/{assessment_id}
* Action: Deletes a specific assessment record.

---

## 3. User Practice Flow

```
1. User logs in → receives JWT token
2. Browses song list → selects a song
3. Selects a Vocal Track to practice (single selection, the reference for evaluation)
4. Selects Accompaniment Tracks to sing along with (multiple selection allowed)
5. Structure tree displays SECTION/PHRASE hierarchy with lyrics; user clicks two items to set
   a continuous practice range (or skips this step for full-song practice)
6. Client downloads the full MIDI file from the server (`GET /songs/{id}/midi`) and parses it locally
7. Upon selecting a track + structure, the client generates reference data by filtering parsed MIDI notes:
   `parsedNotes.filter(n => n.track === trackIndex && n.start >= start - 0.05 && n.start < end)`
   Each reference note contains `{pitch, start, dur}`.
8. Headset detection runs → wired/bluetooth/none with appropriate warning
   (Bluetooth 100-300ms latency warning, no-headset feedback risk warning)
9. User clicks "Start Practice" → 3-beat count-in (3, 2, 1 visual + click)
10. Web Audio API synthesizer (triangle wave + lowpass filter + ADSR envelope)
    plays selected accompaniment notes within the chosen time range
11. Simultaneously MediaRecorder records the user's singing
12. Recording stops → accompaniment stops → pitch detection runs automatically
    (autocorrelation: 2048 window, 50ms hop, 0.01 RMS threshold, 50-2000Hz)
13. Client-side comparison: detected notes vs reference MIDI track
    ├── Pitch accuracy per note (cents deviation)
    └── Duration accuracy per note (seconds deviation)
14. Visualization dashboard displays:
    ├── Pitch Contour Overlay
    ├── Per-Note Deviation Bar Chart
    ├── Duration Comparison Chart
    ├── Segment Score Dashboard
    └── Waveform + MIDI Piano Roll
15. User reviews results → clicks "Submit" → results uploaded to server
16. Results stored under user's account → viewable in history
17. User can view aggregate stats, download individual results, or delete old records
```

---

## 4. Core Logic & Services

### 4.0 Client-side Reference Data Generation
* Capabilities:
  1. Client downloads the full MIDI file via `GET /api/v1/songs/{song_id}/midi`.
  2. Parses MIDI bytes client-side to extract all tracks and notes with `.track` index.
  3. Maps selected track UUID to MIDI track index by iterating `tracks[i].id === selectedTrackId`.
  4. Filters notes by track index and time range `[start - 0.05, end)` to produce reference data.
  5. No separate server endpoint needed for reference data—reduces server load and latency.

### 4.1 MIDI Parser Service (Server-side)

* Capabilities:
  1. Accepts Standard MIDI File (.mid) binary upload.
  2. Parses MIDI header (format, track count, ticks per quarter note).
  3. Reads tempo map events and computes absolute time for each tick.
  4. Iterates through all tracks, reading note-on/note-off events.
  5. Groups notes by track, converting delta ticks to absolute seconds using tempo map.
  6. Auto-detects vocal tracks (by track name keywords or designated metadata).
  7. Returns structured `SongModel` with `MIDITrack[]` containing parsed `MIDINote[]` and `TempoMapEntry[]`.

### 4.2 User Authentication Service (Server-side)

* Logic:
  1. Registers users with email + bcrypt-hashed password.
  2. Login validates credentials → issues JWT token with expiry.
  3. Middleware validates JWT on protected endpoints.
  4. Assessment submissions are automatically associated with authenticated user.

### 4.3 Client-side Pitch Detection (Browser)

* Logic:
  1. Records user audio via `MediaRecorder` (webm blob).
  2. Decodes audio blob to `AudioBuffer` via `AudioContext.decodeAudioData`.
  3. Applies autocorrelation pitch detection:
     - Window: 2048 samples, hop: 50ms
     - RMS silence threshold: < 0.01
     - Frequency range: 50Hz – 2000Hz
     - Correlation threshold: > 0.3
  4. Converts detected pitch (Hz) to MIDI note number: `12 × log2(freq/440) + 69`
  5. Segments continuous pitch into notes; groups consecutive frames where pitch changes < 2 semitones.

### 4.4 Client-side MIDI Comparison Engine

* Logic:
  1. Loads reference MIDI track (selected Vocal Track).
  2. Sorts both reference and detected sequences by `start_time`.
  3. Aligns reference notes to detected notes by time proximity (tolerance: 0.5s).
  4. Computes pitch deviation per matched pair in cents: `(refPitch - detectedPitch) × 100`.
  5. Computes duration deviation per matched pair in seconds: `refDuration - detectedDuration`.
  6. Calculates overall score: weighted combination of pitch accuracy and duration accuracy.
  7. Generates detailed note-to-note comparison report for visualization.

### 4.5 Client-side Visualization Engine

#### 4.5.1 Pitch Contour Overlay
- Canvas-based chart rendering.
- X-axis: time (seconds), Y-axis: MIDI note number.
- Blue solid line: reference MIDI pitch contour.
- Orange dashed line: detected user pitch contour.
- Red highlight regions: deviation > threshold.

#### 4.5.2 Per-Note Deviation Bar Chart
- Bar per reference note, height = |pitch deviation| in cents.
- Color coding: green (< 50 cents), yellow (50-100 cents), red (> 100 cents).
- Bars above axis = sharp, bars below axis = flat.

#### 4.5.3 Duration Comparison Chart
- Paired horizontal bars (reference vs user) per note.
- Shows if the user held notes too long or too short.

#### 4.5.4 Segment Score Dashboard
- Each Section/Phrase displays individual score (0-100).
- Progress bar or donut chart visualization.
- Click to expand per-note detail within that segment.

#### 4.5.5 Waveform + MIDI Piano Roll
- Audio waveform displayed at bottom.
- MIDI Piano Roll overlaid on top showing correct note positions.
- Users can visually verify what notes they should have sung at each point.

---

## 5. 使用方式

### 管理後台 (`ui-screens/admin-*.html`)
- 建立與管理使用者帳號（含啟用/停用）
- 上傳 MIDI 檔案（自動解析多聲部 Track、Tempo Map）
- 檢視/管理歌曲段落結構（Section / Phrase）
- 管理歌詞（批量新增/更新/刪除）

### 前台 (`index.html`、`ui-screens/user-practice.html`、`ui-screens/user-dashboard.html`)
- 登入帳號
- 選擇歌曲 → 瀏覽 MIDI Track → 選擇練習聲部與伴奏聲部
- 選擇練習段落（整首歌或特定段落）
- 跟著伴奏演唱 → 錄音 → 自動分析
- 檢視視覺化分析結果 → 上傳評分至伺服器
- 查閱歷史評分記錄、統計數據、下載/刪除評分

### 工具命令
```bash
# MIDI 解析檢查
go run cmd/midi_inspect/main.go

# MIDI vs WAV 比對報告（生成 HTML）
go run cmd/midi_vs_wav_report/main.go

# 生成專案快照
go run cmd/snapshot/main.go
```

### 啟動伺服器
```bash
go run cmd/server/main.go
```

### 執行測試
```bash
go test ./... -v
```

---

## 6. 未來發展方向

- 支援更多音頻分析功能
- Standard MIDI File 匯入增強（多格式相容）
- 使用者練習進度趨勢圖
- 社交分享功能（分享練習成果）
- 持久化儲存（PostgreSQL）
- 管理後台認證授權
- MIDI 轉 audio 功能（自動生成各 Track 參考音訊）
- 練習提醒與目標設定
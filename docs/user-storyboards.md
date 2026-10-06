# User Storyboards — Practicing for a Performance

## Why this document exists

These are not requirements — they're a way of thinking through the app from inside a
learner's head before deciding what to build next. Each storyboard follows one persona
through a real session: what she's trying to accomplish, what information she's hunting for
at each step, how she actually clicks/taps through the app as it exists *today* (verified
against the current code, not assumed), and where she hits a wall. The "Disappointment"
notes are the point of the exercise — they're what turns into the improvement list at the
end.

**Persona: Mei.** Mid-20s, sings for fun, has been asked to perform a song at a friend's
wedding in three weeks. Moderate experience — can hold a tune, reads simple lyrics sheets,
has never had formal training. Practices alone, at night, on a laptop with a USB headset
mic, usually for 20–30 minutes before she's tired or self-conscious about the neighbors.

---

## Storyboard 1 — First contact: "Where do I even start?"

**Goal:** Pick the song, figure out which part she's actually responsible for, and get a
first attempt in without feeling lost.

1. Mei logs in, picks her song from the dropdown. The page loads the vocal-track list and
   structure tree. *She's looking for: "which track is mine?"* — track names come straight
   from the MIDI file's track names (whatever the admin who uploaded it typed, or the
   auto-detected label), so if the song has tracks named `Track 3` and `Melody`, she has to
   guess. There's no description, no "recommended for beginners" or "this is the part sung
   by the bride" hint — just raw track names.
2. She picks a track, sees the SECTION/PHRASE tree with lyrics attached to some phrases.
   *She's looking for: "how long is this whole thing, and which part should I practice
   first?"* — the tree shows title/time range per row, but nothing indicates difficulty,
   nothing marks "you haven't tried this part yet" vs "you've already nailed this one."
   Every phrase looks equally untouched, every time.
3. She clicks a phrase to select it (the whole-section-vs-phrase toggle from `selection.js`
   works correctly — this part is solid, per the fixes made in an earlier session). Lyrics
   for that phrase show up in the side panel.
4. She clicks "開始練習." A "準備麥克風中" message appears, then (per the recent rework) a
   3-2-1 visual countdown and the backing track start together, in tempo. She sings. A
   score shows up: `82`.

**Disappointment points:**
- **D1.1 — No sense of scope.** She has no way to see "this song has 6 sections, here's
  roughly how hard each one looks" before diving in. She just picks the first thing and
  hopes.
- **D1.2 — The score is a number with no context.** `82` — out of what? Good for a first
  try? She has nothing to compare it to (see Storyboard 3 — there's no working history to
  compare against anyway).
- **D1.3 — No "what went wrong" summary in plain language.** The pitch-deviation chart and
  note-comparison table are there, but they're built for someone who already knows what
  "cents" and "matched/missed" mean. Mei doesn't know if `-45 cents` on one note is "barely
  noticeable" or "very flat."

---

## Storyboard 2 — The practice loop itself: "Did it hear me right?"

**Goal:** Get several clean attempts at a tricky phrase in a row, each one actually starting
when she expects it to.

1. Mei re-selects the same phrase and hits "開始練習" again. Status text says "準備麥克風中"
   — she now knows to wait a beat before the count-in starts (this is new; previously the
   count-in and backing track started immediately and the mic negotiation happened silently
   underneath, so the recording's actual start time could lag behind what the count-in
   implied — now fixed).
2. Count-in: 3, 2, 1, 🎤 開始唱！ — a click on each beat, then a distinct visual cue exactly
   when she should come in. *She's checking: "did I come in right on the beat, or a hair
   late?"* — the click-to-note gap is now a full beat interval regardless of song tempo, so
   for the first time this actually matches what her ear expects from a normal count-in.
3. She sings. A "🔴 錄音中" badge is now visible the whole time (also new) — she can glance
   over and confirm the app is actually listening, not just playing music at her.
4. She finishes. Status says "分析完成，已加入練習紀錄" — reassuring in the moment... except
   (see Storyboard 3) there is currently nowhere in the app that actually shows her that
   record again, so the reassurance is a dead end.
5. She wants to try again *immediately*, from the same spot, without re-picking the section
   from the tree. She has to scroll back up to the structure list and click the same phrase
   again — there's no "🔁 重新練習這句" button right next to the result she just got.

**Disappointment points:**
- **D2.1 — No quick-retry.** After seeing a result, the fastest path back to "try that exact
  phrase again" is scrolling away from the result to the tree and re-clicking. For a phrase
  she's drilling 5–10 times in a row, this adds real friction every single time.
- **D2.2 — No slow-practice option.** If a run of notes is too fast for her to nail cleanly,
  there is no way to slow the backing track down for practice (no tempo/playback-rate
  control anywhere in `assets/js/audio.js` or the UI) — she can only ever practice at full
  speed or not at all.
- **D2.3 — Lyrics panel doesn't track live.** The lyrics for the selected phrase are shown
  once, statically, before she starts singing — nothing highlights which word she should be
  on as the backing track plays, which matters most for exactly the kind of piece with dense
  or unfamiliar lyrics (foreign-language songs, tongue-twister sections) where losing her
  place mid-line is the actual failure mode, not pitch.

---

## Storyboard 3 — "Am I getting better?" (the one that surfaced the biggest gap)

**Goal:** A few days later, check whether she's actually improving before deciding whether
she needs more practice time or is ready.

1. Mei logs in and looks for something like a history or dashboard link. She finds
   `user-dashboard.html`, which has a "練習統計" sidebar (45 practice sessions, 82 average
   score, 12 songs, 7-day streak) and a "練習記錄" list with score badges, note-match
   counts, and dates.
2. She reads it as her real progress — that's what a stats dashboard is *for*. She's
   thinking: "7-day streak, nice, and my average is 82, better than I thought."
3. **This is all fabricated.** Every number on that page — the 45/82/12/7 stats, all four
   history entries, the storage-usage bar, the "local vs. cloud sync" badges, the two
   unread messages congratulating her on a streak and suggesting she work on a chorus's
   pitch — is hardcoded directly in `ui-screens/user-dashboard.html`. None of it calls
   `GET /api/v1/assessments`, `/assessments/stats`, or anything else the backend actually
   exposes. Every button on the page (`downloadCloud`, `syncAll`, `exportAllData`,
   `cleanupOldRecords`, `deleteAssessment`) is an `alert()` stub that changes nothing real.
   The page is a visual prototype that was never wired up, and there is currently no code
   path anywhere in the app that lets a user see their *actual* saved practice history.

**Disappointment points:**
- **D3.1 (severe) — The one "progress" page in the app shows numbers that have nothing to do
  with what she actually did.** This is worse than having no history feature at all: an
  empty state honestly says "you haven't practiced enough to show anything yet"; a fake
  dashboard actively misinforms her about her own performance. If she happens to *not* match
  the fictional "7-day streak" messaging emotionally, or notices the numbers never change no
  matter what she does, trust in the whole app takes the hit — not just this one page.
- **D3.2 — Even setting aside the fakery, there's no real UI consuming the assessment
  history endpoints at all.** `GET /assessments`, `/assessments/stats`,
  `/assessments/{id}`, `/assessments/{id}/download`, `DELETE /assessments/{id}` are all
  implemented and tested on the backend (per `internal/handler/assessments.go` and its
  tests) and, since the last session's fix, actually get written to on every practice
  attempt via `POST /assessments/submit` — but nothing in `assets/js/` ever reads them back.
  The data exists; there's no window onto it.
- **D3.3 — No per-song or per-section trend.** Even a real version of this page, as designed
  in the mockup, is a flat list of individual attempts — there's nothing that says "here's
  your Chorus specifically, attempt 1 → attempt 8, trending up" which is the actual question
  Mei has three weeks out from a performance: not "what was my score yesterday" but "is this
  phrase getting better."

---

## Storyboard 4 — The night before: "Just let me drill the one hard bit"

**Goal:** With one evening left, spend all remaining practice time on the single phrase
she's least confident about, and get out the door to bed at a reasonable hour.

1. Mei opens the song. She already knows which phrase is the problem (the bridge's big jump)
   from doing this the past three weeks — but the app doesn't know that, because nothing
   she's done has ever been recorded anywhere she can see (Storyboard 3). She has to
   remember it herself.
2. She selects the bridge phrase and drills it repeatedly (see D2.1 — each repeat means
   leaving the result view and re-selecting from the tree).
3. She wants one more thing tonight specifically: to hear just her own voice back,
   isolated, to check if a particular note is really flat or if it's her imagination. The
   replay button (`showReplayButton`) plays back her recorded take — good, this exists and
   works — but it plays the whole take start to finish; she can't jump straight to the
   3 seconds around the note she's worried about, so she replays the whole phrase every time
   just to hear one word.
4. She closes the laptop still not entirely sure if she's ready, because the app never told
   her anything that amounted to "ready" or "not ready" — just a series of disconnected
   session scores with no throughline.

**Disappointment points:**
- **D4.1 — No persistent "known weak spot" marker.** Nothing lets her (or the app, from her
  own history) flag "this phrase is the one I keep messing up" so it surfaces first next
  session instead of relying on her memory.
- **D4.2 — No seek/scrub on the replay.** Reviewing one moment means replaying everything.
- **D4.3 — No readiness signal.** Even a simple "your last 3 attempts on this section
  averaged 91, up from 68 a week ago" would answer the actual question she came in with.
  This is downstream of D3.1/D3.2 — it can't exist until real history exists.

---

## Improvement opportunities, ranked by how many storyboards they unblock

1. **Wire a real practice-history view to the real API.** (Fixes D3.1, D3.2 directly;
   unblocks D3.3, D4.1, D4.3.) This is the highest-leverage fix in the whole set: it's the
   precondition for almost every "am I improving" question a performer-in-training actually
   has, the backend fully supports it already, and the current state (a fabricated
   dashboard) is actively worse than having nothing.
2. **Quick-retry on a result.** (Fixes D2.1.) Small, contained, high-frequency-use fix.
3. **Plain-language result summary.** (Fixes D1.2/D1.3.) Translate the existing
   pitch/duration deviation numbers into a sentence a non-technical singer can act on, using
   data the app already computes.
4. **Per-section/phrase trend, once real history exists.** (Fixes D3.3, D4.3.) Depends on #1.
5. **Weak-spot marking.** (Fixes D4.1.) Depends on #1 — needs real history to compute
   "this phrase's average is lower than the others" from.
6. **Replay seek/scrub.** (Fixes D4.2.) Self-contained, moderate effort (needs a scrub UI on
   the existing `<audio>` element).
7. **Slow-practice / tempo control.** (Fixes D2.2.) Larger effort — touches the Web Audio
   scheduling in `audio.js` and the MIDI-to-audio-time math throughout, closer in scope to
   the count-in timing work than a quick fix.
8. **Live lyrics tracking.** (Fixes D2.3.) Larger effort — needs word-level or line-level
   timing data that doesn't currently exist anywhere in the lyrics model (lyrics are stored
   as one text blob per phrase, not per-word timed).
9. **Track/section guidance for new songs.** (Fixes D1.1.) Content/authoring problem more
   than a code problem — would need admins to add descriptions, not something the practice
   flow itself can invent.

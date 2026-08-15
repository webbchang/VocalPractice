# Section & Phrase Selection — Logic & Implementation Notes

> Reference for refactoring the "段落與句子" (section/phrase) selection on the
> user-practice page. Current code treats a hierarchy as a flat index range;
> this document explains the existing logic and proposes a set-based replacement.

---

## 1. Where the logic lives

| Concern | File | Key symbols (line refs) |
|---|---|---|
| State | `assets/js/state.js` | `selectedStructure`, `selectedPhraseIds`, `selectedPhrasesData`, `flatItems`, `selectionRange` (~L10–24) |
| Flat list build | `assets/js/practice-business.js` | `buildFlatItems()` (L41–52) |
| Selection rules | `assets/js/practice-business.js` | `selectStructure()` (L238–287), `computeSelectedStructure()` (L57–129) |
| Selection UI update | `assets/js/practice-business.js` | `updateSelectionUI()` private (L289–309) |
| Render list | `assets/js/practice-ui.js` | `renderStructureList()` (L156–218), `isItemSelected()` (L108–114), `isSectionImplicitlySelected()` (L119–153) |
| Exported UI update | `assets/js/practice-ui.js` | `updateSelectionUI()` exported (L260–276, effectively dead) |
| Auto-select on load | `assets/js/practice-business.js` | `selectTrack()` (L201–204) |
| Inline handlers / bridges | `ui-screens/user-practice.html` | `window.selectStructure`, `window.__selectStructure` (L181, L197, L213–215) |
| Styles | `ui-screens/user-practice.css` | `.structure-option` (L230–240), `.selected` (L246–249), `.phrased` (L251–253) |
| Locked behavior | `assets/js/__tests__/*.test.js` | `practice-business.test.js` L194–325, `practice-ui.test.js` L168–215 |

---

## 2. Current data model

Selection is stored as a **continuous index range** over a **flat array**:

```js
// state.js
selectionRange: { from: null, to: null }   // indices into flatItems
flatItems: []   // [{ type:'section'|'phrase', data, section }, ...]
selectedStructure: null   // { id, start, end, title } derived from the range
selectedPhraseIds: []     // derived
selectedPhrasesData: []   // derived
```

`flatItems` is built by `buildFlatItems()` (practice-business.js L41–52):
it pushes each section, then each of that section's phrases, one after another.

```text
[ Verse(s0), p1(1), p2(2), Chorus(s3), p3(4), p4(5) ]
```

So the hierarchy is flattened into a 1-D list and the "selection" is just
`slice(from, to + 1)`.

---

## 3. Current selection algorithm (`selectStructure`, L238–287)

1. Find the clicked item's index in `flatItems` (`clickedIdx`).
2. If no range yet → `{ from: clickedIdx, to: clickedIdx }`.
3. If `clickedIdx` is **inside** the current range:
   - it's the **first** edge → shrink range to `[min+1, max]`
   - it's the **last** edge → shrink range to `[min, max-1]`
   - it's in the **middle** → reset range to `[clickedIdx, clickedIdx]`
4. If `clickedIdx` is **outside** the range → extend to the new min/max.

Then `computeSelectedStructure()` (L57–129) derives `selectedStructure`,
`selectedPhraseIds`, `selectedPhrasesData` from the slice:

- For every selected section it pulls in **all** of that section's phrases.
- `title` becomes `"First ∼ Last（N項, M句）"` when more than one item.

### 3.1 `computeSelectedStructure` detail (L57–129)

- Returns early (clears selection) if `from === null || to === null || from > to`.
- `selected = flatItems.slice(from, to + 1)`.
- Collects phrases: explicit phrase items, plus **all** phrases of any selected
  section item (deduped by id).
- Computes `minStart`/`maxEnd` across every selected item (section or phrase use
  the same `start_time`/`end_time` accessors).
- Builds `title`:
  - single item → that item's title
  - multiple → `"${firstTitle} ∼ ${lastTitle}（${count}項${totalPhrases > count ? ', ' + totalPhrases + '句' : ''}）"`
- Writes `state.selectedStructure = { id: from + '-' + to, title, start, end }`.

---

## 4. Current rendering (`renderStructureList`, L156–218)

For each `flatItems[i]`:
- `isItemSelected(i)` → direct highlight if `i` is inside `selectionRange`.
- For a section, additionally compute `isSectionImplicitlySelected(item.data.id)`
  (L119–153): the section is highlighted only if **every** of its phrase indices
  falls inside `selectionRange`.

Both conditions are OR-ed into the `selected` CSS class.

```js
// practice-ui.js L193–204 (simplified)
if (item.type === 'section') {
  const isImplicit = !isSelected && isSectionImplicitlySelected(item.data.id);
  html += `<div class="structure-option ${isSelected || isImplicit ? 'selected' : ''}" ...>`;
}
```

Every row (section and phrase) also renders its own ▶️ (`playRangeWithBeats`)
and ⏹️ (`window.__stopPlayback`) buttons.

---

## 5. Why it is "messy" (the actual bugs/UX problems)

1. **Two contradictory ways a section looks "selected".**
   - Directly, when its own index is in the range.
   - Implicitly, when all its phrases are in the range.
   Both produce the green highlight but mean different things, and the phrase
   rows underneath may or may not be lit depending on which path was taken.

2. **Selecting a section does NOT highlight its phrases**, even though those
   phrases are in `selectedPhrasesData` and the lyrics panel shows them.
   Selecting only the phrases *does* light the section (implicitly). The same
   effective practice range → two different visual states.

3. **Ranges cross section boundaries silently.** Clicking `p2` then `p3`
   yields range `[2,4]` = `p2, Chorus-header, p3`, which **excludes** the Verse
   header but **implicitly includes** `p4` (because the Chorus header is in the
   range). Title becomes `Line 2 ∼ Line 3` spanning a boundary.

4. **Undiscoverable click semantics.** Extend/shrink/reset depend on where you
   click relative to the invisible range; there is no checkbox, modifier key,
   or affordance.

5. **Flat visual layout.** Sections and phrases are siblings with near-identical
   styling; phrases only get `+24px` tall left padding. No grouping, no collapse,
   no indent guides. Every row carries its own ▶️/⏹️ buttons (clutter).

6. **Inconsistent time display** (L210): phrases *with* lyrics hide the end
   time; phrases without lyrics and all sections show it.

7. **Auto-selects the first section on load** (L201–204), so a whole section is
   pre-highlighted immediately — and then subject to the inconsistent rules.

8. **Code cruft.** Two `updateSelectionUI` functions (business L289 vs ui L260).
   Redundant `window.__selectStructure` / `window.selectStructure` bridge
   aliases (user-practice.html L213–215). The exported `updateSelectionUI` in
   `practice-ui.js` is effectively dead code.

---

## 6. Proposed new model (set-based, hierarchy-aware)

### 6.1 State

```js
// state.js — replace flatItems / selectionRange with:
selectedSectionIds: new Set(),   // sections whose WHOLE body is selected
selectedPhraseIds: new Set(),    // individually selected phrases
// derived (unchanged shape, easier to compute):
selectedStructure: null,         // { id, start, end, title }
selectedPhrasesData: [],
```

Rule of thumb: a phrase is "active" if it is in `selectedPhraseIds` OR its
parent section is in `selectedSectionIds`. A section is "fully selected" if it
is in `selectedSectionIds`.

**Helpers:**
```js
function findParentSection(phraseId) {
  for (const s of state.structures) {
    if ((s.phrases || []).some(p => p.id === phraseId)) return s;
  }
  return null;
}
function activePhraseIds() {
  const ids = new Set(state.selectedPhraseIds);
  for (const s of state.structures) {
    if (state.selectedSectionIds.has(s.id)) {
      (s.phrases || []).forEach(p => ids.add(p.id));
    }
  }
  return ids;
}
```

### 6.2 Selection handlers (replace `selectStructure`)

```js
export function selectSection(id) {
  const section = state.structures.find(s => s.id === id);
  if (!section) return;
  if (state.selectedSectionIds.has(id)) {
    // toggle OFF: drop section + any of its phrases
    state.selectedSectionIds.delete(id);
    (section.phrases || []).forEach(p => state.selectedPhraseIds.delete(p.id));
  } else {
    // toggle ON: select whole section
    state.selectedSectionIds.add(id);
    (section.phrases || []).forEach(p => state.selectedPhraseIds.add(p.id));
  }
  recomputeSelection();
}

export function selectPhrase(id) {
  const section = findParentSection(id);
  if (!section) return;
  if (state.selectedPhraseIds.has(id)) {
    state.selectedPhraseIds.delete(id);
    state.selectedSectionIds.delete(section.id); // no longer "whole section"
  } else {
    state.selectedPhraseIds.add(id);
    // if now all phrases of the section are selected → mark section whole
    if ((section.phrases || []).every(p => state.selectedPhraseIds.has(p.id)))
      state.selectedSectionIds.add(section.id);
  }
  recomputeSelection();
}
```

`recomputeSelection()` replaces `computeSelectedStructure()`: build
`selectedStructure` (min start / max end over all active phrases + whole
sections), and a human title like `"Verse（2 句）"` or
`"Verse + Chorus（4 句）"`.

```js
function recomputeSelection() {
  const active = [];
  for (const s of state.structures) {
    const whole = state.selectedSectionIds.has(s.id);
    const phrases = (s.phrases || []).filter(p =>
      whole || state.selectedPhraseIds.has(p.id));
    phrases.forEach(p => active.push(p));
  }
  if (active.length === 0) {
    state.selectedStructure = null;
    state.selectedPhrasesData = [];
    return;
  }
  const minStart = Math.min(...active.map(p => p.start_time));
  const maxEnd = Math.max(...active.map(p => p.end_time));
  const titles = active.map(p => p.title);
  let title;
  if (titles.length === 1) {
    title = titles[0];
  } else if (state.selectedSectionIds.size > 0) {
    title = [...state.selectedSectionIds]
      .map(sid => state.structures.find(s => s.id === sid).title)
      .join(' + ') + `（${active.length} 句）`;
  } else {
    title = `${titles[0]} … ${titles[titles.length - 1]}（${active.length} 句）`;
  }
  state.selectedStructure = { id: 'sel-' + Date.now(), title, start: minStart, end: maxEnd };
  state.selectedPhrasesData = active;
}
```

> Note: `selectedPhraseIds` should remain a `Set` internally; the rest of the
> app (lyrics panel, note grouping, `startPractice`) only reads
> `selectedPhraseIds.size`/iteration and `selectedPhrasesData`, so converting to
> an array at the boundary (or updating those few reads) is sufficient.

### 6.3 Rendering (replace `renderStructureList`)

Render **nested, collapsible** groups instead of a flat list:

```html
<div class="structure-group">
  <div class="structure-option section ${wholeSelected ? 'selected':''}"
       onclick="window.selectSection('Verse')">
    📂 Verse  <span class="structure-time">0:00 - 0:10</span>
    <button class="btn-play" onclick="event.stopPropagation();playRangeWithBeats(...)">▶️</button>
  </div>
  <div class="structure-children">
    <div class="structure-option phrase ${active ? 'selected':''}"
         onclick="window.selectPhrase('p1')">
      📄 Line 1  <span class="structure-time">0:00 - 0:05</span> ▶️
    </div>
    ...
  </div>
</div>
```

- One highlight rule: `selected` if the item is active (phrase) / whole (section).
- No more `isSectionImplicitlySelected()`.
- Keep a single ▶️ per item; route ⏹️ to one global control.
- Uniform time display on every row (always show `start - end`).

### 6.4 Remove

- `flatItems`, `selectionRange`, `buildFlatItems()`.
- `isItemSelected()`, `isSectionImplicitlySelected()`.
- The duplicate `updateSelectionUI` in `practice-ui.js`; keep one in
  `practice-business.js`.
- `window.__selectStructure` / `window.__selectTrack` bridge aliases
  (user-practice.html L213–215). Use the single `window.selectStructure` /
  `window.selectPhrase` already wired in L179–191.

---

## 7. Test impact (must be updated together with the refactor)

`practice-business.test.js` (L194–325) asserts the **flat-range** rules:
`selectionRange` equality, shrink/extend/reset behavior, implicit-phrase
inclusion when a section header is in range. These tests no longer apply —
replace with set-based assertions:

```js
it('selecting a section selects all its phrases', () => {
  businessModule.selectSection('s1');
  expect([...state.selectedSectionIds]).toEqual(['s1']);
  expect([...state.selectedPhraseIds].sort()).toEqual(['p1', 'p2']);
});

it('deselecting one phrase unmarks the whole section', () => {
  businessModule.selectSection('s1');
  businessModule.selectPhrase('p1');
  expect(state.selectedSectionIds.has('s1')).toBe(false);
  expect(state.selectedPhraseIds.has('p1')).toBe(false);
});

it('selecting every phrase of a section marks it whole', () => {
  businessModule.selectPhrase('p1');
  businessModule.selectPhrase('p2');
  expect(state.selectedSectionIds.has('s1')).toBe(true);
});
```

`practice-ui.test.js` (L168–215) asserts the implicit-section highlight. Replace
with nested-render assertions: section gets `selected` only when in
`selectedSectionIds`; phrase gets `selected` when active.

---

## 8. Suggested implementation order

1. Add `selectedSectionIds`/`selectedPhraseIds` (`Set`) to `state.js`; keep old
   fields temporarily for a diff-free transition if desired.
2. Rewrite `selectStructure` → `selectSection` + `selectPhrase` +
   `recomputeSelection` in `practice-business.js`; delete `buildFlatItems` and
   `computeSelectedStructure`.
3. Rewrite `renderStructureList` for nested/collapsible output in
   `practice-ui.js`; delete `isItemSelected`/`isSectionImplicitlySelected`.
4. Add CSS for `.structure-group` / `.structure-children` (indent + connector).
5. Remove duplicate `updateSelectionUI` and the `window.__*` bridges.
6. Update `user-practice.html` inline handlers to call `selectSection`/`selectPhrase`.
7. Update both test files to the set-based assertions.
8. Run `npm test` (vitest) and manually verify in the browser.

---

## 9. Quick reference: behavior matrix

| User action | Current (flat range) | Proposed (sets) |
|---|---|---|
| Click a section | Selects section index only; phrases not highlighted but included in range | Selects whole section; all child phrases highlighted |
| Click a phrase | Extends/shrinks numeric range depending on position | Toggles that phrase independently |
| Click section then a phrase in another section | Crosses boundary, includes intermediate header implicitly | Independent toggles; no implicit cross-section selection |
| Deselect one phrase of a fully-selected section | Resets nothing automatically (range math) | Section becomes partially selected (unmarked as whole) |
| Visual highlight rule | Direct index OR implicit-all-phrases (two rules) | Single rule: active/whole |

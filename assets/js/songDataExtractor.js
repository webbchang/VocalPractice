export function extractReferenceNotes(allNotes, selectedTrackIndex, rangeStart, rangeEnd) {
    if (!allNotes || !selectedTrackIndex === null || !rangeStart === null || !rangeEnd === null) {
        return [];
    }

    const referenceNotes = allNotes.filter(n =>
        n.track === selectedTrackIndex &&
        n.start >= rangeStart - 0.05 && // small tolerance
        n.start < rangeEnd
    ).map(n => ({
        pitch: n.pitch,
        start: n.start,
        dur: Math.min(n.dur, rangeEnd - n.start),
    })).filter(n => n.dur > 0.02);

    return referenceNotes;
}

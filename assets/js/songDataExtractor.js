function extractReferenceNotes(allNotes, selectedTrackIndex, rangeStart, rangeEnd) {
    if (!Array.isArray(allNotes) || selectedTrackIndex === null || selectedTrackIndex === undefined) {
        return [];
    }

    const safeRangeStart = Number.isFinite(rangeStart) ? rangeStart : 0;
    const safeRangeEnd = Number.isFinite(rangeEnd) ? rangeEnd : Infinity;

    return allNotes.filter(n =>
        n.track === selectedTrackIndex &&
        n.start >= safeRangeStart - 0.05 &&
        n.start < safeRangeEnd
    ).map(n => ({
        pitch: n.pitch,
        start_time: n.start,
        end_time: n.start + Math.min(n.dur, safeRangeEnd - n.start),
    })).filter(n => (n.end_time - n.start_time) > 0.02;
}

window.SongDataExtractor = {
    extractReferenceNotes,
};

// === Pitch Detection Module ===

/**
 * Pitch detection using autocorrelation
 */
export function detectPitch(audioBuffer) {
    const samples = audioBuffer.getChannelData(0);
    const sampleRate = audioBuffer.sampleRate;
    const windowSize = 2048;
    const hopSize = Math.floor(sampleRate * 0.05); // 50ms hop
    const silenceThreshold = 0.01;
    const correlationThreshold = 0.3;
    const minFreq = 50;
    const maxFreq = 2000;

    const notes = [];
    let currentNote = null;
    let consecutiveFrames = 0;

    for (let start = 0; start + windowSize < samples.length; start += hopSize) {
        const window = samples.slice(start, start + windowSize);

        // RMS energy
        let rms = 0;
        for (let i = 0; i < windowSize; i++) {
            rms += window[i] * window[i];
        }
        rms = Math.sqrt(rms / windowSize);

        if (rms < silenceThreshold) {
            // Silence - end current note if any
            if (currentNote) {
                currentNote.endTime = start / sampleRate;
                if (currentNote.pitch) {
                    // Merge if pitch close to previous
                    const lastNote = notes[notes.length - 1];
                    if (lastNote && Math.abs(lastNote.pitch - currentNote.pitch) < 2) {
                        lastNote.endTime = currentNote.endTime;
                    } else {
                        notes.push({ ...currentNote });
                    }
                }
                currentNote = null;
            }
            consecutiveFrames = 0;
            continue;
        }

        // Autocorrelation
        const correlation = [];
        const minPeriod = Math.floor(sampleRate / maxFreq);
        const maxPeriod = Math.ceil(sampleRate / minFreq);

        for (let period = minPeriod; period <= maxPeriod; period++) {
            let corr = 0;
            for (let i = 0; i < windowSize - period; i++) {
                corr += window[i] * window[i + period];
            }
            correlation.push({ period, value: corr / windowSize });
        }

        // Find peak
        let maxCorr = 0;
        let bestPeriod = 0;
        for (const c of correlation) {
            if (c.value > maxCorr) {
                maxCorr = c.value;
                bestPeriod = c.period;
            }
        }

        if (maxCorr < correlationThreshold) {
            if (currentNote) {
                currentNote.endTime = start / sampleRate;
                if (currentNote.pitch) {
                    const lastNote = notes[notes.length - 1];
                    if (lastNote && Math.abs(lastNote.pitch - currentNote.pitch) < 2) {
                        lastNote.endTime = currentNote.endTime;
                    } else {
                        notes.push({ ...currentNote });
                    }
                }
                currentNote = null;
            }
            continue;
        }

        // Convert to frequency and MIDI note
        const freq = sampleRate / bestPeriod;
        const midiNote = 12 * Math.log2(freq / 440) + 69;
        const roundedPitch = Math.round(midiNote);

        if (roundedPitch < 40 || roundedPitch > 93) {
            // Outside vocal range, skip
            consecutiveFrames = 0;
            continue;
        }

        const time = start / sampleRate;

        if (!currentNote) {
            currentNote = {
                pitch: roundedPitch,
                startTime: time,
                endTime: time + (hopSize / sampleRate)
            };
            consecutiveFrames = 1;
        } else {
            // Check if pitch changed significantly
            if (Math.abs(currentNote.pitch - roundedPitch) >= 2) {
                // End previous note
                currentNote.endTime = time;
                if (currentNote.pitch) {
                    const lastNote = notes[notes.length - 1];
                    if (lastNote && Math.abs(lastNote.pitch - currentNote.pitch) < 2 && time - lastNote.startTime < 0.3) {
                        lastNote.endTime = currentNote.endTime;
                    } else {
                        notes.push({ ...currentNote });
                    }
                }
                currentNote = {
                    pitch: roundedPitch,
                    startTime: time,
                    endTime: time + (hopSize / sampleRate)
                };
                consecutiveFrames = 1;
            } else {
                currentNote.endTime = time + (hopSize / sampleRate);
                consecutiveFrames++;
            }
        }
    }

    // Finalize last note
    if (currentNote && currentNote.pitch) {
        const lastNote = notes[notes.length - 1];
        if (lastNote && Math.abs(lastNote.pitch - currentNote.pitch) < 2 &&
            currentNote.startTime - lastNote.endTime < 0.3) {
            lastNote.endTime = currentNote.endTime;
        } else {
            notes.push({ ...currentNote });
        }
    }

    // Filter out very short notes (< 100ms)
    return notes.filter(n => (n.endTime - n.startTime) > 0.1);
}

/**
 * Groups consecutive same-pitch notes whose gap < 50ms into one merged note.
 * Returns an array of { pitch, startTime, endTime, eventIdx[] }.
 */
export function mergeSamePitchNotes(notes) {
    if (!notes || notes.length === 0) return [];

    const sorted = [...notes].sort((a, b) => (a.start_time ?? a.startTime) - (b.start_time ?? b.startTime));

    const merged = [];
    let current = {
        pitch: sorted[0].pitch,
        startTime: sorted[0].start_time ?? sorted[0].startTime,
        endTime: sorted[0].end_time ?? sorted[0].endTime,
        eventIdx: [0]
    };

    const gapThreshold = 0.05;

    for (let i = 1; i < sorted.length; i++) {
        const n = sorted[i];
        const s = n.start_time ?? n.startTime;
        const e = n.end_time ?? n.endTime;
        const gap = s - current.endTime;

        if (n.pitch === current.pitch && gap >= 0 && gap <= gapThreshold) {
            if (e > current.endTime) current.endTime = e;
            current.eventIdx.push(i);
        } else {
            merged.push(current);
            current = { pitch: n.pitch, startTime: s, endTime: e, eventIdx: [i] };
        }
    }
    merged.push(current);
    return merged;
}

/**
 * Compares merged reference note groups against detected notes.
 * If a merged group has >=80% overlap with a same-pitch detected note,
 * all events in that group are marked "matched" with group-level deviations.
 */
export function compareMergedNotes(reference, mergedRef, detected) {
    const detSorted = [...detected].sort((a, b) => a.startTime - b.startTime);

    const matched = reference.map(ref => ({
        refPitch: ref.pitch,
        userPitch: 0,
        refStart: ref.start_time ?? ref.startTime,
        refEnd: ref.end_time ?? ref.endTime,
        userStart: 0,
        userEnd: 0,
        pitchDeviationCents: 0,
        durationDeviationSec: 0,
        matchStatus: 'missed'
    }));

    const usedDetected = new Set();
    let matchedCount = 0;
    let totalPitchDev = 0;
    let totalDurationDev = 0;

    for (const mg of mergedRef) {
        const mgLen = mg.endTime - mg.startTime;
        let bestIdx = -1;
        let bestOverlap = 0;

        for (let i = 0; i < detSorted.length; i++) {
            if (usedDetected.has(i)) continue;
            const det = detSorted[i];
            if (det.pitch !== mg.pitch) continue;

            const overlapStart = Math.max(mg.startTime, det.startTime);
            const overlapEnd = Math.min(mg.endTime, det.endTime);
            if (overlapEnd <= overlapStart) continue;

            const overlapLen = overlapEnd - overlapStart;
            const detLen = det.endTime - det.startTime;
            const shorterLen = Math.min(mgLen, detLen);
            if (shorterLen <= 0) continue;

            const ratio = overlapLen / shorterLen;
            if (ratio > bestOverlap) {
                bestOverlap = ratio;
                bestIdx = i;
            }
        }

        if (bestIdx >= 0 && bestOverlap >= 0.8) {
            usedDetected.add(bestIdx);
            const det = detSorted[bestIdx];

            const pitchDev = (mg.pitch - det.pitch) * 100;
            const durDev = mgLen - (det.endTime - det.startTime);

            for (const eidx of mg.eventIdx) {
                matched[eidx].userPitch = det.pitch;
                matched[eidx].userStart = det.startTime;
                matched[eidx].userEnd = det.endTime;
                matched[eidx].pitchDeviationCents = pitchDev;
                matched[eidx].durationDeviationSec = durDev;
                matched[eidx].matchStatus = 'matched';
            }

            matchedCount += mg.eventIdx.length;
            totalPitchDev += Math.abs(pitchDev) * mg.eventIdx.length;
            totalDurationDev += Math.abs(durDev) * mg.eventIdx.length;
        }
    }

    const avgPitchDev = matchedCount > 0 ? totalPitchDev / matchedCount : 0;
    const avgDurationDev = matchedCount > 0 ? totalDurationDev / matchedCount : 0;

    const pitchScore = Math.max(0, 100 - avgPitchDev * 0.5);
    const durationScore = Math.max(0, 100 - avgDurationDev * 50);
    const overallScore = pitchScore * 0.7 + durationScore * 0.3;

    const pitchDeviations = matched.map(m => m.pitchDeviationCents);
    const durationDeviations = matched.map(m => m.durationDeviationSec);

    return {
        score: Math.round(overallScore),
        totalNotes: reference.length,
        matchedNotes: matchedCount,
        averagePitchDeviation: Math.round(avgPitchDev * 10) / 10,
        averageDurationDeviation: Math.round(avgDurationDev * 100) / 100,
        pitchDeviation: pitchDeviations,
        durationDeviation: durationDeviations,
        noteComparison: matched
    };
}
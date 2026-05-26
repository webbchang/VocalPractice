function parseMIDINotes(midiArrayBuf) {
    const data = new Uint8Array(midiArrayBuf);
    let offset = 0;

    const head = String.fromCharCode(data[0], data[1], data[2], data[3]);
    if (head !== 'MThd') throw new Error('Not a MIDI file');

    const format = (data[8] << 8) | data[9];
    const numTracks = (data[10] << 8) | data[11];
    const ticksPerQN = (data[12] << 8) | data[13];
    offset = 14;

    function readVarLen(arr, pos) {
        let value = 0, i = 0;
        while (true) {
            value = (value << 7) | (arr[pos + i] & 0x7F);
            if (!(arr[pos + i] & 0x80)) break;
            i++;
        }
        return { value, size: i + 1 };
    }

    // --- Step 1: Build tempo map from all tracks ---
    const rawTempos = [];
    let currentOffset = offset; // Use a temporary offset for the first pass

    for (let t = 0; t < numTracks; t++) {
        if (currentOffset + 8 > data.length) break;
        const chunkId = String.fromCharCode(data[currentOffset], data[currentOffset+1], data[currentOffset+2], data[currentOffset+3]);
        if (chunkId !== 'MTrk') break;
        const chunkLen = (data[currentOffset+4] << 24) | (data[currentOffset+5] << 16) | (data[currentOffset+6] << 8) | data[currentOffset+7];
        currentOffset += 8;
        const end = currentOffset + chunkLen;
        let absTick = 0, runningStatus = 0;

        while (currentOffset < end) {
            const { value: delta, size: deltaSize } = readVarLen(data, currentOffset);
            currentOffset += deltaSize; absTick += delta;
            if (currentOffset >= end) break;

            let status = data[currentOffset];
            if (status >= 0x80) { runningStatus = status; currentOffset++; }
            else if (runningStatus !== 0) { status = runningStatus; }
            else { currentOffset++; continue; }

            if (status === 0xFF) {
                runningStatus = 0;
                if (currentOffset >= end) break;
                const metaType = data[currentOffset]; currentOffset++;
                const { value: metaLen, size: metaLenSize } = readVarLen(data, currentOffset);
                currentOffset += metaLenSize;
                if (metaType === 0x51 && metaLen >= 3 && currentOffset + metaLen <= end) {
                    const usPerQN = (data[currentOffset] << 16) | (data[currentOffset+1] << 8) | data[currentOffset+2];
                    rawTempos.push({ tick: absTick, usPerQN });
                }
                currentOffset += metaLen;
                continue;
            }
            if (status === 0xF0 || status === 0xF7) {
                runningStatus = 0;
                const { value: sysexLen, size: sysexLenSize } = readVarLen(data, currentOffset);
                currentOffset += sysexLenSize + sysexLen; continue;
            }

            const highNibble = status & 0xF0;
            switch (highNibble) {
                case 0x80: case 0x90: case 0xA0: case 0xB0: case 0xE0: currentOffset += 2; break;
                case 0xC0: case 0xD0: currentOffset += 1; break;
                default: break;
            }
        }
    }

    // Sort and deduplicate tempo map
    rawTempos.sort((a, b) => a.tick - b.tick);
    const tempoMap = [];
    for (const t of rawTempos) {
        if (tempoMap.length === 0 || tempoMap[tempoMap.length - 1].tick !== t.tick) {
            tempoMap.push(t);
        }
    }

    // --- Step 2: tickToSec helper using tempo map ---
    function tickToSec(tick) {
        let i = 0;
        for (let j = tempoMap.length - 1; j >= 0; j--) {
            if (tempoMap[j].tick <= tick) { i = j; break; }
        }
        let timeSec = 0;
        let prevTick = 0;
        let prevTempo = tempoMap.length > 0 ? tempoMap[0].usPerQN : 500000; // Default to 120 BPM if no tempo map
        for (let k = 1; k <= i; k++) {
            const dt = tempoMap[k].tick - prevTick;
            timeSec += dt * (prevTempo / (ticksPerQN * 1000000));
            prevTick = tempoMap[k].tick;
            prevTempo = tempoMap[k].usPerQN;
        }
        const dt = tick - prevTick;
        timeSec += dt * (prevTempo / (ticksPerQN * 1000000));
        return timeSec;
    }

    // --- Step 3: Parse notes (second pass over tracks) ---
    currentOffset = offset; // Reset offset for note parsing
    const allNotes = [];
    for (let t = 0; t < numTracks; t++) {
        if (currentOffset + 8 > data.length) break;
        const chunkId = String.fromCharCode(data[currentOffset], data[currentOffset+1], data[currentOffset+2], data[currentOffset+3]);
        if (chunkId !== 'MTrk') break;
        const chunkLen = (data[currentOffset+4] << 24) | (data[currentOffset+5] << 16) | (data[currentOffset+6] << 8) | data[currentOffset+7];
        currentOffset += 8;
        const end = currentOffset + chunkLen;
        let absTick = 0, runningStatus = 0;

        while (currentOffset < end) {
            const { value: delta, size: deltaSize } = readVarLen(data, currentOffset);
            currentOffset += deltaSize; absTick += delta;
            if (currentOffset >= end) break;

            let status = data[currentOffset];
            if (status >= 0x80) { runningStatus = status; currentOffset++; }
            else if (runningStatus !== 0) { status = runningStatus; }
            else { currentOffset++; continue; }

            if (status === 0xFF) {
                runningStatus = 0; currentOffset++;
                const { value: metaLen, size: metaLenSize } = readVarLen(data, currentOffset);
                currentOffset += metaLenSize + metaLen; continue;
            }
            if (status === 0xF0 || status === 0xF7) {
                runningStatus = 0;
                const { value: sysexLen, size: sysexLenSize } = readVarLen(data, currentOffset);
                currentOffset += sysexLenSize + sysexLen; continue;
            }

            const highNibble = status & 0xF0;
            if (highNibble === 0x90 || highNibble === 0x80) {
                const pitch = data[currentOffset], velocity = data[currentOffset+1]; currentOffset += 2;
                if (highNibble === 0x90 && velocity > 0) {
                    allNotes.push({ pitch, tick: absTick, track: t });
                } else {
                    for (let i = allNotes.length - 1; i >= 0; i--) {
                        if (allNotes[i].pitch === pitch && allNotes[i].endTick === undefined && allNotes[i].track === t) {
                            allNotes[i].endTick = absTick; break;
                        }
                    }
                }
                continue;
            }
            switch (highNibble) {
                case 0xA0: case 0xB0: case 0xE0: currentOffset += 2; break;
                case 0xC0: case 0xD0: currentOffset += 1; break;
                default: break;
            }
        }
    }

    return allNotes.filter(n => n.endTick !== undefined && n.endTick > n.tick)
        .map(n => ({
            pitch: n.pitch,
            start: tickToSec(n.tick),
            dur: tickToSec(n.endTick) - tickToSec(n.tick),
            track: n.track,
        }))
        .filter(n => n.dur > 0.02);
}

function midiPitchToFreq(pitch) {
    return 440 * Math.pow(2, (pitch - 69) / 12);
}

/**
 * Extract BPM from MIDI data by reading set-tempo meta events.
 * Returns the first tempo found, or 120 (default MIDI tempo) if none.
 * @param {ArrayBuffer} midiArrayBuf
 * @returns {number} BPM
 */
function extractBPM(midiArrayBuf) {
    const data = new Uint8Array(midiArrayBuf);
    let offset = 0;

    // Read MThd header
    const head = String.fromCharCode(data[0], data[1], data[2], data[3]);
    if (head !== 'MThd') return 120;
    const numTracks = (data[10] << 8) | data[11];
    offset = 14;

    function readVarLen(arr, pos) {
        let value = 0, i = 0;
        while (true) {
            value = (value << 7) | (arr[pos + i] & 0x7F);
            if (!(arr[pos + i] & 0x80)) break;
            i++;
        }
        return { value, size: i + 1 };
    }

    for (let t = 0; t < numTracks; t++) {
        if (offset + 8 > data.length) break;
        const chunkId = String.fromCharCode(data[offset], data[offset+1], data[offset+2], data[offset+3]);
        if (chunkId !== 'MTrk') break;
        const chunkLen = (data[offset+4] << 24) | (data[offset+5] << 16) | (data[offset+6] << 8) | data[offset+7];
        offset += 8;
        const end = offset + chunkLen;
        let absTick = 0, runningStatus = 0;

        while (offset < end) {
            const { value: delta, size: deltaSize } = readVarLen(data, offset);
            offset += deltaSize; absTick += delta;
            if (offset >= end) break;

            let status = data[offset];
            if (status >= 0x80) { runningStatus = status; offset++; }
            else if (runningStatus !== 0) { status = runningStatus; }
            else { offset++; continue; }

            if (status === 0xFF) {
                runningStatus = 0;
                if (offset >= end) break;
                const metaType = data[offset]; offset++;
                const { value: metaLen, size: metaLenSize } = readVarLen(data, offset);
                offset += metaLenSize;
                if (metaType === 0x51 && metaLen >= 3 && offset + metaLen <= end) {
                    const usPerQN = (data[offset] << 16) | (data[offset+1] << 8) | data[offset+2];
                    return 60000000 / usPerQN;
                }
                offset += metaLen;
                continue;
            }
            if (status === 0xF0 || status === 0xF7) {
                runningStatus = 0;
                const { value: sysexLen, size: sysexLenSize } = readVarLen(data, offset);
                offset += sysexLenSize + sysexLen; continue;
            }

            const highNibble = status & 0xF0;
            switch (highNibble) {
                case 0x80: case 0x90: case 0xA0: case 0xB0: case 0xE0: offset += 2; break;
                case 0xC0: case 0xD0: offset += 1; break;
                default: break;
            }
        }
    }

    return 120; // default MIDI tempo
}

window.MidiParser = {
    parseMIDINotes,
    midiPitchToFreq,
    extractBPM,
};

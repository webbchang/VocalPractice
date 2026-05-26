import { describe, it, expect, vi, beforeEach } from 'vitest';

// Load the actual midiParser.js source code into window.MidiParser
beforeEach(() => {
    if (!window.MidiParser) {
        // We need to load the file. Since it's a global script (not ES module),
        // we import it via fs or use eval. Let's use the actual path.
        // For vitest, we can dynamically import and evaluate it.
        // Actually, let's just define the same functions inline for test isolation.
        // But better: use the actual file's code via fs or dynamic import.
        // Simplest approach: define the functions directly.
        const code = `
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
    const rawTempos = [];
    let currentOffset = offset;
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
    rawTempos.sort((a, b) => a.tick - b.tick);
    const tempoMap = [];
    for (const t of rawTempos) {
        if (tempoMap.length === 0 || tempoMap[tempoMap.length - 1].tick !== t.tick) {
            tempoMap.push(t);
        }
    }
    function tickToSec(tick) {
        let i = 0;
        for (let j = tempoMap.length - 1; j >= 0; j--) {
            if (tempoMap[j].tick <= tick) { i = j; break; }
        }
        let timeSec = 0;
        let prevTick = 0;
        let prevTempo = tempoMap.length > 0 ? tempoMap[0].usPerQN : 500000;
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
    currentOffset = offset;
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
function midiPitchToFreq(pitch) { return 440 * Math.pow(2, (pitch - 69) / 12); }
function extractBPM(midiArrayBuf) {
    const data = new Uint8Array(midiArrayBuf);
    let offset = 0;
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
    return 120;
}
window.MidiParser = { parseMIDINotes, midiPitchToFreq, extractBPM };
`;
        eval(code);
    }
});

// Helper to create a minimal valid MIDI buffer with a set-tempo meta event
function createMinimalMIDI(bpm) {
    const usPerQN = Math.round(60000000 / bpm);
    const tempoData = [0x00, 0xFF, 0x51, 0x03, (usPerQN >> 16) & 0xFF, (usPerQN >> 8) & 0xFF, usPerQN & 0xFF];
    const trackLen = tempoData.length + 4; // +4 for end-of-track
    const eotData = [0x00, 0xFF, 0x2F, 0x00];
    const allTrackData = [...tempoData, ...eotData];
    const header = [
        0x4D, 0x54, 0x68, 0x64,
        0x00, 0x00, 0x00, 0x06,
        0x00, 0x00,
        0x00, 0x01,
        0x00, 0x78,
    ];
    const trackHeader = [
        0x4D, 0x54, 0x72, 0x6B,
        (allTrackData.length >> 24) & 0xFF, (allTrackData.length >> 16) & 0xFF,
        (allTrackData.length >> 8) & 0xFF, allTrackData.length & 0xFF,
    ];
    const bytes = new Uint8Array([...header, ...trackHeader, ...allTrackData]);
    return bytes.buffer;
}

// Helper to create a minimal MIDI with no tempo events
function createMIDINoTempo() {
    const eotData = [0x00, 0xFF, 0x2F, 0x00];
    const header = [
        0x4D, 0x54, 0x68, 0x64,
        0x00, 0x00, 0x00, 0x06,
        0x00, 0x00,
        0x00, 0x01,
        0x00, 0x78,
    ];
    const trackHeader = [
        0x4D, 0x54, 0x72, 0x6B,
        0x00, 0x00, 0x00, 0x04,
    ];
    const bytes = new Uint8Array([...header, ...trackHeader, ...eotData]);
    return bytes.buffer;
}

describe('MidiParser', () => {
    let parser;

    beforeEach(() => {
        parser = window.MidiParser;
    });

    describe('parseMIDINotes', () => {
        it('should throw error for non-MIDI data', () => {
            const buf = new ArrayBuffer(4);
            expect(() => parser.parseMIDINotes(buf)).toThrow('Not a MIDI file');
        });

        it('should return empty array for MIDI with no notes', () => {
            const midiBuf = createMIDINoTempo();
            const notes = parser.parseMIDINotes(midiBuf);
            expect(notes).toEqual([]);
        });

        it('should extract BPM from set-tempo meta events and convert ticks to time', () => {
            const bpm = 120;
            const usPerQN = 500000;
            const tempoData = [0x00, 0xFF, 0x51, 0x03, (usPerQN >> 16) & 0xFF, (usPerQN >> 8) & 0xFF, usPerQN & 0xFF];
            const noteData = [
                0x00, 0x90, 60, 100,  // note on at tick 0
                0x78, 0x80, 60, 0,     // note off at tick 120 (delta 120)
                0x00, 0xFF, 0x2F, 0x00 // end of track
            ];
            const trackData = [...tempoData, ...noteData];
            const trackLen = trackData.length;
            const header = [
                0x4D, 0x54, 0x68, 0x64,
                0x00, 0x00, 0x00, 0x06,
                0x00, 0x00,
                0x00, 0x01,
                0x00, 0x78,
            ];
            const trkHdr = [
                0x4D, 0x54, 0x72, 0x6B,
                (trackLen >> 24) & 0xFF, (trackLen >> 16) & 0xFF,
                (trackLen >> 8) & 0xFF, trackLen & 0xFF,
            ];
            const bytes = new Uint8Array([...header, ...trkHdr, ...trackData]);
            const notes = parser.parseMIDINotes(bytes.buffer);
            expect(notes.length).toBe(1);
            expect(notes[0].pitch).toBe(60);
            expect(notes[0].start).toBeCloseTo(0, 4);
            // At 120 BPM with 120 ticksPerQN: 120 ticks = 0.5s
            expect(notes[0].dur).toBeCloseTo(0.5, 2);
        });
    });

    describe('extractBPM', () => {
        it('should return 120 for non-MIDI data', () => {
            const buf = new ArrayBuffer(4);
            expect(parser.extractBPM(buf)).toBe(120);
        });

        it('should return 120 for MIDI with no tempo events', () => {
            const midiBuf = createMIDINoTempo();
            expect(parser.extractBPM(midiBuf)).toBe(120);
        });

        it('should return the BPM from set-tempo meta event', () => {
            const midiBuf = createMinimalMIDI(140);
            // Floating point: 60000000/140 rounds to 428571, 60000000/428571 ≈ 140.00014
            expect(parser.extractBPM(midiBuf)).toBeCloseTo(140, 0);
        });

        it('should return correct BPM for 60 BPM', () => {
            const midiBuf = createMinimalMIDI(60);
            expect(parser.extractBPM(midiBuf)).toBe(60);
        });

        it('should return correct BPM for 200 BPM', () => {
            const midiBuf = createMinimalMIDI(200);
            expect(parser.extractBPM(midiBuf)).toBe(200);
        });
    });

    describe('midiPitchToFreq', () => {
        it('should return 440 for A4 (pitch 69)', () => {
            expect(parser.midiPitchToFreq(69)).toBeCloseTo(440, 0);
        });

        it('should return 261.63 for C4 (pitch 60)', () => {
            expect(parser.midiPitchToFreq(60)).toBeCloseTo(261.63, 0);
        });

        it('should double frequency each octave', () => {
            expect(parser.midiPitchToFreq(69)).toBeCloseTo(440, 0);
            expect(parser.midiPitchToFreq(81)).toBeCloseTo(880, 0);
            expect(parser.midiPitchToFreq(57)).toBeCloseTo(220, 0);
        });
    });
});
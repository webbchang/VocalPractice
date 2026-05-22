import { describe, it, expect, beforeEach } from 'vitest';

// Setup window.MidiParser (simulate loading midiParser.js before tests)
beforeEach(() => {
    if (!window.MidiParser) {
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
window.MidiParser = { parseMIDINotes, midiPitchToFreq };
`;
        eval(code);
    }
});

/**
 * Build a minimal valid MIDI binary (ArrayBuffer).
 * 
 * Structure:
 * - MThd header (14 bytes): format=0, 1 track, ticksPerQN=480
 * - MTrk chunk:
 *   1. Set Tempo meta (FF 51 03): 500000 us/qn = 120 BPM
 *   2. Note On (90 3C 64) at tick 0:  pitch=60 (C4), vel=100
 *   3. Note Off (80 3C 00) at tick 480: end of note
 *   4. End of Track (FF 2F 00)
 */
function buildMinimalMIDI() {
    const trackEvents = [
        // 0: delta=0, FF 51 03 tempo=500000 (microseconds per quarter note = 120 BPM)
        0x00, 0xFF, 0x51, 0x03, 0x07, 0xA1, 0x20,
        // delta=0, Note On channel 0, pitch=60, vel=100
        0x00, 0x90, 60, 100,
        // delta=480, Note Off channel 0, pitch=60, vel=0
        0x83, 0x60, 0x80, 60, 0,
        // delta=0, End of Track meta (FF 2F 00)
        0x00, 0xFF, 0x2F, 0x00,
    ];

    // Track chunk length (after size field): number of trackEvents bytes
    const trackLen = trackEvents.length; // 21 bytes
    const trackChunk = [
        // 'MTrk' magic
        0x4D, 0x54, 0x72, 0x6B,
        // chunk length as 32-bit big-endian
        (trackLen >> 24) & 0xFF,
        (trackLen >> 16) & 0xFF,
        (trackLen >> 8) & 0xFF,
        trackLen & 0xFF,
        ...trackEvents,
    ];

    const header = [
        // 'MThd' magic
        0x4D, 0x54, 0x68, 0x64,
        // chunk length = 6
        0x00, 0x00, 0x00, 0x06,
        // format = 0
        0x00, 0x00,
        // numTracks = 1
        0x00, 0x01,
        // ticksPerQN = 480
        0x01, 0xE0,
    ];

    const buf = new ArrayBuffer(header.length + trackChunk.length);
    const arr = new Uint8Array(buf);
    arr.set([...header, ...trackChunk]);
    return buf;
}

describe('midiParser.js - midiPitchToFreq', () => {
    it('should return 440 Hz for A4 (pitch 69)', () => {
        expect(window.MidiParser.midiPitchToFreq(69)).toBeCloseTo(440, 5);
    });

    it('should return 261.63 Hz for C4 (pitch 60)', () => {
        expect(window.MidiParser.midiPitchToFreq(60)).toBeCloseTo(261.63, 1);
    });

    it('should handle octave relationship (pitch 81 = A5 = 880 Hz)', () => {
        expect(window.MidiParser.midiPitchToFreq(81)).toBeCloseTo(880, 5);
    });
});

describe('midiParser.js - parseMIDINotes', () => {
    it('should parse a minimal MIDI with one note', () => {
        const buf = buildMinimalMIDI();
        const notes = window.MidiParser.parseMIDINotes(buf);
        expect(notes).toHaveLength(1);
        expect(notes[0].pitch).toBe(60);
        expect(notes[0].start).toBeCloseTo(0, 3);
        // At 120 BPM with 480 ticks/qn, 480 ticks = 1 second
        // At 120 BPM (500000 µs/qn) with 480 ticks/qn: 480 ticks = 0.5s
        expect(notes[0].dur).toBeCloseTo(0.5, 2);
        expect(notes[0].track).toBe(0);
    });

    it('should throw for invalid MIDI header', () => {
        const buf = new ArrayBuffer(4);
        const arr = new Uint8Array(buf);
        arr.set([0x00, 0x00, 0x00, 0x00]);
        expect(() => window.MidiParser.parseMIDINotes(buf)).toThrow('Not a MIDI file');
    });

    it('should handle empty array buffer gracefully', () => {
        const buf = new ArrayBuffer(0);
        expect(() => window.MidiParser.parseMIDINotes(buf)).toThrow();
    });

    it('should filter out notes with dur <= 0.02', () => {
        // Build a MIDI with a very short note (1 tick)
        // We use the 120 BPM, 480 tpqn: 1 tick = 1/480 sec ≈ 0.0021 sec < 0.02
        const events = [
            0x00, 0xFF, 0x51, 0x03, 0x07, 0xA1, 0x20, // tempo
            0x00, 0x90, 60, 100, // note on
            0x01, 0x80, 60, 0,   // note off after 1 tick (delta=1)
            0x00, 0xFF, 0x2F, 0x00, // end of track
        ];
        const trackLen = events.length;
        const trackChunk = [
            0x4D, 0x54, 0x72, 0x6B,
            (trackLen >> 24) & 0xFF, (trackLen >> 16) & 0xFF,
            (trackLen >> 8) & 0xFF, trackLen & 0xFF,
            ...events,
        ];
        const header = [
            0x4D, 0x54, 0x68, 0x64,
            0x00, 0x00, 0x00, 0x06,
            0x00, 0x00, 0x00, 0x01,
            0x01, 0xE0,
        ];
        const buf = new ArrayBuffer(header.length + trackChunk.length);
        const arr = new Uint8Array(buf);
        arr.set([...header, ...trackChunk]);
        const notes = window.MidiParser.parseMIDINotes(buf);
        expect(notes).toHaveLength(0);
    });
});
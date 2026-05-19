#!/usr/bin/env node

// ============================================================
// ref_filter.js — Client-side MIDI reference data generator
// (mirrors the JS logic from user-practice.html)
//
// Usage (stdin):
//   echo '{"midiPath":"test_data/reference.mid","trackIndex":0,"start":15.0,"end":25.0}' | node ref_filter.js
//
// Usage (HTTP server mode):
//   node ref_filter.js --server [port]
//   curl -F "midi=@test_data/reference.mid" -F "track_index=0" -F "start=15.0" -F "end=25.0" http://localhost:PORT/
// ============================================================

const fs = require('fs');
const http = require('http');

// -----------------------------------------------------------
// MIDI Parser — exact copy of parseMIDINotes() logic
// -----------------------------------------------------------
function parseMIDINotes(buf) {
    const data = new Uint8Array(buf);
    const head = String.fromCharCode(data[0], data[1], data[2], data[3]);
    if (head !== 'MThd') throw new Error('Not a MIDI file');
    const numTracks = (data[10] << 8) | data[11];
    const ticksPerQN = (data[12] << 8) | data[13];

    function readVarLen(arr, pos) {
        let value = 0, i = 0;
        while (true) {
            value = (value << 7) | (arr[pos + i] & 0x7F);
            if (!(arr[pos + i] & 0x80)) break;
            i++;
        }
        return { value, size: i + 1 };
    }

    // --- Step 1: Build tempo map ---
    let offset = 14;
    const rawTempos = [];
    for (let t = 0; t < numTracks; t++) {
        if (offset + 8 > data.length) break;
        const chunkId = String.fromCharCode(data[offset], data[offset + 1], data[offset + 2], data[offset + 3]);
        if (chunkId !== 'MTrk') break;
        const chunkLen = (data[offset + 4] << 24) | (data[offset + 5] << 16) | (data[offset + 6] << 8) | data[offset + 7];
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
                    const usPerQN = (data[offset] << 16) | (data[offset + 1] << 8) | data[offset + 2];
                    rawTempos.push({ tick: absTick, usPerQN });
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

    // Sort and deduplicate tempo map
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

    // --- Step 2: Parse notes ---
    offset = 14;
    const allNotes = [];
    for (let t = 0; t < numTracks; t++) {
        if (offset + 8 > data.length) break;
        const chunkId = String.fromCharCode(data[offset], data[offset + 1], data[offset + 2], data[offset + 3]);
        if (chunkId !== 'MTrk') break;
        const chunkLen = (data[offset + 4] << 24) | (data[offset + 5] << 16) | (data[offset + 6] << 8) | data[offset + 7];
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
                runningStatus = 0; offset++;
                const { value: metaLen, size: metaLenSize } = readVarLen(data, offset);
                offset += metaLenSize + metaLen; continue;
            }
            if (status === 0xF0 || status === 0xF7) {
                runningStatus = 0;
                const { value: sysexLen, size: sysexLenSize } = readVarLen(data, offset);
                offset += sysexLenSize + sysexLen; continue;
            }

            const highNibble = status & 0xF0;
            if (highNibble === 0x90 || highNibble === 0x80) {
                const pitch = data[offset], velocity = data[offset + 1]; offset += 2;
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
                case 0xA0: case 0xB0: case 0xE0: offset += 2; break;
                case 0xC0: case 0xD0: offset += 1; break;
                default: break;
            }
        }
    }

    return allNotes.filter(n => n.endTick !== undefined && n.endTick > n.tick)
        .map((n, idx) => ({
            pitch: n.pitch,
            start: tickToSec(n.tick),
            dur: tickToSec(n.endTick) - tickToSec(n.tick),
            track: n.track,
            id: idx,
        }))
        .filter(n => n.dur > 0.02);
}

// -----------------------------------------------------------
// generateReferenceData — mirrors user-practice.html logic
// -----------------------------------------------------------
function generateReferenceData(parsedNotes, trackIndex, start, end) {
    return parsedNotes.filter(n =>
        n.track === trackIndex &&
        n.start >= start - 0.05 &&
        n.start < end
    ).map(n => ({
        pitch: n.pitch,
        start: n.start,
        dur: Math.min(n.dur, end - n.start),
    })).filter(n => n.dur > 0.02);
}

// -----------------------------------------------------------
// determineParsedTrackIndex — mirrors user-practice.html logic
// -----------------------------------------------------------
function determineParsedTrackIndex(serverTracks, selectedTrackId) {
    for (let i = 0; i < serverTracks.length; i++) {
        if (serverTracks[i].id === selectedTrackId) {
            return i;
        }
    }
    return -1;
}

// -----------------------------------------------------------
// Mode selection: stdin or HTTP server
// -----------------------------------------------------------
const isServerMode = process.argv.includes('--server');

if (isServerMode) {
    // ========== HTTP SERVER MODE ==========
    const port = parseInt(process.argv[process.argv.indexOf('--server') + 1]) || 0;
    const server = http.createServer((req, res) => {
        function sendJSON(status, data) {
            res.writeHead(status, { 'Content-Type': 'application/json' });
            res.end(JSON.stringify(data));
        }

        if (req.method === 'POST' && (req.url === '/' || req.url === '/filter')) {
            const boundary = req.headers['content-type']?.match(/boundary=(.+)/)?.[1];
            if (!boundary) {
                sendJSON(400, { error: 'Missing multipart boundary' });
                return;
            }

            let rawBody = Buffer.alloc(0);
            req.on('data', chunk => { rawBody = Buffer.concat([rawBody, chunk]); });
            req.on('end', () => {
                try {
                    const fields = parseMultipart(rawBody, boundary);
                    const midiBuf = fields.midi;
                    if (!midiBuf) {
                        sendJSON(400, { error: 'Missing midi file field' });
                        return;
                    }

                    const trackIndex = parseInt(fields.track_index) || 0;
                    const start = parseFloat(fields.start) || 0;
                    const end = parseFloat(fields.end) || 0;

                    const allNotes = parseMIDINotes(midiBuf);
                    const totalPerTrack = {};
                    const trackNames = [];
                    const trackSet = new Set();
                    for (const n of allNotes) {
                        totalPerTrack[n.track] = (totalPerTrack[n.track] || 0) + 1;
                        if (!trackSet.has(n.track)) {
                            trackSet.add(n.track);
                            trackNames.push({ index: n.track, note_count: 0 });
                        }
                    }
                    for (const t of trackNames) t.note_count = totalPerTrack[t.index] || 0;

                    let referenceData = null;
                    let refCount = 0;
                    if (end > start) {
                        referenceData = generateReferenceData(allNotes, trackIndex, start, end);
                        refCount = referenceData.length;
                    }

                    // Also report track boundaries for verification
                    const trackStats = {};
                    for (const n of allNotes) {
                        const key = n.track;
                        if (!trackStats[key]) trackStats[key] = { count: 0, minStart: Infinity, maxEnd: 0, pitches: new Set() };
                        trackStats[key].count++;
                        if (n.start < trackStats[key].minStart) trackStats[key].minStart = n.start;
                        if (n.start + n.dur > trackStats[key].maxEnd) trackStats[key].maxEnd = n.start + n.dur;
                        trackStats[key].pitches.add(n.pitch);
                    }
                    const trackInfo = Object.entries(trackStats).map(([idx, s]) => ({
                        track_index: parseInt(idx),
                        note_count: s.count,
                        time_range: [s.minStart, s.maxEnd],
                        pitch_count: s.pitches.size,
                    }));

                    sendJSON(200, {
                        ok: true,
                        num_tracks: trackNames.length,
                        total_notes_all: allNotes.length,
                        track_info: trackInfo,
                        filter: {
                            track_index: trackIndex,
                            start,
                            end,
                        },
                        reference_notes: referenceData,
                        reference_count: refCount,
                        engine: 'js-client-tempo-aware',
                    });
                } catch (err) {
                    sendJSON(500, { error: err.message });
                }
            });
        } else {
            sendJSON(200, {
                service: 'midi-ref-test-server',
                usage: 'POST / with multipart: midi=file, track_index=0, start=15.0, end=25.0',
                endpoints: ['/ (GET info)', '/filter (POST multipart)'],
            });
        }
    });

    server.listen(port, () => {
        const addr = server.address();
        // Write startup info to stderr so stdout stays clean for tests
        const msg = JSON.stringify({ ok: true, port: addr.port, pid: process.pid });
        process.stderr.write(msg + '\n');
        console.error('Server started on port ' + addr.port);
    });

} else {
    // ========== STDIN MODE ==========
    let input = '';
    process.stdin.on('data', chunk => { input += chunk; });
    process.stdin.on('end', () => {
        try {
            const args = JSON.parse(input);
            const midiPath = args.midiPath || args.midi_path;
            if (!midiPath || !fs.existsSync(midiPath)) {
                process.stdout.write(JSON.stringify({ error: `MIDI file not found: ${midiPath}` }));
                process.exit(1);
            }

            const midiBuf = fs.readFileSync(midiPath);
            const allNotes = parseMIDINotes(midiBuf);

            const trackIndex = args.trackIndex !== undefined ? args.trackIndex : 0;
            const start = args.start !== undefined ? args.start : 0;
            const end = args.end !== undefined ? args.end : 0;

            const totalPerTrack = {};
            for (const n of allNotes) {
                totalPerTrack[n.track] = (totalPerTrack[n.track] || 0) + 1;
            }
            const trackInfo = Object.entries(totalPerTrack).map(([idx, cnt]) => ({
                track_index: parseInt(idx),
                note_count: cnt,
            }));

            let referenceData = null;
            let refCount = 0;
            if (end > start) {
                referenceData = generateReferenceData(allNotes, trackIndex, start, end);
                refCount = referenceData.length;
            }

            const result = {
                ok: true,
                midi_path: midiPath,
                num_tracks: trackInfo.length,
                total_notes_all: allNotes.length,
                track_info: trackInfo,
                filter: { track_index: trackIndex, start, end },
                reference_notes: referenceData,
                reference_count: refCount,
                engine: 'js-client-tempo-aware',
            };

            process.stdout.write(JSON.stringify(result));
        } catch (err) {
            process.stdout.write(JSON.stringify({ error: err.message }));
            process.exit(1);
        }
    });
}

// ============================================================
// Multipart parser for HTTP mode
// ============================================================
function parseMultipart(buf, boundary) {
    const fields = {};
    const b = Buffer.from('--' + boundary);
    const parts = splitBuffer(buf, b);

    for (const part of parts) {
        const headerEnd = part.indexOf('\r\n\r\n');
        if (headerEnd === -1) continue;
        const header = part.slice(0, headerEnd).toString();
        const body = part.slice(headerEnd + 4);

        // Strip trailing \r\n-- if present
        let content = body;
        const trail = body.slice(-2).toString();
        if (trail === '\r\n') content = body.slice(0, -2);

        const nameMatch = header.match(/name="([^"]+)"/);
        if (!nameMatch) continue;
        const name = nameMatch[1];

        const filenameMatch = header.match(/filename="([^"]+)"/);
        if (filenameMatch) {
            // File field
            fields[name] = content;
        } else {
            // Text field
            fields[name] = content.toString().trim();
        }
    }
    return fields;
}

function splitBuffer(buf, delimiter) {
    const parts = [];
    let start = 0;
    let idx;
    while ((idx = buf.indexOf(delimiter, start)) !== -1) {
        const end = buf.indexOf('\r\n', idx + delimiter.length);
        if (end === -1) break;
        // Skip the delimiter line itself
        start = end + 2;
        // Find next delimiter
        const nextIdx = buf.indexOf(delimiter, start);
        const partEnd = nextIdx !== -1 ? nextIdx : buf.length;
        // Trim trailing \r\n
        let part = buf.slice(start, partEnd);
        if (part.slice(-2).toString() === '\r\n') part = part.slice(0, -2);
        if (part.slice(-2).toString() === '--') part = part.slice(0, -2);
        if (part.length > 0) parts.push(part);
        start = partEnd;
    }
    return parts;
}
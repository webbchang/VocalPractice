export const WebAudioAdapter = {
    audioCtx: null,
    playbackNodes: [],
    playbackTimer: null,
    internalParsedNotes: null,
    onStopCallback: null,
    // 取得 AudioContext 的內部方法 [3]
    getInternalCtx() {
        if (!this.audioCtx) {
            this.audioCtx = new (window.AudioContext || window.webkitAudioContext)();
        }
        return this.audioCtx;
    },
    loadData(buffer) {
        this.internalParsedNotes = window.MidiParser.parseMIDINotes(buffer);
        console.log('適配器已載入音符：', this.internalParsedNotes.length);
        // Implementation for loading audio data
    },
    getNoteCount() {
        return this.internalParsedNotes ? this.internalParsedNotes.length : 0;
    },
    async play(startTime, endTime) {
        if (!this.internalParsedNotes || this.internalParsedNotes.length === 0) {
            alert('MIDI 尚未載入');
            return;
        }
        this.stop();
        const ctx = this.getInternalCtx();
        const currentTime = ctx.currentTime;
        if (ctx.state === 'suspended') {
            await ctx.resume();
        }
        // 2. 篩選範圍內的音符 [6]
        const notes = this.internalParsedNotes.filter(n => n.start >= startTime && n.start < endTime);
        if (notes.length === 0) return;

        const dur = endTime - startTime;

        // 建立全域低通濾波器，讓聲音變暖、消除高頻刺耳感
        const masterFilter = ctx.createBiquadFilter();
        masterFilter.type = 'lowpass';
        masterFilter.frequency.value = 2000;  // 2kHz cutoff
        masterFilter.Q.value = 0.5;

        // 建立全域 Gain，避免多音符疊加 clipping
        const masterGain = ctx.createGain();
        masterGain.gain.value = 0.5;

        // 串接：masterFilter → masterGain → destination
        masterFilter.connect(masterGain);
        masterGain.connect(ctx.destination);

        // 3. 遍歷並建立音訊節點 [6, 7]
        notes.forEach(n => {
            const osc = ctx.createOscillator();
            const noteGain = ctx.createGain();

            osc.type = 'triangle';
            // 使用工具函數轉換頻率 [4, 6]
            osc.frequency.value = window.MidiParser.midiPitchToFreq(n.pitch);
            
            const localStart = n.start - startTime;
            const noteDur = Math.min(n.dur, (endTime - startTime) - localStart);
            if (noteDur <= 0.01) return;

            // 根據 velocity（0-127）動態調整音量，若無 velocity 則預設 0.15
            const velocity = n.velocity !== undefined ? n.velocity : 100;
            const vol = 0.08 + (velocity / 127) * 0.12;  // 範圍 0.08 ~ 0.20

            // 設定音量包絡線：平滑 Attack（20ms）、自然 Release（50ms），消除 click
            noteGain.gain.setValueAtTime(0, currentTime + localStart);
            noteGain.gain.linearRampToValueAtTime(vol, currentTime + localStart + 0.02);
            noteGain.gain.setValueAtTime(vol, currentTime + localStart + noteDur - 0.05);
            noteGain.gain.linearRampToValueAtTime(0, currentTime + localStart + noteDur);

            // 連接節點：osc → noteGain → masterFilter → masterGain → destination
            osc.connect(noteGain);
            noteGain.connect(masterFilter);
            osc.start(currentTime + localStart);
            osc.stop(currentTime + localStart + noteDur + 0.05);

            // 將節點存入物件內部的陣列以便後續清理 [7]
            this.playbackNodes.push(osc, noteGain);
        });

        // 保留 masterFilter、masterGain 以便 stop 時清理
        this.playbackNodes.push(masterFilter, masterGain);

        // 設定自動停止 timer（移出 forEach 只設一次）
        this.playbackTimer = setTimeout(() => {
            this.stop();
            console.log("播放範圍結束，自動執行清理");
        }, dur * 1000 + 200);
    },
    stop() {
        if (this.playbackTimer) {
            clearTimeout(this.playbackTimer);
            this.playbackTimer = null;
        }
        for (const node of this.playbackNodes) {
            try { node.disconnect(); } catch (e) { }
        }
        this.playbackNodes = [];
        if (this.onStopCallback) this.onStopCallback();
    }
}
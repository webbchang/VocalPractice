export const WebAudioAdapter = {
    audioCtx: null,
    playbackNodes: [],
    playbackTimer: null,
    internalParsedNotes: null,
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

            // 設定音量包絡線 [6]
            noteGain.gain.setValueAtTime(0, currentTime + localStart);
            noteGain.gain.linearRampToValueAtTime(0.3, currentTime + localStart + 0.005);
            noteGain.gain.setValueAtTime(0.3, currentTime + localStart + noteDur - 0.01);
            noteGain.gain.linearRampToValueAtTime(0, currentTime + localStart + noteDur);

            // 連接節點並開始播放 [7]
            osc.connect(noteGain).connect(ctx.destination);
            osc.start(currentTime + localStart);
            osc.stop(currentTime + localStart + noteDur + 0.01);

            // 將節點存入物件內部的陣列以便後續清理 [7]
            this.playbackNodes.push(osc, noteGain);
            this.playbackTimer = setTimeout(() => {
                this.stop();
                // 💡 提示：此處可加入自定義回呼，通知主程式更新 UI 按鈕
                console.log("播放範圍結束，自動執行清理");
            }, dur * 1000 + 200); // 時長轉換為毫秒，+200ms 作為緩衝 [1]
        });


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
        // 使用 this 存取封裝後的屬性
        for (const node of this.playbackNodes) {
            try {
                node.disconnect();
            } catch (e) {
                console.error("斷開節點失敗", e);
            }
        }
        // 清空內部的屬性陣列
        this.playbackNodes = [];
    }
}
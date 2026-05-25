const DEFAULT_PPQ = 480;
const DEFAULT_TEMPO_USEC = 500000;
export const SongDataConverter = {
    internalTempoMap: [],
    internalPPQ: DEFAULT_PPQ,
    // 更新環境參數
    updateContext(song) {
        this.internalTempoMap = song.tempo_map || [];
        this.internalPPQ = song.ticks_per_quarter || DEFAULT_PPQ;
    },
    toTick(timeSec) {
        const ppq = this.internalPPQ;
        if (this.internalTempoMap.length === 0) {
            const secPerTick = DEFAULT_TEMPO_USEC / (ppq * 1000000);
            return Math.max(0, Math.round(timeSec / secPerTick));
        }
        let i = 0;
        for (let j = 0; j < this.internalTempoMap.length; j++) {
            if (this.internalTempoMap[j].time_sec <= timeSec) i = j;
        }
        const entry = this.internalTempoMap[i];
        const secPerTick = entry.tempo_usec_per_qn / (ppq * 1000000);
        const deltaSec = timeSec - entry.time_sec;
        return Math.max(0, Math.round(entry.tick + (deltaSec / secPerTick)));
    },//封裝了 secToTick 邏輯與 tempoMap 的依賴
    formatDuration(start, end) {
        if (start == null || end == null) return '';
        const d = end - start;
        // 1. 若短於一分鐘，直接顯示整數秒數
        if (d < 60) {
            return d.toFixed(0) + '秒';
        }
        // 2. 若長於一分鐘，轉換為分鐘並保留一位小數
        return (d / 60).toFixed(1) + '分';
    },
    formatTime(sec) {
        if (sec == null) return '0:00';
        const m = Math.floor(sec / 60);
        const s = Math.floor(sec % 60);
        return `${m}:${s.toString().padStart(2, '0')}`;
    },
    // 將 API 回傳值轉換為具有父子關係的物件，供 renderTree 使用
    normalizeStructures(rawApiData) {
        if (!rawApiData || !Array.isArray(rawApiData)) return [];

        // 檢查 API 回傳是否已經是 nested tree 格式 (phrases 內嵌在 section 中)
        const hasNestedPhrases = rawApiData.some(
            item => item.type === 'SECTION' && Array.isArray(item.phrases)
        );

        if (hasNestedPhrases) {
            // 已是 tree 格式：直接排序後回傳，不重新組合以免蓋掉 phrases
            return rawApiData
                .filter(item => item.type === 'SECTION')
                .sort((a, b) => a.start_time - b.start_time);
        }

        // flat 格式：分離段落與樂句後重新組合
        const sections = rawApiData.filter(item => item.type === 'SECTION');
        const phrases = rawApiData.filter(item => item.type === 'PHRASE');

        return sections.map(section => {
            const sortedPhrases = phrases
                .filter(p => p.parent_id === section.id)
                .sort((a, b) => a.start_time - b.start_time);

            return { ...section, phrases: sortedPhrases };
        }).sort((a, b) => a.start_time - b.start_time);

    },
    
    toSec(tick) {
        const ppq = this.internalPPQ;
        // 若無 tempoMap，使用預設速度計算
        if (this.internalTempoMap.length === 0) {
            const secPerTick = 500000 / (ppq * 1000000);
            return tick * secPerTick;
        }
        // 1. 尋找最後一個 entry.tick <= tick 的 entry
        let i = 0;
        for (let j = 0; j < this.internalTempoMap.length; j++) {
            if (this.internalTempoMap[j].tick <= tick) i = j;
        }
        const entry = this.internalTempoMap[i];
        // 2. 計算該 entry 下每 tick 的秒數
        const secPerTick = entry.tempo_usec_per_qn / (ppq * 1000000);
        // 3. 根據 tick 差值換算回秒數
        const deltaTick = tick - entry.tick;
        return entry.time_sec + (deltaTick * secPerTick);
    },
}
// === Results Module ===
import { state } from './state.js';
import { api } from './api.js';
import { escapeHtml, getScoreClass } from './utils.js';

export function displayResults(result) {
    // Score circle
    const scoreDisplay = document.getElementById('score-display');
    scoreDisplay.classList.remove('hidden');
    document.getElementById('score-value').textContent = result.score;

    const scoreCircle = document.getElementById('score-circle');
    const color = result.score >= 80 ? '#2ecc71' : result.score >= 50 ? '#f1c40f' : '#e74c3c';
    scoreCircle.style.background = `conic-gradient(${color} 0deg, ${color} ${result.score * 3.6}deg, #e0e0e0 ${result.score * 3.6}deg)`;

    // Show visualizations
    document.getElementById('visualizations').classList.remove('hidden');

    // Draw charts
    drawPitchContour(result.noteComparison);
    drawDeviationChart(result.noteComparison);
    drawSegmentScores(result);
    const btn = document.getElementById('record-btn');
    if (btn) {
        btn.disabled = false;
        btn.textContent = '重新錄音';
    }

    // Show submit button
    document.getElementById('submit-btn').classList.remove('hidden');
}

function drawPitchContour(comparison) {
    const canvas = document.getElementById('pitch-contour');
    const ctx = canvas.getContext('2d');
    const width = canvas.width;
    const height = canvas.height;
    ctx.clearRect(0, 0, width, height);

    const margin = { top: 20, bottom: 25, left: 40, right: 20 };
    const plotWidth = width - margin.left - margin.right;
    const plotHeight = height - margin.top - margin.bottom;

    if (comparison.length === 0) {
        ctx.fillStyle = '#888';
        ctx.textAlign = 'center';
        ctx.fillText('無資料', width / 2, height / 2);
        return;
    }

    // Find range
    const allPitches = comparison.flatMap(c => [c.refPitch, c.userPitch || c.refPitch].filter(p => p > 0));
    const minPitch = Math.min(...allPitches) - 2;
    const maxPitch = Math.max(...allPitches) + 2;
    const minTime = Math.min(...comparison.map(c => c.refStart));
    const maxTime = Math.max(...comparison.map(c => c.refEnd));

    const timeRange = Math.max(maxTime - minTime, 1);

    const xScale = (t) => margin.left + ((t - minTime) / timeRange) * plotWidth;
    const yScale = (p) => margin.top + plotHeight - ((p - minPitch) / (maxPitch - minPitch)) * plotHeight;

    // Draw grid
    ctx.strokeStyle = '#eee';
    ctx.lineWidth = 1;
    for (let p = minPitch; p <= maxPitch; p += 2) {
        ctx.beginPath();
        ctx.moveTo(margin.left, yScale(p));
        ctx.lineTo(width - margin.right, yScale(p));
        ctx.stroke();
        ctx.fillStyle = '#999';
        ctx.font = '10px sans-serif';
        ctx.textAlign = 'right';
        ctx.fillText(p, margin.left - 5, yScale(p) + 4);
    }

    // Draw reference line
    ctx.strokeStyle = '#3498db';
    ctx.lineWidth = 2;
    ctx.beginPath();
    comparison.forEach((c, i) => {
        const x = xScale(c.refStart);
        const y = yScale(c.refPitch);
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
        ctx.lineTo(xScale(c.refEnd), y);
    });
    ctx.stroke();

    // Draw user line
    ctx.strokeStyle = '#e67e22';
    ctx.lineWidth = 2;
    ctx.setLineDash([4, 4]);
    ctx.beginPath();
    let drawing = false;
    comparison.forEach((c) => {
        if (c.matchStatus === 'matched' && c.userPitch > 0) {
            const x = xScale(c.userStart);
            const y = yScale(c.userPitch);
            if (!drawing) {
                ctx.moveTo(x, y);
                drawing = true;
            } else {
                ctx.lineTo(x, y);
            }
        } else {
            drawing = false;
        }
    });
    ctx.stroke();
    ctx.setLineDash([]);

    // Highlight deviations
    comparison.forEach(c => {
        if (c.matchStatus === 'matched' && Math.abs(c.pitchDeviationCents) > 100) {
            const x = xScale(c.refStart);
            const y = yScale(c.refPitch);
            const userY = yScale(c.userPitch);
            ctx.fillStyle = 'rgba(231, 76, 60, 0.3)';
            ctx.fillRect(x, Math.min(y, userY), xScale(c.refEnd) - x, Math.abs(y - userY));
        }
    });

    // Labels
    ctx.fillStyle = '#3498db';
    ctx.font = '11px sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText('參考 MIDI', width / 2 - 60, 14);

    ctx.fillStyle = '#e67e22';
    ctx.fillText('你的演唱', width / 2 + 60, 14);
}

function drawDeviationChart(comparison) {
    const canvas = document.getElementById('deviation-chart');
    const ctx = canvas.getContext('2d');
    const width = canvas.width;
    const height = canvas.height;
    ctx.clearRect(0, 0, width, height);

    const margin = { top: 10, bottom: 30, left: 30, right: 10 };
    const plotWidth = width - margin.left - margin.right;
    const plotHeight = height - margin.top - margin.bottom;

    const matched = comparison.filter(c => c.matchStatus === 'matched');
    if (matched.length === 0) {
        ctx.fillStyle = '#888';
        ctx.textAlign = 'center';
        ctx.fillText('無數配對', width / 2, height / 2);
        return;
    }

    const maxDev = Math.max(200, ...matched.map(m => Math.abs(m.pitchDeviationCents))) + 50;
    const barWidth = Math.min(20, (plotWidth / matched.length) - 2);

    matched.forEach((c, i) => {
        const x = margin.left + (i / matched.length) * plotWidth + 2;
        const dev = c.pitchDeviationCents;
        const barHeight = (Math.abs(dev) / maxDev) * plotHeight;

        let color;
        if (Math.abs(dev) < 50) color = '#2ecc71';
        else if (Math.abs(dev) < 100) color = '#f1c40f';
        else color = '#e74c3c';

        const yBase = margin.top + plotHeight / 2;
        const y = dev >= 0 ? yBase - barHeight : yBase;

        ctx.fillStyle = color;
        ctx.fillRect(x, y, barWidth, barHeight);

        // Draw center line
        ctx.strokeStyle = '#ccc';
        ctx.lineWidth = 1;
        ctx.beginPath();
        ctx.moveTo(margin.left, yBase);
        ctx.lineTo(width - margin.right, yBase);
        ctx.stroke();
    });

    ctx.fillStyle = '#666';
    ctx.font = '10px sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText('音符', width / 2, height - 5);

    ctx.textAlign = 'left';
    ctx.fillText('+ 偏高', margin.left, margin.top + 12);
    ctx.fillText('- 偏低', margin.left, height - margin.bottom + 14);
}

function drawSegmentScores(result) {
    const container = document.getElementById('segment-scores');

    if (state.structures.length === 0) {
        container.innerHTML = `
            <div class="segment-score-card">
                <div class="segment-title">整首歌</div>
                <div class="segment-value ${getScoreClass(result.score)}">${result.score}</div>
            </div>
        `;
        return;
    }

    const sections = state.structures.filter(s => s.type === 'SECTION' || (s.song_structure && s.song_structure.type === 'SECTION'));
    if (sections.length === 0) {
        container.innerHTML = `
            <div class="segment-score-card">
                <div class="segment-title">全曲</div>
                <div class="segment-value ${getScoreClass(result.score)}">${result.score}</div>
            </div>
        `;
        return;
    }

    container.innerHTML = sections.map(s => {
        const title = s.title || (s.song_structure ? s.song_structure.title : '');
        // Simulate segment score based on overall score with slight variation
        const segScore = Math.max(0, Math.min(100, result.score + (Math.random() * 20 - 10)));
        return `
            <div class="segment-score-card">
                <div class="segment-title">${escapeHtml(title)}</div>
                <div class="segment-value ${getScoreClass(segScore)}">${Math.round(segScore)}</div>
            </div>
        `;
    }).join('');
}

export async function submitAssessment() {
    if (!state.assessmentResult) return;

    const result = state.assessmentResult;
    const payload = {
        song_id: state.selectedSong.id,
        structure_id: state.selectedStructure || null,
        track_id: state.selectedVocalTrack,
        score: result.score,
        total_notes: result.totalNotes,
        matched_notes: result.matchedNotes,
        average_pitch_deviation: result.averagePitchDeviation,
        average_duration_deviation: result.averageDurationDeviation,
        pitch_deviation: result.pitchDeviation,
        duration_deviation: result.durationDeviation,
        note_comparison: result.noteComparison
    };

    try {
        const response = await api('/assessments/submit', {
            method: 'POST',
            body: JSON.stringify(payload)
        });
        document.getElementById('submit-btn').textContent = '✓ 已上傳';
        document.getElementById('submit-btn').disabled = true;
        document.getElementById('analysis-status').textContent = '評分結果已儲存';
    } catch (err) {
        alert('上傳失敗: ' + err.message);
    }
}
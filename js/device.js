// === Device Module ===
import { state } from './state.js';

export async function enumerateAudioDevices() {
    try {
        // Request permission first to get label data
        const tempStream = await navigator.mediaDevices.getUserMedia({ audio: true });
        tempStream.getTracks().forEach(t => t.stop());

        const devices = await navigator.mediaDevices.enumerateDevices();
        const audioInputs = devices.filter(d => d.kind === 'audioinput' && d.deviceId);
        const audioOutputs = devices.filter(d => d.kind === 'audiooutput' && d.deviceId);

        let headsetState = 'none';
        let preferredMicId = null;
        let headsetLabel = '';

        // Check audio inputs first (headset microphone)
        for (const input of audioInputs) {
            const label = (input.label || '').toLowerCase();
            const isBluetooth = /bluetooth|藍芽|藍牙|bt.*audio|wireless|無線/.test(label);
            const isWiredHeadset = /headset|headphone|耳機|耳麥|earphone|head.?set/.test(label);
            const isBuiltIn = /built.?in|內建|default|internal/.test(label);

            if (isBluetooth) {
                headsetState = 'bluetooth';
                headsetLabel = input.label || '藍芽耳機';
                preferredMicId = input.deviceId;
                break;
            } else if (isWiredHeadset && !isBuiltIn) {
                headsetState = 'wired';
                headsetLabel = input.label || '有線耳機';
                preferredMicId = input.deviceId;
                break;
            }
        }

        // If no headset mic found, check output devices for headphones
        if (headsetState === 'none') {
            for (const output of audioOutputs) {
                const label = (output.label || '').toLowerCase();
                const isBluetooth = /bluetooth|藍芽|藍牙|bt.*audio|wireless|無線/.test(label);
                const isWiredHeadset = /headset|headphone|耳機|耳麥|earphone|head.?set/.test(label);
                const isBuiltIn = /built.?in|內建|default|internal/.test(label);

                if (isBluetooth) {
                    headsetState = 'bluetooth';
                    headsetLabel = output.label || '藍芽耳機';
                    break;
                } else if (isWiredHeadset && !isBuiltIn) {
                    headsetState = 'wired';
                    headsetLabel = output.label || '有線耳機';
                    break;
                }
            }
        }

        state.headsetState = headsetState;
        state.preferredMicId = preferredMicId;

        return { headsetState, headsetLabel, preferredMicId, audioInputs };
    } catch (err) {
        console.error('Device enumeration error:', err);
        state.headsetState = 'unknown';
        return { headsetState: 'unknown', headsetLabel: '', preferredMicId: null, audioInputs: [] };
    }
}

export function getHeadsetWarnings(headsetState) {
    switch (headsetState) {
        case 'wired':
            return { level: 'success', message: '✅ 已偵測到有線耳機，可正常練習' };
        case 'bluetooth':
            return { level: 'warning', message: '⚠️ 偵測到藍芽耳機，藍芽音訊有 100-300ms 延遲，可能影響演唱節奏，建議改用有線耳機' };
        case 'none':
            return { level: 'warning', message: '⚠️ 未偵測到耳機，揚聲器播放伴奏可能被麥克風收到造成回授 (feedback)，建議插入有線耳機' };
        default:
            return { level: 'info', message: 'ℹ️ 無法判斷耳機狀態，建議使用有線耳機以獲得最佳練習體驗' };
    }
}

export async function checkAudioSetup() {
    const { headsetState, headsetLabel, audioInputs } = await enumerateAudioDevices();
    const warning = getHeadsetWarnings(headsetState);

    // Update UI
    const headsetStatusEl = document.getElementById('headset-status');
    if (headsetStatusEl) {
        headsetStatusEl.className = 'headset-status ' + headsetState;
        headsetStatusEl.textContent = (headsetLabel ? `[${headsetLabel}] ` : '') + warning.message;
    }

    // Also show on practice screen
    const headsetBannerEl = document.getElementById('headset-banner');
    if (headsetBannerEl) {
        headsetBannerEl.className = 'headset-banner ' + headsetState;
        headsetBannerEl.textContent = (headsetLabel ? `[${headsetLabel}] ` : '') + warning.message;
    }

    // Log available inputs for debugging
    if (audioInputs.length) {
        console.log('Available audio inputs:', audioInputs.map(d => d.label || '(unnamed)').join(', '));
    }

    return { headsetState, preferredMicId: state.preferredMicId };
}
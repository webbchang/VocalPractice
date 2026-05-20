// auth.js
export async function login(state, api, loadSongs, showScreen) {
    const email = document.getElementById('login-email').value;
    const password = document.getElementById('login-password').value;
    const errorEl = document.getElementById('login-error');

    if (!email || !password) {
        errorEl.textContent = '請輸入 Email 和密碼';
        return;
    }

    try {
        const result = await api('/auth/login', {
            method: 'POST',
            body: JSON.stringify({ email, password })
        });
        state.token = result.token;
        state.user = result.user;
        errorEl.textContent = '';
        await loadSongs();
    } catch (err) {
        errorEl.textContent = '登入失敗: ' + err.message;
    }
}

export function logout(state, stopAccompaniment, showScreen) {
    stopAccompaniment();
    state.token = null;
    state.user = null;
    state.selectedSong = null;
    document.getElementById('login-email').value = '';
    document.getElementById('login-password').value = '';
    showScreen('login-screen');
}

// auth.js — standalone authentication module
// No longer depends on main.js or any js/ modules

const API_BASE = '/api/v1';

async function api(path, options = {}) {
    const headers = { 'Content-Type': 'application/json', ...options.headers };
    const token = localStorage.getItem('token');
    if (token) {
        headers['Authorization'] = `Bearer ${token}`;
    }
    const res = await fetch(`${API_BASE}${path}`, { ...options, headers });
    if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        throw new Error(err.error || 'API Error');
    }
    const text = await res.text();
    return text ? JSON.parse(text) : null;
}

export async function login() {
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

        // Store credentials in localStorage for downstream pages
        localStorage.setItem('token', result.token);
        localStorage.setItem('user', JSON.stringify(result.user));

        errorEl.textContent = '';

        // Redirect based on role
        if (result.user.role === 'admin') {
            window.location.href = 'ui-screens/admin-dashboard.html';
        } else {
            window.location.href = 'ui-screens/user-practice.html';
        }
    } catch (err) {
        errorEl.textContent = '登入失敗: ' + err.message;
    }
}

export function logout() {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    window.location.href = '/index.html';
}
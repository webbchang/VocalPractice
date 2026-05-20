// === API Module ===
import { API_BASE } from './state.js';
import { state } from './state.js';

export async function api(path, options = {}) {
    const headers = { 'Content-Type': 'application/json', ...options.headers };
    if (state.token) {
        headers['Authorization'] = `Bearer ${state.token}`;
    }
    const res = await fetch(`${API_BASE}${path}`, { ...options, headers });
    if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        throw new Error(err.error || 'API Error');
    }
    const text = await res.text();
    return text ? JSON.parse(text) : null;
}
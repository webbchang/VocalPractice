import { describe, it, expect, vi, beforeEach } from 'vitest';
import { formatTime, api } from '../api.js';

describe('api.js - formatTime', () => {
    it('should return "0:00" for null/undefined', () => {
        expect(formatTime(null)).toBe('0:00');
        expect(formatTime(undefined)).toBe('0:00');
    });

    it('should return "0:00" for 0', () => {
        expect(formatTime(0)).toBe('0:00');
    });

    it('should format seconds correctly', () => {
        expect(formatTime(5)).toBe('0:05');
        expect(formatTime(59)).toBe('0:59');
        expect(formatTime(60)).toBe('1:00');
        expect(formatTime(61)).toBe('1:01');
        expect(formatTime(3661)).toBe('61:01');
    });

    it('should handle edge values', () => {
        expect(formatTime(3599)).toBe('59:59');
        expect(formatTime(3600)).toBe('60:00');
    });
});

describe('api.js - api()', () => {
    beforeEach(() => {
        vi.restoreAllMocks();
    });

    it('should call fetch with correct URL and return JSON', async () => {
        const mockData = { id: '1', title: 'Test' };
        global.fetch = vi.fn().mockResolvedValue({
            ok: true,
            json: () => Promise.resolve(mockData),
            statusText: 'OK',
        });

        const result = await api('/songs');
        expect(result).toEqual(mockData);
        expect(global.fetch).toHaveBeenCalledWith('/api/v1/songs', {
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json' },
        });
    });

    it('should merge custom options with defaults', async () => {
        global.fetch = vi.fn().mockResolvedValue({
            ok: true,
            json: () => Promise.resolve({}),
            statusText: 'OK',
        });

        await api('/songs/1', { method: 'POST', body: '{}' });
        expect(global.fetch).toHaveBeenCalledWith('/api/v1/songs/1', {
            credentials: 'same-origin',
            headers: { 'Content-Type': 'application/json' },
            method: 'POST',
            body: '{}',
        });
    });

    it('should throw error with parsed error message on HTTP error', async () => {
        global.fetch = vi.fn().mockResolvedValue({
            ok: false,
            status: 404,
            statusText: 'Not Found',
            json: () => Promise.resolve({ error: 'Resource not found' }),
        });

        await expect(api('/songs/999')).rejects.toThrow('Resource not found');
    });

    it('should throw statusText fallback if error JSON fails', async () => {
        global.fetch = vi.fn().mockResolvedValue({
            ok: false,
            status: 500,
            statusText: 'Internal Server Error',
            json: () => Promise.reject(new Error('parse fail')),
        });

        await expect(api('/songs')).rejects.toThrow('Internal Server Error');
    });
});
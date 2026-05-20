import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock state module before importing api
const mockState = { token: null };
vi.mock('../state.js', () => ({
  API_BASE: '/api/v1',
  state: mockState,
}));

const { api } = await import('../api.js');

describe('api', () => {
  beforeEach(() => {
    mockState.token = null;
    vi.restoreAllMocks();
  });

  it('should make a GET request and return parsed JSON', async () => {
    const mockData = { id: 1, name: 'test' };
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(JSON.stringify(mockData)),
    });

    const result = await api('/songs');
    expect(result).toEqual(mockData);
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/v1/songs',
      expect.objectContaining({
        headers: expect.objectContaining({ 'Content-Type': 'application/json' }),
      })
    );
  });

  it('should include Authorization header when token exists', async () => {
    mockState.token = 'test-token';
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(JSON.stringify({})),
    });

    await api('/songs');
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/v1/songs',
      expect.objectContaining({
        headers: expect.objectContaining({
          'Authorization': 'Bearer test-token',
        }),
      })
    );
  });

  it('should throw error on non-ok response', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 401,
      statusText: 'Unauthorized',
      json: () => Promise.resolve({ error: 'Invalid token' }),
    });

    await expect(api('/songs')).rejects.toThrow('Invalid token');
  });

  it('should throw statusText when error response has no JSON body', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      json: () => Promise.reject(new Error('parse error')),
    });

    await expect(api('/songs')).rejects.toThrow('Internal Server Error');
  });

  it('should return null for empty response body', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(''),
    });

    const result = await api('/songs');
    expect(result).toBeNull();
  });

  it('should pass custom headers and options', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      text: () => Promise.resolve(JSON.stringify({})),
    });

    await api('/songs', {
      method: 'POST',
      body: JSON.stringify({ title: 'New Song' }),
      headers: { 'X-Custom': 'value' },
    });

    expect(global.fetch).toHaveBeenCalledWith(
      '/api/v1/songs',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ title: 'New Song' }),
        headers: expect.objectContaining({
          'Content-Type': 'application/json',
          'X-Custom': 'value',
        }),
      })
    );
  });
});
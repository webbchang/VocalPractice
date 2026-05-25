import { describe, it, expect, vi, beforeEach } from 'vitest';
import { SongService } from '../services/api-service.js';

// Mock localStorage
beforeEach(() => {
    localStorage.clear();
    localStorage.setItem('token', 'test-token');
});

describe('SongService', () => {
    beforeEach(() => {
        vi.restoreAllMocks();
    });

    describe('_request', () => {
        it('should call fetch with correct URL for user paths', async () => {
            global.fetch = vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve({ songs: [] }),
            });

            await SongService.getSongs();
            expect(global.fetch).toHaveBeenCalledWith('/api/v1/songs', {
                credentials: 'same-origin',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': 'Bearer test-token'
                },
            });
        });

        it('should call fetch with correct URL for admin paths', async () => {
            global.fetch = vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve({ imported: 5 }),
            });

            await SongService.deleteStructure('song-1', 'struct-1');
            expect(global.fetch).toHaveBeenCalledWith('/api/v1/admin/songs/song-1/structures/struct-1', {
                credentials: 'same-origin',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': 'Bearer test-token'
                },
                method: 'DELETE',
            });
        });

        it('should throw error with data on HTTP error', async () => {
            global.fetch = vi.fn().mockResolvedValue({
                ok: false,
                status: 409,
                statusText: 'Conflict',
                json: () => Promise.resolve({ error: 'Time conflict', conflicts: [] }),
            });

            try {
                await SongService.getSongs();
                expect.unreachable();
            } catch (err) {
                expect(err.message).toContain('Time conflict');
                expect(err.status).toBe(409);
                expect(err.data.conflicts).toEqual([]);
            }
        });

        it('should use FormData headers when body is FormData', async () => {
            localStorage.setItem('token', 'test-token');
            global.fetch = vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve({}),
            });

            const formData = new FormData();
            formData.append('file', 'test');

            // Access _request directly
            await SongService._request('/admin/songs/upload', {
                method: 'POST',
                body: formData,
            });

            const callArgs = global.fetch.mock.calls[0];
            // Should NOT have Content-Type when body is FormData
            expect(callArgs[1].headers['Content-Type']).toBeUndefined();
            expect(callArgs[1].headers['Authorization']).toBe('Bearer test-token');
        });
    });

    describe('getSongs', () => {
        it('should return song list', async () => {
            const mockSongs = [{ id: '1', title: 'Song 1' }];
            global.fetch = vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve(mockSongs),
            });

            const result = await SongService.getSongs();
            expect(result).toEqual(mockSongs);
        });
    });

    describe('getSongDetail', () => {
        it('should return song detail with tracks', async () => {
            const mockDetail = { id: '1', title: 'Test', tracks: [] };
            global.fetch = vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve(mockDetail),
            });

            const result = await SongService.getSongDetail('song-1');
            expect(result).toEqual(mockDetail);
            expect(global.fetch).toHaveBeenCalledWith('/api/v1/songs/song-1', expect.anything());
        });
    });

    describe('getStructures', () => {
        it('should fetch structures without track filter', async () => {
            global.fetch = vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve({ structures: [] }),
            });

            await SongService.getStructures('song-1');
            expect(global.fetch).toHaveBeenCalledWith('/api/v1/songs/song-1/structures', expect.anything());
        });

        it('should fetch structures with track filter', async () => {
            global.fetch = vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve({ structures: [] }),
            });

            await SongService.getStructures('song-1', 'track-1');
            expect(global.fetch).toHaveBeenCalledWith(
                '/api/v1/songs/song-1/structures?track_id=track-1',
                expect.anything()
            );
        });
    });

    describe('getMIDIData', () => {
        it('should return ArrayBuffer from MIDI endpoint', async () => {
            const mockBuffer = new ArrayBuffer(8);
            global.fetch = vi.fn().mockResolvedValue({
                ok: true,
                arrayBuffer: () => Promise.resolve(mockBuffer),
            });

            const result = await SongService.getMIDIData('song-1');
            expect(result).toBe(mockBuffer);
            expect(global.fetch).toHaveBeenCalledWith(
                '/api/v1/songs/song-1/midi',
                expect.objectContaining({ credentials: 'same-origin' })
            );
        });
    });

    describe('getMidi (alias)', () => {
        it('should delegate to getMIDIData', async () => {
            const spy = vi.spyOn(SongService, 'getMIDIData').mockResolvedValue(new ArrayBuffer(8));
            await SongService.getMidi('song-1');
            expect(spy).toHaveBeenCalledWith('song-1');
        });
    });

    describe('createStructures', () => {
        it('should POST structures payload', async () => {
            const payload = { structures: [{ type: 'SECTION', title: 'Verse' }] };
            global.fetch = vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve({}),
            });

            await SongService.createStructures('song-1', payload);
            expect(global.fetch).toHaveBeenCalledWith(
                '/api/v1/admin/songs/song-1/structures',
                expect.objectContaining({
                    method: 'POST',
                    body: JSON.stringify(payload),
                })
            );
        });
    });

    describe('updateStructure', () => {
        it('should PUT updated fields', async () => {
            global.fetch = vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve({}),
            });

            await SongService.updateStructure('song-1', 'struct-1', { title: 'New Title' });
            expect(global.fetch).toHaveBeenCalledWith(
                '/api/v1/admin/songs/song-1/structures/struct-1',
                expect.objectContaining({
                    method: 'PUT',
                    body: JSON.stringify({ title: 'New Title' }),
                })
            );
        });
    });

    describe('deleteStructure', () => {
        it('should DELETE a structure', async () => {
            global.fetch = vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve({}),
            });

            await SongService.deleteStructure('song-1', 'struct-1');
            expect(global.fetch).toHaveBeenCalledWith(
                '/api/v1/admin/songs/song-1/structures/struct-1',
                expect.objectContaining({ method: 'DELETE' })
            );
        });
    });

    describe('saveLyrics', () => {
        it('should POST lyrics payload', async () => {
            const lyricsPayload = [{ track_id: 't1', structure_id: 's1', lyrics: 'Hello' }];
            global.fetch = vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve({ updated: 1, total: 1 }),
            });

            const result = await SongService.saveLyrics('song-1', lyricsPayload);
            expect(result.updated).toBe(1);
            expect(global.fetch).toHaveBeenCalledWith(
                '/api/v1/admin/songs/song-1/lyrics',
                expect.objectContaining({
                    method: 'POST',
                    body: JSON.stringify({ lyrics: lyricsPayload }),
                })
            );
        });
    });

    describe('exportCSV', () => {
        it('should fetch CSV export', async () => {
            const mockData = { csv: 'type,title\nS,Verse', song: { title: 'Test', tracks: [] } };
            global.fetch = vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve(mockData),
            });

            const result = await SongService.exportCSV('song-1');
            expect(result.csv).toContain('type,title');
            expect(global.fetch).toHaveBeenCalledWith(
                '/api/v1/admin/songs/song-1/structures/export',
                expect.anything()
            );
        });
    });

    describe('importCSV', () => {
        it('should POST CSV text with force=true', async () => {
            const csvText = 'type,title\nS,Verse';
            global.fetch = vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve({ imported: 1 }),
            });

            const result = await SongService.importCSV('song-1', csvText);
            expect(result.imported).toBe(1);
            expect(global.fetch).toHaveBeenCalledWith(
                '/api/v1/admin/songs/song-1/structures/import?force=true',
                expect.objectContaining({
                    method: 'POST',
                    headers: { 'Content-Type': 'text/csv' },
                    body: csvText,
                })
            );
        });
    });
});
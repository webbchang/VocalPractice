import { describe, it, expect, vi } from 'vitest';
import { getStorageEstimate } from '../storage.js';

describe('getStorageEstimate', () => {
  it('should return supported: false when navigator.storage is not available', async () => {
    const originalStorage = navigator.storage;
    delete navigator.storage;

    const result = await getStorageEstimate();
    expect(result).toEqual({ used: 0, total: 0, supported: false });

    navigator.storage = originalStorage;
  });

  it('should return storage estimate when API is available', async () => {
    const mockEstimate = { usage: 1024, quota: 1048576 };
    navigator.storage = {
      estimate: vi.fn().mockResolvedValue(mockEstimate),
    };

    const result = await getStorageEstimate();
    expect(result).toEqual({
      used: 1024,
      total: 1048576,
      supported: true,
    });
  });

  it('should return supported: false when estimate throws', async () => {
    navigator.storage = {
      estimate: vi.fn().mockRejectedValue(new Error('Storage error')),
    };

    const result = await getStorageEstimate();
    expect(result).toEqual({ used: 0, total: 0, supported: false });
  });

  it('should handle missing usage/quota properties', async () => {
    navigator.storage = {
      estimate: vi.fn().mockResolvedValue({}),
    };

    const result = await getStorageEstimate();
    expect(result).toEqual({ used: 0, total: 0, supported: true });
  });
});
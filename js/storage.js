// === Storage Module ===

export async function getStorageEstimate() {
    if (!navigator.storage?.estimate) {
        return { used: 0, total: 0, supported: false };
    }
    try {
        const estimate = await navigator.storage.estimate();
        return {
            used: estimate.usage || 0,
            total: estimate.quota || 0,
            supported: true
        };
    } catch {
        return { used: 0, total: 0, supported: false };
    }
}
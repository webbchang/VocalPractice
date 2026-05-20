// === Utility Module ===

export function escapeHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

export function formatDuration(seconds) {
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}:${secs.toString().padStart(2, '0')}`;
}

export function formatStorage(bytes) {
    if (bytes === 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
}

export function getScoreClass(score) {
    if (score >= 80) return 'green';
    if (score >= 50) return 'yellow';
    return 'red';
}

export function sortStructures(nodes) {
    nodes.sort((a, b) => (a.order_index || a.song_structure?.order_index || 0) - (b.order_index || b.song_structure?.order_index || 0));
    for (const node of nodes) {
        if (node.phrases && node.phrases.length) {
            sortStructures(node.phrases);
        }
    }
}

export function flattenStructures(nodes) {
    let result = [];
    for (const node of nodes) {
        result.push(node);
        if (node.phrases && node.phrases.length) {
            result = result.concat(flattenStructures(node.phrases));
        }
    }
    return result;
}

export function findStructureTitle(id, structures) {
    const allStructures = flattenStructures(structures);
    const found = allStructures.find(s => (s.id || s.song_structure?.id) === id);
    return found ? (found.title || found.song_structure?.title || '') : '';
}
import { describe, it, expect } from 'vitest';
import { escapeHtml, formatDuration, formatStorage, getScoreClass, flattenStructures, findStructureTitle } from '../utils.js';

function ent(s) {
  return s;
}

describe('escapeHtml', () => {
  it('should return empty string for falsy values', () => {
    expect(escapeHtml(null)).toBe('');
    expect(escapeHtml(undefined)).toBe('');
    expect(escapeHtml('')).toBe('');
  });

  it('should escape HTML special characters', () => {
    const output1 = escapeHtml('<script>');
    expect(output1).toBe(escapeHtml('<') + 'script' + escapeHtml('>'));
    const output2 = escapeHtml('"quoted"');
    expect(output2).toBe(escapeHtml('"') + 'quoted' + escapeHtml('"'));
    expect(escapeHtml("'single'")).toBe("'single'");
    const output3 = escapeHtml('a & b');
    expect(output3).toBe('a ' + escapeHtml('&') + ' b');
  });

  it('should return the same string for plain text', () => {
    expect(escapeHtml('hello world')).toBe('hello world');
  });
});

describe('formatDuration', () => {
  it('should format seconds into m:ss format', () => {
    expect(formatDuration(0)).toBe('0:00');
    expect(formatDuration(5)).toBe('0:05');
    expect(formatDuration(65)).toBe('1:05');
    expect(formatDuration(3661)).toBe('61:01');
  });
});

describe('formatStorage', () => {
  it('should return "0 B" for 0 bytes', () => {
    expect(formatStorage(0)).toBe('0 B');
  });

  it('should format bytes with appropriate units', () => {
    expect(formatStorage(1024)).toBe('1.0 KB');
    expect(formatStorage(1048576)).toBe('1.0 MB');
    expect(formatStorage(1073741824)).toBe('1.0 GB');
    expect(formatStorage(500)).toBe('500.0 B');
  });
});

describe('getScoreClass', () => {
  it('should return "green" for scores >= 80', () => {
    expect(getScoreClass(80)).toBe('green');
    expect(getScoreClass(100)).toBe('green');
  });

  it('should return "yellow" for scores >= 50 and < 80', () => {
    expect(getScoreClass(50)).toBe('yellow');
    expect(getScoreClass(79)).toBe('yellow');
  });

  it('should return "red" for scores < 50', () => {
    expect(getScoreClass(0)).toBe('red');
    expect(getScoreClass(49)).toBe('red');
  });
});

describe('flattenStructures', () => {
  it('should return empty array for empty input', () => {
    expect(flattenStructures([])).toEqual([]);
  });

  it('should flatten nested structures', () => {
    const nodes = [
      { id: '1', title: 'A', phrases: [
        { id: '1.1', title: 'A1', phrases: [
          { id: '1.1.1', title: 'A1a' }
        ]}
      ]},
      { id: '2', title: 'B', phrases: [] }
    ];
    const result = flattenStructures(nodes);
    expect(result).toHaveLength(4);
    expect(result.map(n => n.id)).toEqual(['1', '1.1', '1.1.1', '2']);
  });
});

describe('findStructureTitle', () => {
  it('should find title by direct id', () => {
    const structures = [
      { id: '1', title: 'Verse' },
      { id: '2', title: 'Chorus' }
    ];
    expect(findStructureTitle('1', structures)).toBe('Verse');
    expect(findStructureTitle('2', structures)).toBe('Chorus');
  });

  it('should find title by nested song_structure id', () => {
    const structures = [
      { song_structure: { id: '1', title: 'Verse' } },
      { song_structure: { id: '2', title: 'Chorus' } }
    ];
    expect(findStructureTitle('1', structures)).toBe('Verse');
  });

  it('should return empty string for missing id', () => {
    expect(findStructureTitle('nonexistent', [])).toBe('');
  });

  it('should find title in nested structures', () => {
    const structures = [
      { id: '1', title: 'A', phrases: [
        { id: '1.1', title: 'A1', phrases: [
          { id: '1.1.1', title: 'Deep' }
        ]}
      ]}
    ];
    expect(findStructureTitle('1.1.1', structures)).toBe('Deep');
  });
});
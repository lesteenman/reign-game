import { describe, it, expect } from 'vitest';
import { parseFlowType, buildCurationFlowId } from './types';

describe('parseFlowType', () => {
  it('returns the value for known FlowType strings', () => {
    // Arrange — known values from the union
    // Act + Assert
    expect(parseFlowType('curation')).toBe('curation');
    expect(parseFlowType('daily')).toBe('daily');
    expect(parseFlowType('pack')).toBe('pack');
  });

  it('returns null for unrecognized strings', () => {
    // Arrange — common typos and non-flow values
    // Act + Assert
    expect(parseFlowType('curations')).toBeNull();
    expect(parseFlowType('admin')).toBeNull();
    expect(parseFlowType('')).toBeNull();
  });

  it('returns null for missing parameter', () => {
    // Arrange + Act + Assert
    expect(parseFlowType(null)).toBeNull();
  });
});

describe('buildCurationFlowId', () => {
  it('builds the canonical curation flowId from size and mode', () => {
    // Arrange + Act + Assert
    expect(buildCurationFlowId(5, 'standard')).toBe('5x5-standard');
    expect(buildCurationFlowId(7, 'double')).toBe('7x7-double');
    expect(buildCurationFlowId(9, 'standard')).toBe('9x9-standard');
  });
});

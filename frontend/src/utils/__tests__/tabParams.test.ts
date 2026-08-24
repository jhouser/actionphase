import { describe, it, expect } from 'vitest';
import { clearForeignTabParams, TAB_OWNED_PARAMS } from '../tabParams';

const clear = (search: string, tabId: string) => {
  const params = new URLSearchParams(search);
  clearForeignTabParams(params, tabId);
  return params;
};

describe('clearForeignTabParams', () => {
  it('drops the history drill-down when navigating to another tab', () => {
    const params = clear('tab=history&phase=12&subTab=results&characters=3,4', 'people');
    expect(params.get('phase')).toBeNull();
    expect(params.get('subTab')).toBeNull();
    expect(params.get('characters')).toBeNull();
  });

  it('drops the common room sub-view when navigating to another tab', () => {
    const params = clear('tab=common-room&view=polls&poll=9', 'history');
    expect(params.get('view')).toBeNull();
    expect(params.get('poll')).toBeNull();
  });

  it('keeps a tab its own params when navigating to it', () => {
    const params = clear('tab=people&phase=12&subTab=results', 'history');
    // Arriving at history from elsewhere must not carry a stale drill-down,
    // but params legitimately targeted at history are preserved.
    const target = clear('phase=12&subTab=results', 'history');
    expect(target.get('phase')).toBe('12');
    expect(target.get('subTab')).toBe('results');
    expect(params.get('phase')).toBe('12');
  });

  it('preserves non-tab params such as tab itself', () => {
    const params = clear('tab=history&phase=12&highlight=abc', 'people');
    expect(params.get('tab')).toBe('history');
    expect(params.get('highlight')).toBe('abc');
  });

  it('keeps comment when moving between the two tabs that deep-link it', () => {
    expect(clear('comment=55', 'history').get('comment')).toBe('55');
    expect(clear('comment=55', 'common-room').get('comment')).toBe('55');
    expect(clear('comment=55', 'people').get('comment')).toBeNull();
  });

  it('clears every other tab-owned param for an unknown tab', () => {
    const all = Object.values(TAB_OWNED_PARAMS).flat();
    const params = clear(all.map(k => `${k}=x`).join('&'), 'info');
    all.forEach(key => expect(params.get(key)).toBeNull());
  });
});

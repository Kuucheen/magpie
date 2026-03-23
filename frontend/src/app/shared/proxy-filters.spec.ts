import {
  activeProxyFilterCount,
  buildFiltersFromFormValue,
  buildProxyListFilterPayload,
  createDefaultProxyFilterValues,
} from './proxy-filters';

describe('proxy-filters', () => {
  it('builds health filter values into applied filters and payload', () => {
    const formValue = {
      ...createDefaultProxyFilterValues(),
      minHealthOverall: 65,
      minHealthHttp: 100,
      minHealthHttps: 101,
      minHealthSocks4: -2,
      minHealthSocks5: 45,
    };

    const filters = buildFiltersFromFormValue(formValue);

    expect(filters.minHealthOverall).toBe(65);
    expect(filters.minHealthHttp).toBe(100);
    expect(filters.minHealthHttps).toBe(100);
    expect(filters.minHealthSocks4).toBe(0);
    expect(filters.minHealthSocks5).toBe(45);

    expect(activeProxyFilterCount(filters)).toBe(4);
    expect(buildProxyListFilterPayload(filters)).toEqual({
      minHealthOverall: 65,
      minHealthHttp: 100,
      minHealthHttps: 100,
      minHealthSocks5: 45,
    });
  });
});

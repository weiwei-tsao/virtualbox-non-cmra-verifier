import { Mailbox, CrawlRun, MailboxFilter, Stats } from '../types';

const API_BASE = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080';

const toQueryString = (params: Record<string, string | number | undefined>) =>
  Object.entries(params)
    .filter(([, v]) => v !== undefined && v !== '')
    .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`)
    .join('&');

const request = async (path: string, init?: RequestInit) => {
  const res = await fetch(`${API_BASE}${path}`, {
    cache: 'no-store',
    headers: { 'Accept': 'application/json', ...(init?.headers || {}) },
    ...init,
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res;
};

export const api = {
  getMailboxes: async (filter: MailboxFilter) => {
    const qs = toQueryString({
      state: filter.state,
      cmra: filter.cmra,
      rdi: filter.rdi,
      active: 'true',
      page: filter.page,
      pageSize: filter.pageSize,
    });
    const res = await request(`/api/mailboxes?${qs}`);
    const data = await res.json();
    const items: Mailbox[] = (data.items || []).map((m: any) => ({
      id: m.id || m.link,
      name: m.name,
      street: m.addressRaw?.street,
      city: m.addressRaw?.city,
      state: m.addressRaw?.state,
      zip: m.addressRaw?.zip,
      price: m.price || 0,
      link: m.link,
      cmra: m.cmra || 'Unknown',
      rdi: m.rdi || 'Unknown',
      standardizedAddress: m.standardizedAddress,
      lastValidatedAt: m.lastValidatedAt,
      crawlRunId: m.crawlRunId,
    }));
    return { items, total: data.total || 0, page: data.page || filter.page };
  },

  getStats: async (): Promise<Stats> => {
    const res = await request('/api/stats');
    const data = await res.json();
    const byState = Object.entries(data.byState || {}).map(([name, value]) => ({
      name,
      value: Number(value),
    }));
    return {
      totalMailboxes: data.totalMailboxes || 0,
      commercialCount: data.totalCommercial || 0,
      residentialCount: data.totalResidential || 0,
      avgPrice: data.avgPrice || 0,
      byState,
    };
  },

  triggerCrawl: async (links: string[]): Promise<string> => {
    const res = await request('/api/crawl/run', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ links }),
    });
    const data = await res.json();
    return data.runId;
  },

  getCrawlStatus: async (runId: string): Promise<CrawlRun> => {
    const res = await request(`/api/crawl/status?runId=${encodeURIComponent(runId)}&ts=${Date.now()}`);
    const data = await res.json();
    return {
      id: data.runId,
      startedAt: data.startedAt,
      finishedAt: data.finishedAt,
      status: data.status,
      stats: data.stats || { found: 0, validated: 0, skipped: 0, failed: 0 },
      errorsSample: data.errorsSample || [],
    };
  },
  
  exportCSV: async () => {
    const url = `${API_BASE}/api/mailboxes/export`;
    window.open(url, '_blank');
    return true;
  }
};

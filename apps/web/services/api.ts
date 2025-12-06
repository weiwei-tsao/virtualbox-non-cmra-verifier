import { Mailbox, CrawlRun, MailboxFilter, Stats } from '../types';
import { generateMailboxes, generateCrawlRuns } from './mockData';

// Simulated database
let DB_MAILBOXES = generateMailboxes(250);
let DB_RUNS = generateCrawlRuns();

const delay = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));

export const api = {
  getMailboxes: async (filter: MailboxFilter) => {
    await delay(600);
    
    let filtered = DB_MAILBOXES.filter(item => {
      if (filter.state && item.state !== filter.state) return false;
      if (filter.cmra && item.cmra !== filter.cmra) return false;
      if (filter.rdi && item.rdi !== filter.rdi) return false;
      if (filter.search) {
        const lowerQ = filter.search.toLowerCase();
        return item.name.toLowerCase().includes(lowerQ) || 
               item.city.toLowerCase().includes(lowerQ) ||
               item.street.toLowerCase().includes(lowerQ);
      }
      return true;
    });

    const total = filtered.length;
    const start = (filter.page - 1) * filter.pageSize;
    const items = filtered.slice(start, start + filter.pageSize);

    return { items, total, page: filter.page };
  },

  getStats: async (): Promise<Stats> => {
    await delay(400);
    const stateMap = new Map<string, number>();
    let comm = 0;
    let res = 0;
    let totalPrice = 0;

    DB_MAILBOXES.forEach(m => {
      stateMap.set(m.state, (stateMap.get(m.state) || 0) + 1);
      if (m.rdi === 'Commercial') comm++;
      else res++;
      totalPrice += m.price;
    });

    return {
      totalMailboxes: DB_MAILBOXES.length,
      commercialCount: comm,
      residentialCount: res,
      avgPrice: totalPrice / DB_MAILBOXES.length,
      byState: Array.from(stateMap.entries()).map(([name, value]) => ({ name, value }))
    };
  },

  getCrawlRuns: async (): Promise<CrawlRun[]> => {
    await delay(300);
    return [...DB_RUNS];
  },

  triggerCrawl: async (): Promise<string> => {
    await delay(800);
    const newRun: CrawlRun = {
      id: `RUN_${new Date().getTime()}`,
      startedAt: new Date().toISOString(),
      status: 'running',
      totalFound: 0,
      totalValidated: 0,
      totalFailed: 0,
      errorsSample: []
    };
    DB_RUNS.unshift(newRun);
    return newRun.id;
  },
  
  exportCSV: async () => {
    // In a real app, this would hit the backend export endpoint
    console.log("Export triggered");
    return true;
  }
};
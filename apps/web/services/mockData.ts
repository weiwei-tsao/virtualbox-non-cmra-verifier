import { Mailbox, CrawlRun } from '../types';

const STATES = ['CA', 'NY', 'TX', 'FL', 'WA', 'NV', 'DE'];
const CITIES: Record<string, string[]> = {
  CA: ['San Francisco', 'Los Angeles', 'San Diego', 'Sacramento'],
  NY: ['New York', 'Brooklyn', 'Albany', 'Buffalo'],
  TX: ['Austin', 'Houston', 'Dallas', 'San Antonio'],
  FL: ['Miami', 'Orlando', 'Tampa', 'Jacksonville'],
  WA: ['Seattle', 'Bellevue', 'Tacoma', 'Spokane'],
  NV: ['Las Vegas', 'Reno', 'Henderson', 'Carson City'],
  DE: ['Wilmington', 'Dover', 'Newark', 'Middletown']
};

export const generateMailboxes = (count: number): Mailbox[] => {
  return Array.from({ length: count }).map((_, i) => {
    const state = STATES[Math.floor(Math.random() * STATES.length)];
    const cities = CITIES[state];
    const city = cities[Math.floor(Math.random() * cities.length)];
    const isCommercial = Math.random() > 0.3;
    
    return {
      id: `mb_${i + 1000}`,
      name: `${city} Mail Center #${i + 1}`,
      street: `${Math.floor(Math.random() * 9000) + 100} Main St`,
      city,
      state,
      zip: `${Math.floor(Math.random() * 89999) + 10000}`,
      price: parseFloat((Math.random() * 50 + 9.99).toFixed(2)),
      link: `https://anytimemailbox.com/l/${city.toLowerCase().replace(' ', '-')}`,
      cmra: isCommercial ? 'Y' : 'N',
      rdi: isCommercial ? 'Commercial' : 'Residential',
      lastValidatedAt: new Date(Date.now() - Math.floor(Math.random() * 1000000000)).toISOString(),
      crawlRunId: 'RUN_2025_01_01_001',
      standardizedAddress: {
        deliveryLine1: `${Math.floor(Math.random() * 9000) + 100} MAIN ST`,
        lastLine: `${city.toUpperCase()} ${state} ${Math.floor(Math.random() * 89999) + 10000}`
      }
    };
  });
};

export const generateCrawlRuns = (): CrawlRun[] => [
  {
    id: 'RUN_2025_05_21_003',
    startedAt: new Date().toISOString(),
    status: 'running',
    totalFound: 142,
    totalValidated: 89,
    totalFailed: 0,
    errorsSample: []
  },
  {
    id: 'RUN_2025_05_20_002',
    startedAt: new Date(Date.now() - 86400000).toISOString(),
    finishedAt: new Date(Date.now() - 82800000).toISOString(),
    status: 'success',
    totalFound: 2300,
    totalValidated: 2285,
    totalFailed: 15,
    errorsSample: []
  },
  {
    id: 'RUN_2025_05_19_001',
    startedAt: new Date(Date.now() - 172800000).toISOString(),
    finishedAt: new Date(Date.now() - 172000000).toISOString(),
    status: 'failed',
    totalFound: 500,
    totalValidated: 100,
    totalFailed: 400,
    errorsSample: [
      { link: 'https://anytimemailbox.com/...', reason: 'Smarty API 402 Payment Required' }
    ]
  }
];
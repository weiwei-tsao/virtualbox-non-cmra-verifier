import React, { useState } from 'react';
import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query';
import { api } from '../services/api';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { TrendingUp, Building, Home, DollarSign, RefreshCw, Database } from 'lucide-react';
import { MailboxFilter } from '../types';

// Filter types for the bar chart
type ChartFilter = 'all' | 'avgPrice' | 'commercial' | 'residential' | 'ATMB' | 'iPost1';

interface StatCardProps {
  title: string;
  value: string;
  icon: React.ReactNode;
  color: string;
  onClick?: () => void;
  isActive?: boolean;
}

const StatCard = ({ title, value, icon, color, onClick, isActive }: StatCardProps) => (
  <div
    className={`bg-white overflow-hidden rounded-lg shadow-sm border-2 transition-all ${
      isActive
        ? 'border-primary ring-2 ring-primary ring-opacity-50'
        : 'border-gray-200 hover:border-gray-300'
    } ${onClick ? 'cursor-pointer' : ''}`}
    onClick={onClick}
  >
    <div className="p-5">
      <div className="flex items-center">
        <div className="flex-shrink-0">
          <div className={`p-3 rounded-md ${color} bg-opacity-10`}>
            {React.isValidElement(icon)
              ? React.cloneElement(icon as React.ReactElement<{ className?: string }>, { className: `h-6 w-6 ${color.replace('bg-', 'text-')}` })
              : icon}
          </div>
        </div>
        <div className="ml-5 w-0 flex-1">
          <dl>
            <dt className="text-sm font-medium text-gray-500 truncate">{title}</dt>
            <dd>
              <div className="text-lg font-medium text-gray-900">{value}</div>
            </dd>
          </dl>
        </div>
      </div>
    </div>
  </div>
);

export const Analytics: React.FC = () => {
  const queryClient = useQueryClient();
  const [activeFilter, setActiveFilter] = useState<ChartFilter>('all');

  const { data: stats, isLoading } = useQuery({
    queryKey: ['stats'],
    queryFn: api.getStats,
  });

  // Query for filtered mailboxes when a filter is selected
  const { data: filteredData, isLoading: isLoadingFiltered } = useQuery({
    queryKey: ['chartData', activeFilter],
    queryFn: async () => {
      if (activeFilter === 'all') return null;

      // Build filter based on active selection
      const filter: MailboxFilter = {
        page: 1,
        pageSize: 10000,
      };

      if (activeFilter === 'ATMB' || activeFilter === 'iPost1') {
        filter.source = activeFilter;
      } else if (activeFilter === 'commercial') {
        filter.rdi = 'Commercial';
      } else if (activeFilter === 'residential') {
        filter.rdi = 'Residential';
      }
      // For avgPrice, we need all mailboxes

      const result = await api.getMailboxes(filter);

      if (activeFilter === 'avgPrice') {
        // Calculate average price by state
        const stateData: Record<string, { total: number; count: number }> = {};
        result.items.forEach(m => {
          if (m.state) {
            if (!stateData[m.state]) {
              stateData[m.state] = { total: 0, count: 0 };
            }
            stateData[m.state].total += m.price || 0;
            stateData[m.state].count += 1;
          }
        });
        return Object.entries(stateData).map(([name, data]) => ({
          name,
          value: data.count > 0 ? Math.round(data.total / data.count * 100) / 100 : 0,
        }));
      } else {
        // Aggregate count by state
        const byState: Record<string, number> = {};
        result.items.forEach(m => {
          if (m.state) {
            byState[m.state] = (byState[m.state] || 0) + 1;
          }
        });
        return Object.entries(byState).map(([name, value]) => ({ name, value }));
      }
    },
    enabled: activeFilter !== 'all',
  });

  const refreshMutation = useMutation({
    mutationFn: api.refreshStats,
    onSuccess: (data) => {
      queryClient.setQueryData(['stats'], data);
    },
  });

  if (isLoading) return <div className="p-10 text-center">Loading analytics...</div>;
  if (!stats) return null;

  // Data to display in bar chart
  const displayByState = activeFilter === 'all' ? stats.byState : (filteredData || []);

  // Chart title based on filter
  const getChartTitle = () => {
    switch (activeFilter) {
      case 'avgPrice': return 'Avg. Price by State';
      case 'commercial': return 'Commercial Mailboxes by State';
      case 'residential': return 'Residential Mailboxes by State';
      case 'ATMB': return 'ATMB Mailboxes by State';
      case 'iPost1': return 'iPost1 Mailboxes by State';
      default: return 'Mailboxes by State';
    }
  };

  // Bar color based on filter
  const getBarColor = () => {
    switch (activeFilter) {
      case 'avgPrice': return '#10B981'; // green
      case 'commercial': return '#3B82F6'; // blue
      case 'residential': return '#8B5CF6'; // purple
      case 'ATMB': return '#4F46E5'; // indigo
      case 'iPost1': return '#F59E0B'; // amber
      default: return '#4F46E5';
    }
  };

  const formatLastUpdated = (dateStr?: string) => {
    if (!dateStr) return 'Unknown';
    const date = new Date(dateStr);
    return date.toLocaleString();
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Analytics Overview</h1>
        <div className="flex items-center gap-4">
          <span className="text-sm text-gray-500">
            Last updated: {formatLastUpdated(stats.lastUpdated)}
          </span>
          <button
            onClick={() => refreshMutation.mutate()}
            disabled={refreshMutation.isPending}
            className="inline-flex items-center px-3 py-2 border border-gray-300 shadow-sm text-sm leading-4 font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <RefreshCw className={`h-4 w-4 mr-2 ${refreshMutation.isPending ? 'animate-spin' : ''}`} />
            {refreshMutation.isPending ? 'Refreshing...' : 'Refresh Stats'}
          </button>
        </div>
      </div>

      {/* KPI Cards - Clickable Filters */}
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="Total Mailboxes"
          value={stats.totalMailboxes.toLocaleString()}
          icon={<TrendingUp />}
          color="bg-primary text-primary"
          onClick={() => setActiveFilter('all')}
          isActive={activeFilter === 'all'}
        />
        <StatCard
          title="Avg. Price"
          value={`$${stats.avgPrice.toFixed(2)}`}
          icon={<DollarSign />}
          color="bg-green-600 text-green-600"
          onClick={() => setActiveFilter('avgPrice')}
          isActive={activeFilter === 'avgPrice'}
        />
        <StatCard
          title="Commercial (RDI)"
          value={stats.commercialCount.toLocaleString()}
          icon={<Building />}
          color="bg-blue-600 text-blue-600"
          onClick={() => setActiveFilter('commercial')}
          isActive={activeFilter === 'commercial'}
        />
        <StatCard
          title="Residential (RDI)"
          value={stats.residentialCount.toLocaleString()}
          icon={<Home />}
          color="bg-purple-600 text-purple-600"
          onClick={() => setActiveFilter('residential')}
          isActive={activeFilter === 'residential'}
        />
      </div>

      {/* Source Breakdown Cards - Clickable Filters */}
      {stats.bySource.length > 0 && (
        <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
          {stats.bySource.map((source) => (
            <StatCard
              key={source.name}
              title={`${source.name} Mailboxes`}
              value={source.value.toLocaleString()}
              icon={<Database />}
              color={source.name === 'ATMB' ? 'bg-indigo-600 text-indigo-600' : 'bg-amber-500 text-amber-500'}
              onClick={() => setActiveFilter(source.name as ChartFilter)}
              isActive={activeFilter === source.name}
            />
          ))}
        </div>
      )}

      {/* Bar Chart - Dynamic based on selected filter */}
      <div className="bg-white p-6 rounded-lg shadow-sm border border-gray-200">
        <h3 className="text-lg font-medium leading-6 text-gray-900 mb-4">{getChartTitle()}</h3>
        <div className="h-80 relative">
          {isLoadingFiltered && activeFilter !== 'all' && (
            <div className="absolute inset-0 bg-white/50 flex items-center justify-center z-10">
              <span className="text-gray-500">Loading...</span>
            </div>
          )}
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={displayByState}>
              <CartesianGrid strokeDasharray="3 3" vertical={false} />
              <XAxis dataKey="name" />
              <YAxis />
              <Tooltip formatter={(value: number) => activeFilter === 'avgPrice' ? `$${value.toFixed(2)}` : value} />
              <Bar dataKey="value" fill={getBarColor()} radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>

    </div>
  );
};

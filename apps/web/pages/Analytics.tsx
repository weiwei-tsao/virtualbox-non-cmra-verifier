import React from 'react';
import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query';
import { api } from '../services/api';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts';
import { TrendingUp, Building, Home, DollarSign, RefreshCw, Database } from 'lucide-react';

const StatCard = ({ title, value, icon, color }: { title: string, value: string, icon: React.ReactNode, color: string }) => (
  <div className="bg-white overflow-hidden rounded-lg shadow-sm border border-gray-200">
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

const SOURCE_COLORS: Record<string, string> = {
  'ATMB': '#4F46E5',
  'iPost1': '#F59E0B',
  'default': '#6B7280',
};

export const Analytics: React.FC = () => {
  const queryClient = useQueryClient();

  const { data: stats, isLoading } = useQuery({
    queryKey: ['stats'],
    queryFn: api.getStats,
  });

  const refreshMutation = useMutation({
    mutationFn: api.refreshStats,
    onSuccess: (data) => {
      queryClient.setQueryData(['stats'], data);
    },
  });

  if (isLoading) return <div className="p-10 text-center">Loading analytics...</div>;
  if (!stats) return null;

  const pieData = [
    { name: 'Commercial', value: stats.commercialCount },
    { name: 'Residential', value: stats.residentialCount },
  ];
  const RDI_COLORS = ['#4F46E5', '#10B981'];

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

      {/* KPI Cards */}
      <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="Total Mailboxes"
          value={stats.totalMailboxes.toLocaleString()}
          icon={<TrendingUp />}
          color="bg-primary text-primary"
        />
        <StatCard
          title="Avg. Price"
          value={`$${stats.avgPrice.toFixed(2)}`}
          icon={<DollarSign />}
          color="bg-green-600 text-green-600"
        />
        <StatCard
          title="Commercial (RDI)"
          value={stats.commercialCount.toLocaleString()}
          icon={<Building />}
          color="bg-blue-600 text-blue-600"
        />
        <StatCard
          title="Residential (RDI)"
          value={stats.residentialCount.toLocaleString()}
          icon={<Home />}
          color="bg-purple-600 text-purple-600"
        />
      </div>

      {/* Source Breakdown Cards */}
      {stats.bySource.length > 0 && (
        <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4">
          {stats.bySource.map((source) => (
            <StatCard
              key={source.name}
              title={`${source.name} Mailboxes`}
              value={source.value.toLocaleString()}
              icon={<Database />}
              color={source.name === 'ATMB' ? 'bg-indigo-600 text-indigo-600' : 'bg-amber-500 text-amber-500'}
            />
          ))}
        </div>
      )}

      {/* Bar Chart - Mailboxes by State (Full Width) */}
      <div className="bg-white p-6 rounded-lg shadow-sm border border-gray-200">
        <h3 className="text-lg font-medium leading-6 text-gray-900 mb-4">Mailboxes by State</h3>
        <div className="h-80">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={stats.byState}>
              <CartesianGrid strokeDasharray="3 3" vertical={false} />
              <XAxis dataKey="name" />
              <YAxis />
              <Tooltip />
              <Bar dataKey="value" fill="#4F46E5" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Pie Charts Row - 50% each */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Pie Chart - Residential vs Commercial */}
        <div className="bg-white p-6 rounded-lg shadow-sm border border-gray-200">
          <h3 className="text-lg font-medium leading-6 text-gray-900 mb-4">Residential vs Commercial</h3>
          <div className="h-80">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={pieData}
                  cx="50%"
                  cy="50%"
                  labelLine={false}
                  label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
                  outerRadius={100}
                  fill="#8884d8"
                  dataKey="value"
                >
                  {pieData.map((_, index) => (
                    <Cell key={`cell-${index}`} fill={RDI_COLORS[index % RDI_COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip />
              </PieChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Pie Chart - By Source */}
        <div className="bg-white p-6 rounded-lg shadow-sm border border-gray-200">
          <h3 className="text-lg font-medium leading-6 text-gray-900 mb-4">Mailboxes by Source</h3>
          <div className="h-80">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={stats.bySource}
                  cx="50%"
                  cy="50%"
                  labelLine={false}
                  label={({ name, percent }) => `${name} ${(percent * 100).toFixed(0)}%`}
                  outerRadius={100}
                  fill="#8884d8"
                  dataKey="value"
                >
                  {stats.bySource.map((entry) => (
                    <Cell key={`cell-${entry.name}`} fill={SOURCE_COLORS[entry.name] || SOURCE_COLORS.default} />
                  ))}
                </Pie>
                <Tooltip />
              </PieChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>
    </div>
  );
};

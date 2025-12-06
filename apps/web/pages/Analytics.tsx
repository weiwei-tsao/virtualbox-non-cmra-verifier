import React, { useEffect, useState } from 'react';
import { api } from '../services/api';
import { Stats } from '../types';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts';
import { TrendingUp, Building, Home, DollarSign } from 'lucide-react';

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

export const Analytics: React.FC = () => {
  const [stats, setStats] = useState<Stats | null>(null);

  useEffect(() => {
    api.getStats().then(setStats);
  }, []);

  if (!stats) return <div className="p-10 text-center">Loading analytics...</div>;

  const pieData = [
    { name: 'Commercial', value: stats.commercialCount },
    { name: 'Residential', value: stats.residentialCount },
  ];
  const COLORS = ['#4F46E5', '#10B981'];

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Analytics Overview</h1>

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

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Bar Chart */}
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

        {/* Pie Chart */}
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
                  {pieData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
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
import React from 'react';

interface StatCardProps {
  title: string;
  value: string;
  icon: React.ReactNode;
  color: string;
  onClick?: () => void;
  isActive?: boolean;
}

export const StatCard: React.FC<StatCardProps> = ({ title, value, icon, color, onClick, isActive }) => (
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

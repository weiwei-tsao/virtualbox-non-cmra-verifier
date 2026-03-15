import React from 'react';
import { useToast } from '../contexts/ToastContext';
import { Toast } from './Toast';

export const ToastContainer: React.FC = () => {
  const { toasts, hideToast } = useToast();

  if (toasts.length === 0) return null;

  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2 pointer-events-none">
      {toasts.slice(-3).map(toast => (
        <div key={toast.id} className="pointer-events-auto">
          <Toast toast={toast} onClose={() => hideToast(toast.id)} />
        </div>
      ))}
    </div>
  );
};

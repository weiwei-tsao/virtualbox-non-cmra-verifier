import React, { useState } from 'react';
import { Layout } from './components/Layout';
import { Mailboxes } from './pages/Mailboxes';
import { Analytics } from './pages/Analytics';
import { Crawler } from './pages/Crawler';

function App() {
  const [page, setPage] = useState('dashboard');

  const renderPage = () => {
    switch (page) {
      case 'dashboard':
        return <Mailboxes />;
      case 'analytics':
        return <Analytics />;
      case 'crawler':
        return <Crawler />;
      case 'settings':
        return (
          <div className='p-10 text-center text-gray-500'>
            Settings placeholder (API Config, User Mgmt)
          </div>
        );
      default:
        return <Mailboxes />;
    }
  };

  return (
    <Layout currentPage={page} onNavigate={setPage}>
      {renderPage()}
    </Layout>
  );
}

export default App;

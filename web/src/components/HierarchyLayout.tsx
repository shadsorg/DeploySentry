import { useEffect } from 'react';
import { Outlet, useParams } from 'react-router-dom';
import Sidebar from './Sidebar';
import Breadcrumb from './Breadcrumb';
import RealtimeManager from '@/services/realtime';

export default function HierarchyLayout() {
  const { orgSlug, projectSlug, appSlug } = useParams();

  // Persist last-used context for DefaultRedirect and LegacyRedirect
  useEffect(() => {
    if (orgSlug) localStorage.setItem('ds_last_org', orgSlug);
    if (projectSlug) localStorage.setItem('ds_last_project', projectSlug);
    if (appSlug) localStorage.setItem('ds_last_app', appSlug);
  }, [orgSlug, projectSlug, appSlug]);

  // Initialize realtime (moved from App.tsx)
  useEffect(() => {
    const initializeRealtime = async () => {
      try {
        const realtimeManager = RealtimeManager.getInstance();
        await realtimeManager.initialize({
          baseUrl: window.location.origin,
          refreshInterval: 30000,
        });
      } catch (error) {
        console.warn('[HierarchyLayout] Failed to initialize real-time updates:', error);
      }
    };

    initializeRealtime();
    return () => { RealtimeManager.getInstance().dispose(); };
  }, []);

  return (
    <div className="app-layout">
      <Sidebar />
      <main className="main-content">
        <Breadcrumb />
        <Outlet />
      </main>
    </div>
  );
}

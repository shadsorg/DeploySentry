import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { AuthProvider } from './auth';
import { SessionExpiryWarning } from './components/SessionExpiryWarning';
import { AppRoutes } from './App';
import { initServiceWorker } from './registerSW';
import './styles/mobile.css';

initServiceWorker();

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter basename="/m">
      <AuthProvider>
        <SessionExpiryWarning />
        <AppRoutes />
      </AuthProvider>
    </BrowserRouter>
  </React.StrictMode>,
);

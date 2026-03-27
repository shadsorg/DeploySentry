import React from 'react';

// ---------------------------------------------------------------------------
// Mock data
// ---------------------------------------------------------------------------

const FLAG_CATEGORIES = [
  { key: 'release', label: 'Release', count: 12 },
  { key: 'feature', label: 'Feature', count: 18 },
  { key: 'experiment', label: 'Experiment', count: 8 },
  { key: 'ops', label: 'Ops', count: 5 },
  { key: 'permission', label: 'Permission', count: 4 },
] as const;

const TOTAL_FLAGS = FLAG_CATEGORIES.reduce((sum, c) => sum + c.count, 0);

interface ActivityEvent {
  id: string;
  description: React.ReactNode;
  actor: string;
  timestamp: string;
  warning?: boolean;
}

const RECENT_ACTIVITY: ActivityEvent[] = [
  {
    id: 'evt-1',
    description: (
      <>
        Flag <code className="font-mono">new-checkout-flow</code> toggled{' '}
        <span className="badge badge-enabled">ON</span>
      </>
    ),
    actor: 'alice@example.com',
    timestamp: '2 min ago',
  },
  {
    id: 'evt-2',
    description: (
      <>
        Deployment <strong>v2.4.1</strong> promoted to production
      </>
    ),
    actor: 'ci/deploy-bot',
    timestamp: '15 min ago',
  },
  {
    id: 'evt-3',
    description: (
      <>
        Flag <code className="font-mono">vendor-api-circuit-breaker</code> created
      </>
    ),
    actor: 'team-platform',
    timestamp: '1 hour ago',
  },
  {
    id: 'evt-4',
    description: (
      <>
        Release <strong>v2.4.0</strong> archived
      </>
    ),
    actor: 'bob@example.com',
    timestamp: '3 hours ago',
  },
  {
    id: 'evt-5',
    description: (
      <>
        Flag <code className="font-mono">dark-mode-v2</code> expired &mdash; needs
        cleanup
      </>
    ),
    actor: 'system',
    timestamp: '1 day ago',
    warning: true,
  },
];

interface ExpiringFlag {
  name: string;
  category: 'release' | 'feature' | 'experiment' | 'ops' | 'permission';
  owner: string;
  daysRemaining: number;
}

const EXPIRING_SOON: ExpiringFlag[] = [
  { name: 'legacy-payment-gateway', category: 'release', owner: 'team-payments', daysRemaining: 2 },
  { name: 'onboarding-tooltip-v3', category: 'feature', owner: 'maria@example.com', daysRemaining: 5 },
  { name: 'price-rounding-test', category: 'experiment', owner: 'team-growth', daysRemaining: 7 },
  { name: 'log-verbose-mode', category: 'ops', owner: 'infra@example.com', daysRemaining: 10 },
];

interface Deployment {
  id: string;
  service: string;
  version: string;
  strategy: 'canary' | 'blue-green' | 'rolling';
  trafficPct: number;
  healthScore: number;
  status: 'active' | 'pending' | 'rolling-back';
}

const ACTIVE_DEPLOYMENTS: Deployment[] = [
  {
    id: 'dep-1',
    service: 'api-gateway',
    version: 'v2.4.1',
    strategy: 'canary',
    trafficPct: 25,
    healthScore: 99.8,
    status: 'active',
  },
  {
    id: 'dep-2',
    service: 'checkout-service',
    version: 'v1.12.0',
    strategy: 'blue-green',
    trafficPct: 50,
    healthScore: 97.5,
    status: 'active',
  },
  {
    id: 'dep-3',
    service: 'recommendation-engine',
    version: 'v3.1.0-rc2',
    strategy: 'rolling',
    trafficPct: 100,
    healthScore: 95.1,
    status: 'pending',
  },
];

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function strategyBadgeClass(strategy: Deployment['strategy']): string {
  switch (strategy) {
    case 'canary':
      return 'badge badge-experiment';
    case 'blue-green':
      return 'badge badge-release';
    case 'rolling':
      return 'badge badge-ops';
  }
}

function statusBadgeClass(status: Deployment['status']): string {
  switch (status) {
    case 'active':
      return 'badge badge-active';
    case 'pending':
      return 'badge badge-pending';
    case 'rolling-back':
      return 'badge badge-rolling-back';
  }
}

function healthColor(score: number): string {
  if (score >= 99) return 'text-success';
  if (score >= 95) return 'text-warning';
  return 'text-danger';
}

function daysRemainingColor(days: number): string {
  if (days <= 3) return 'text-danger';
  if (days <= 7) return 'text-warning';
  return 'text-secondary';
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const DashboardPage: React.FC = () => {
  return (
    <div>
      {/* Page header */}
      <div className="page-header">
        <h1>Dashboard</h1>
        <p>Overview of your deployment and feature flag activity</p>
      </div>

      {/* Stat cards */}
      <div className="stat-grid">
        <div className="stat-card">
          <div className="stat-label">Total Flags</div>
          <div className="stat-value">{TOTAL_FLAGS}</div>
          <div className="stat-meta">
            {FLAG_CATEGORIES.map((c) => `${c.count} ${c.label.toLowerCase()}`).join(', ')}
          </div>
        </div>

        <div className="stat-card">
          <div className="stat-label">Active Deployments</div>
          <div className="stat-value">3</div>
          <div className="stat-meta">across 3 services</div>
        </div>

        <div className="stat-card">
          <div className="stat-label">Expired Flags</div>
          <div className="stat-value text-warning">5</div>
          <div className="stat-meta">require cleanup</div>
        </div>

        <div className="stat-card">
          <div className="stat-label">Health Score</div>
          <div className="stat-value text-success">98.2%</div>
          <div className="stat-meta">all deployments nominal</div>
        </div>
      </div>

      {/* Two-column layout: Flags by Category + Expiring Soon */}
      <div className="grid-2 mb-4">
        {/* Flags by Category */}
        <div className="card">
          <div className="card-header">
            <span className="card-title">Flags by Category</span>
            <span className="text-xs text-muted">{TOTAL_FLAGS} total</span>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {FLAG_CATEGORIES.map((cat) => (
              <div key={cat.key} className="flex items-center gap-3">
                <span
                  className={`badge badge-${cat.key}`}
                  style={{ minWidth: 90, justifyContent: 'center' }}
                >
                  {cat.label}
                </span>
                <div
                  style={{
                    flex: 1,
                    height: 8,
                    borderRadius: 4,
                    background: 'var(--color-bg)',
                    overflow: 'hidden',
                  }}
                >
                  <div
                    style={{
                      width: `${(cat.count / TOTAL_FLAGS) * 100}%`,
                      height: '100%',
                      borderRadius: 4,
                      background: `var(--color-${
                        cat.key === 'release'
                          ? 'info'
                          : cat.key === 'feature'
                          ? 'purple'
                          : cat.key === 'experiment'
                          ? 'warning'
                          : cat.key === 'ops'
                          ? 'cyan'
                          : 'success'
                      })`,
                      transition: 'width 0.4s ease',
                    }}
                  />
                </div>
                <span className="text-sm" style={{ minWidth: 24, textAlign: 'right' }}>
                  {cat.count}
                </span>
              </div>
            ))}
          </div>
        </div>

        {/* Expiring Soon */}
        <div className="card">
          <div className="card-header">
            <span className="card-title">Expiring Soon</span>
            <span className="text-xs text-muted">{EXPIRING_SOON.length} flags</span>
          </div>
          <div className="table-container">
            <table>
              <thead>
                <tr>
                  <th>Flag</th>
                  <th>Owner</th>
                  <th style={{ textAlign: 'right' }}>Days Left</th>
                </tr>
              </thead>
              <tbody>
                {EXPIRING_SOON.map((flag) => (
                  <tr key={flag.name}>
                    <td>
                      <div className="flex items-center gap-2">
                        <code className="font-mono text-sm">{flag.name}</code>
                        <span className={`badge badge-${flag.category}`}>{flag.category}</span>
                      </div>
                    </td>
                    <td className="text-secondary text-sm">{flag.owner}</td>
                    <td style={{ textAlign: 'right' }}>
                      <span className={daysRemainingColor(flag.daysRemaining)}>
                        {flag.daysRemaining}d
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      {/* Recent Activity */}
      <div className="card mb-4">
        <div className="card-header">
          <span className="card-title">Recent Activity</span>
        </div>
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th>Event</th>
                <th>Actor</th>
                <th style={{ textAlign: 'right' }}>Time</th>
              </tr>
            </thead>
            <tbody>
              {RECENT_ACTIVITY.map((event) => (
                <tr key={event.id}>
                  <td className={event.warning ? 'text-warning' : ''}>{event.description}</td>
                  <td className="text-secondary text-sm">{event.actor}</td>
                  <td
                    className={`text-sm ${event.warning ? 'text-warning' : 'text-muted'}`}
                    style={{ textAlign: 'right', whiteSpace: 'nowrap' }}
                  >
                    {event.timestamp}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Active Deployments */}
      <div className="card">
        <div className="card-header">
          <span className="card-title">Active Deployments</span>
          <span className="text-xs text-muted">{ACTIVE_DEPLOYMENTS.length} running</span>
        </div>
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th>Service</th>
                <th>Version</th>
                <th>Strategy</th>
                <th>Traffic</th>
                <th>Health</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {ACTIVE_DEPLOYMENTS.map((dep) => (
                <tr key={dep.id}>
                  <td style={{ fontWeight: 500 }}>{dep.service}</td>
                  <td>
                    <code className="font-mono text-sm">{dep.version}</code>
                  </td>
                  <td>
                    <span className={strategyBadgeClass(dep.strategy)}>{dep.strategy}</span>
                  </td>
                  <td>{dep.trafficPct}%</td>
                  <td>
                    <span className={healthColor(dep.healthScore)}>
                      {dep.healthScore.toFixed(1)}%
                    </span>
                  </td>
                  <td>
                    <span className={statusBadgeClass(dep.status)}>{dep.status}</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};

export default DashboardPage;

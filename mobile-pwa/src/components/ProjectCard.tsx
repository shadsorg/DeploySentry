import { useState } from 'react';
import type { OrgStatusEnvCell, OrgStatusProjectNode } from '../types';
import { HealthPill } from './HealthPill';
import { EnvChip } from './EnvChip';
import { MonitoringLinkIcon } from './MonitoringLinkIcon';

export function ProjectCard({
  project,
  onEnvTap,
}: {
  project: OrgStatusProjectNode;
  onEnvTap: (cell: OrgStatusEnvCell) => void;
}) {
  const [open, setOpen] = useState(false);
  const appCount = project.applications.length;

  return (
    <div className="m-card m-project-card">
      <button
        type="button"
        className="m-project-card-header"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        <div className="m-project-card-title">
          <span className="m-project-name">{project.project.name}</span>
          <span className="m-project-apps">{appCount} apps</span>
        </div>
        <HealthPill state={project.aggregate_health} />
      </button>

      {open && (
        <ul className="m-app-list">
          {project.applications.map((a) => (
            <li key={a.application.id} className="m-app-row">
              <div className="m-app-row-header">
                <span className="m-app-name">{a.application.name}</span>
                <span className="m-monitor-row">
                  {(a.application.monitoring_links ?? []).map((link) => (
                    <MonitoringLinkIcon key={link.url} link={link} />
                  ))}
                </span>
              </div>
              <div className="m-env-chip-row">
                {a.environments.map((cell) => (
                  <EnvChip key={cell.environment.id} cell={cell} onTap={onEnvTap} />
                ))}
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

import type { OrgEnvironment, FlagEnvironmentState } from '../types';

export function FlagEnvStrip({
  environments,
  states,
}: {
  environments: OrgEnvironment[];
  states: FlagEnvironmentState[];
}) {
  return (
    <div className="m-flag-env-strip" aria-label="Environment states">
      {environments.map((env) => {
        const state = states.find((s) => s.environment_id === env.id);
        const on = state?.enabled === true;
        return (
          <span
            key={env.id}
            className="m-flag-env-pip"
            data-on={on}
            title={`${env.slug}: ${on ? 'on' : 'off'}`}
          >
            {env.slug}
          </span>
        );
      })}
    </div>
  );
}

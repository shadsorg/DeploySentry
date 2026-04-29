import { Link } from 'react-router-dom';
import type { Flag, OrgEnvironment, FlagEnvironmentState } from '../types';
import { CategoryBadge } from './CategoryBadge';
import { FlagEnvStrip } from './FlagEnvStrip';

export function FlagRow({
  flag,
  detailHref,
  environments,
  states,
}: {
  flag: Flag;
  detailHref: string;
  environments: OrgEnvironment[];
  states: FlagEnvironmentState[];
}) {
  return (
    <Link to={detailHref} className="m-card m-flag-row">
      <div className="m-flag-row-head">
        <span className="m-flag-key">{flag.key}</span>
        <CategoryBadge category={flag.category} />
      </div>
      <div className="m-flag-row-name">{flag.name}</div>
      <FlagEnvStrip environments={environments} states={states} />
    </Link>
  );
}

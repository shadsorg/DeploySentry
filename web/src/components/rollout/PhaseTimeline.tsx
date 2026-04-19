import type { Rollout } from '@/types';

export function PhaseTimeline({ rollout }: { rollout: Rollout }) {
  const steps = rollout.strategy_snapshot.steps;
  return (
    <div className="phase-timeline">
      {steps.map((step, idx) => {
        let state: 'done' | 'current' | 'pending';
        if (idx < rollout.current_phase_index) state = 'done';
        else if (idx === rollout.current_phase_index) state = 'current';
        else state = 'pending';
        return (
          <div key={idx} className={`phase-node phase-${state}`}>
            <div className="phase-index">{idx + 1}</div>
            <div className="phase-percent">{step.percent}%</div>
          </div>
        );
      })}
    </div>
  );
}

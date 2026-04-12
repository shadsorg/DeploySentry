type Node = { x: number; label: string };

const DEPLOY: Node[] = [
  { x: 80,  label: 'commit' },
  { x: 240, label: 'build' },
  { x: 400, label: 'deploy' },
  { x: 560, label: 'live (dark)' },
];
const RELEASE: Node[] = [
  { x: 240, label: 'internal' },
  { x: 400, label: '5%' },
  { x: 560, label: '50%' },
  { x: 720, label: '100%' },
];

const TRACK_DEPLOY_Y = 80;
const TRACK_RELEASE_Y = 200;

function Track({
  nodes,
  y,
  startDelay,
  trackId,
}: {
  nodes: Node[];
  y: number;
  startDelay: number;
  trackId: string;
}) {
  return (
    <g>
      {nodes.slice(1).map((node, i) => {
        const prev = nodes[i];
        return (
          <line
            key={`${trackId}-line-${i}`}
            x1={prev.x}
            y1={y}
            x2={node.x}
            y2={y}
            className="flow-rail"
            style={{ animationDelay: `${startDelay + 0.2 + i * 0.4}s` }}
          />
        );
      })}
      {nodes.map((node, i) => (
        <g
          key={`${trackId}-node-${i}`}
          className="flow-node"
          style={{ animationDelay: `${startDelay + i * 0.4}s` }}
        >
          <circle cx={node.x} cy={y} r={7} />
          <text x={node.x} y={y - 18} textAnchor="middle">{node.label}</text>
        </g>
      ))}
    </g>
  );
}

export default function DeployReleaseFlow() {
  return (
    <section className="flow-section">
      <div className="flow-inner">
        <h2 className="section-heading">Deploy. Then release.</h2>
        <p className="section-sub">
          Engineering ships dark. Product opens the gate when they're ready. The two halves are
          decoupled, owned separately, and observable end-to-end.
        </p>
        <div className="flow-diagram-wrap">
          <svg
            className="flow-diagram"
            viewBox="0 0 820 280"
            xmlns="http://www.w3.org/2000/svg"
            role="img"
            aria-label="Deploy track followed by release track, animated"
          >
            <text x={20} y={40} className="flow-track-label">DEPLOY</text>
            <text x={760} y={40} className="flow-owner" textAnchor="end">engineering owns →</text>
            <Track nodes={DEPLOY} y={TRACK_DEPLOY_Y} startDelay={0} trackId="deploy" />

            <line
              x1={20} y1={140} x2={800} y2={140}
              className="flow-separator"
              style={{ animationDelay: '2.4s' }}
            />

            <text x={20} y={170} className="flow-track-label">RELEASE</text>
            <text x={760} y={170} className="flow-owner" textAnchor="end">product / on-call owns →</text>
            <Track nodes={RELEASE} y={TRACK_RELEASE_Y} startDelay={2.8} trackId="release" />
          </svg>
        </div>
      </div>
    </section>
  );
}

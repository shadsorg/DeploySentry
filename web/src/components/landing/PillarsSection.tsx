import { motion } from 'framer-motion';

type Pillar = {
  icon: string;
  label: string;
  title: string;
  body: string;
  tag?: string;
};

const PILLARS: Pillar[] = [
  {
    icon: '⇄',
    label: 'CONTROL',
    title: 'Decoupled deploys & releases',
    body: 'Ship code dark. Release on a flag flip. No rebuild, no rollback drama, no coupling between engineering and product schedules.',
  },
  {
    icon: '◎',
    label: 'METHODOLOGY',
    title: 'Centralized SDK dispatch',
    body: 'Register the functions a flag controls, then call ds.dispatch(flag, ctx) — the SDK picks the right one. Every call site is discoverable from one place.',
    tag: 'Recommended pattern',
  },
  {
    icon: '✻',
    label: 'CLEANUP',
    title: 'LLM-ready flag retirement',
    body: 'Because every flag has a registry entry pointing at its code, an LLM agent can confidently retire a flag and the dead code with it. No archaeology.',
  },
];

const fadeUp = {
  hidden: { opacity: 0, y: 24 },
  show: { opacity: 1, y: 0, transition: { duration: 0.5, ease: 'easeOut' as const } },
};

export default function PillarsSection() {
  return (
    <section className="pillars-section" id="pillars">
      <div className="pillars-inner">
        <h2 className="section-heading">Three controls, one model</h2>
        <p className="section-sub">
          DeploySentry is built around a small, opinionated set of primitives.
        </p>
        <motion.div
          className="pillar-grid"
          initial="hidden"
          whileInView="show"
          viewport={{ once: true, margin: '-100px' }}
          variants={{ show: { transition: { staggerChildren: 0.1 } } }}
        >
          {PILLARS.map((p) => (
            <motion.article key={p.title} className="pillar-card" variants={fadeUp}>
              <div className="pillar-icon" aria-hidden>
                {p.icon}
              </div>
              <div className="pillar-label">{p.label}</div>
              <h3 className="pillar-title">{p.title}</h3>
              <p className="pillar-body">{p.body}</p>
              {p.tag && <div className="pillar-tag">{p.tag}</div>}
            </motion.article>
          ))}
        </motion.div>
      </div>
    </section>
  );
}

import { motion } from 'framer-motion';

const STEPS = [
  { n: '01', title: 'Develop', body: 'Write the new code path behind a flag.' },
  { n: '02', title: 'Ship dark', body: 'Deploy to prod with the flag off.' },
  { n: '03', title: 'Targeted release', body: 'Open the gate for internal, then 5%, then 100%.' },
  { n: '04', title: 'Observe', body: 'Watch metrics and errors per cohort.' },
  { n: '05', title: 'Retire flag', body: 'Mark the flag complete in the registry.' },
  { n: '06', title: 'LLM cleans code', body: 'An agent prunes the dead branch from the source.' },
];

const fadeRight = {
  hidden: { opacity: 0, x: -16 },
  show: { opacity: 1, x: 0, transition: { duration: 0.4, ease: 'easeOut' } },
};

export default function LifecycleStrip() {
  return (
    <section className="lifecycle">
      <div className="lifecycle-inner">
        <h2 className="section-heading">From idea to retirement</h2>
        <p className="section-sub">
          The full lifecycle, in six steps, on one page.
        </p>
        <motion.div
          className="lifecycle-steps"
          initial="hidden"
          whileInView="show"
          viewport={{ once: true, margin: '-80px' }}
          variants={{ show: { transition: { staggerChildren: 0.08 } } }}
        >
          {STEPS.map((s) => (
            <motion.div key={s.n} className="lifecycle-step" variants={fadeRight}>
              <div className="lifecycle-num">{s.n}</div>
              <div className="lifecycle-title">{s.title}</div>
              <div className="lifecycle-body">{s.body}</div>
            </motion.div>
          ))}
        </motion.div>
      </div>
    </section>
  );
}

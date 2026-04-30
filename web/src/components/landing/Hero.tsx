import { motion } from 'framer-motion';
import { Link } from 'react-router-dom';

const fade = {
  hidden: { opacity: 0, y: 12 },
  show: { opacity: 1, y: 0, transition: { duration: 0.5, ease: 'easeOut' as const } },
};

export default function Hero() {
  return (
    <section className="hero">
      <div className="hero-inner">
        <motion.div
          initial="hidden"
          animate="show"
          variants={{ show: { transition: { staggerChildren: 0.06 } } }}
        >
          <motion.div variants={fade} className="hero-eyebrow">
            FEATURE FLAG INFRASTRUCTURE
          </motion.div>
          <motion.h1 variants={fade} className="hero-headline">
            Ship code.
            <br />
            Release features.
            <br />
            <span className="hero-accent">Separately.</span>
          </motion.h1>
          <motion.p variants={fade} className="hero-sub">
            DeploySentry decouples deployment from release. Centralize every flag through the SDK so
            you always know where each one lives — and so an LLM can clean up the dead code when
            it's done.
          </motion.p>
          <motion.div variants={fade} className="hero-actions">
            <Link to="/register" className="btn-primary">
              Get started
            </Link>
            <Link to="/docs" className="btn-secondary">
              Read the docs
            </Link>
          </motion.div>
        </motion.div>
      </div>
    </section>
  );
}

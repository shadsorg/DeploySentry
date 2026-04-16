import { Link } from 'react-router-dom';

export default function CTASection() {
  return (
    <section className="cta-section">
      <div className="cta-inner">
        <h2 className="cta-headline">Stop coupling deploys to releases.</h2>
        <Link to="/register" className="btn-primary cta-button">Get started for free</Link>
        <Link to="/docs" className="cta-secondary">or read the docs →</Link>
      </div>
    </section>
  );
}

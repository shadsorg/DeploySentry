import { Link } from 'react-router-dom';

export default function Footer() {
  return (
    <footer className="landing-footer">
      <div className="landing-footer-inner">
        <div className="landing-footer-col">
          <div className="landing-footer-heading">Product</div>
          <a href="#pillars">Features</a>
          <Link to="/docs/sdks">SDKs</Link>
          <Link to="/docs/cli">CLI</Link>
        </div>
        <div className="landing-footer-col">
          <div className="landing-footer-heading">Docs</div>
          <Link to="/docs/getting-started">Getting started</Link>
          <Link to="/docs/sdks">SDK reference</Link>
          <Link to="/docs/cli">CLI reference</Link>
        </div>
        <div className="landing-footer-col">
          <div className="landing-footer-heading">Project</div>
          <a href="https://github.com/shadsorg/DeploySentry" target="_blank" rel="noreferrer">
            GitHub
          </a>
        </div>
      </div>
      <div className="landing-footer-bottom">
        <span className="site-header-logo">DS</span>
        <span>© DeploySentry · v1.0.0</span>
      </div>
    </footer>
  );
}

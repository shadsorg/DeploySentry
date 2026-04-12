import SiteHeader from '@/components/SiteHeader';

export default function LandingPage() {
  return (
    <div className="landing">
      <SiteHeader variant="landing" />
      <main className="landing-main">
        <section className="landing-placeholder">
          <h1>DeploySentry</h1>
          <p>Landing content coming in subsequent tasks.</p>
        </section>
      </main>
    </div>
  );
}

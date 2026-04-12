import SiteHeader from '@/components/SiteHeader';
import Hero from '@/components/landing/Hero';
import DeployReleaseFlow from '@/components/landing/DeployReleaseFlow';

export default function LandingPage() {
  return (
    <div className="landing">
      <SiteHeader variant="landing" />
      <main className="landing-main">
        <Hero />
        <DeployReleaseFlow />
      </main>
    </div>
  );
}

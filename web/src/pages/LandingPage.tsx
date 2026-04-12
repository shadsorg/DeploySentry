import SiteHeader from '@/components/SiteHeader';
import Hero from '@/components/landing/Hero';
import DeployReleaseFlow from '@/components/landing/DeployReleaseFlow';
import PillarsSection from '@/components/landing/PillarsSection';
import CodeContrast from '@/components/landing/CodeContrast';
import LifecycleStrip from '@/components/landing/LifecycleStrip';

export default function LandingPage() {
  return (
    <div className="landing">
      <SiteHeader variant="landing" />
      <main className="landing-main">
        <Hero />
        <DeployReleaseFlow />
        <PillarsSection />
        <CodeContrast />
        <LifecycleStrip />
      </main>
    </div>
  );
}

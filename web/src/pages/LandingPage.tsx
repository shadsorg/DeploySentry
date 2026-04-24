import SiteHeader from '@/components/SiteHeader';
import Hero from '@/components/landing/Hero';
import DeployReleaseFlow from '@/components/landing/DeployReleaseFlow';
import PillarsSection from '@/components/landing/PillarsSection';
import CodeContrast from '@/components/landing/CodeContrast';
import LifecycleStrip from '@/components/landing/LifecycleStrip';
import CTASection from '@/components/landing/CTASection';
import Footer from '@/components/landing/Footer';

export default function LandingPage() {
  return (
    <div className="landing">
      <SiteHeader variant="landing" size="large" />
      <main className="landing-main">
        <Hero />
        <DeployReleaseFlow />
        <PillarsSection />
        <CodeContrast />
        <LifecycleStrip />
        <CTASection />
      </main>
      <Footer />
    </div>
  );
}

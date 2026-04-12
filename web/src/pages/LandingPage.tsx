import SiteHeader from '@/components/SiteHeader';
import Hero from '@/components/landing/Hero';

export default function LandingPage() {
  return (
    <div className="landing">
      <SiteHeader variant="landing" />
      <main className="landing-main">
        <Hero />
      </main>
    </div>
  );
}

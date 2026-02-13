import { Suspense, lazy, useEffect } from 'react';
import Navigation from './sections/Navigation';
import Hero from './sections/Hero';
import Footer from './sections/Footer';
import DeferredSection from './components/DeferredSection';

const Features = lazy(() => import('./sections/Features'));
const HowItWorks = lazy(() => import('./sections/HowItWorks'));
const DashboardPreview = lazy(() => import('./sections/DashboardPreview'));
const TechStack = lazy(() => import('./sections/TechStack'));
const Installation = lazy(() => import('./sections/Installation'));
const FAQ = lazy(() => import('./sections/FAQ'));
const CTA = lazy(() => import('./sections/CTA'));

interface SectionFallbackProps {
  minHeight: number;
}

function SectionFallback({ minHeight }: SectionFallbackProps) {
  return <div style={{ minHeight }} aria-hidden="true" />;
}

function App() {
  // Smooth scroll behavior
  useEffect(() => {
    // Check for reduced motion preference
    const prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    
    if (prefersReducedMotion) {
      document.documentElement.style.scrollBehavior = 'auto';
    }

    // Add noise overlay class to body
    document.body.classList.add('noise-overlay');

    return () => {
      document.body.classList.remove('noise-overlay');
    };
  }, []);

  return (
    <div className="relative min-h-screen bg-[#0e0e0e] text-white overflow-x-hidden">
      {/* Navigation */}
      <Navigation />

      {/* Main Content */}
      <main>
        {/* Hero Section */}
        <Hero />

        {/* Features Section */}
        <DeferredSection id="features" minHeight={760}>
          <Suspense fallback={<SectionFallback minHeight={760} />}>
            <Features />
          </Suspense>
        </DeferredSection>

        {/* How It Works Section */}
        <DeferredSection id="how-it-works" minHeight={640}>
          <Suspense fallback={<SectionFallback minHeight={640} />}>
            <HowItWorks />
          </Suspense>
        </DeferredSection>

        {/* Dashboard Preview Section */}
        <DeferredSection id="dashboard" minHeight={820}>
          <Suspense fallback={<SectionFallback minHeight={820} />}>
            <DashboardPreview />
          </Suspense>
        </DeferredSection>

        {/* Tech Stack Section */}
        <DeferredSection minHeight={700}>
          <Suspense fallback={<SectionFallback minHeight={700} />}>
            <TechStack />
          </Suspense>
        </DeferredSection>

        {/* Installation Section */}
        <DeferredSection id="install" minHeight={780}>
          <Suspense fallback={<SectionFallback minHeight={780} />}>
            <Installation />
          </Suspense>
        </DeferredSection>

        {/* CTA Section */}
        <DeferredSection minHeight={380}>
          <Suspense fallback={<SectionFallback minHeight={380} />}>
            <CTA />
          </Suspense>
        </DeferredSection>

        {/* FAQ Section */}
        <DeferredSection id="faq" minHeight={820}>
          <Suspense fallback={<SectionFallback minHeight={820} />}>
            <FAQ />
          </Suspense>
        </DeferredSection>
      </main>

      {/* Footer */}
      <Footer />
    </div>
  );
}

export default App;

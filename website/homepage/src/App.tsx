import { useEffect } from 'react';
import Navigation from './sections/Navigation';
import Hero from './sections/Hero';
import Features from './sections/Features';
import HowItWorks from './sections/HowItWorks';
import DashboardPreview from './sections/DashboardPreview';
import TechStack from './sections/TechStack';
import Installation from './sections/Installation';
import CTA from './sections/CTA';
import Footer from './sections/Footer';

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
        <section id="features">
          <Features />
        </section>

        {/* How It Works Section */}
        <section id="how-it-works">
          <HowItWorks />
        </section>

        {/* Dashboard Preview Section */}
        <section id="dashboard">
          <DashboardPreview />
        </section>

        {/* Tech Stack Section */}
        <TechStack />

        {/* Installation Section */}
        <section id="install">
          <Installation />
        </section>

        {/* CTA Section */}
        <CTA />
      </main>

      {/* Footer */}
      <Footer />
    </div>
  );
}

export default App;

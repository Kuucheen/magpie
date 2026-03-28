import { useEffect, useId, useRef } from 'react';
import { ArrowRight, BookOpen, ChevronDown } from 'lucide-react';
import gsap from 'gsap';

import { Button } from '@/components/ui/button';

export default function Hero() {
  const maskId = useId().replace(/:/g, '');
  const heroRef = useRef<HTMLElement>(null);
  const titleRef = useRef<HTMLDivElement>(null);
  const subtitleRef = useRef<HTMLParagraphElement>(null);
  const ctaRef = useRef<HTMLDivElement>(null);

  const scrollToInstall = () => {
    const element = document.querySelector('#install');
    if (element) {
      element.scrollIntoView({ behavior: 'smooth' });
    }
  };

  const scrollToFeatures = () => {
    const element = document.querySelector('#features');
    if (element) {
      element.scrollIntoView({ behavior: 'smooth' });
    }
  };

  useEffect(() => {
    const ctx = gsap.context(() => {
      gsap.fromTo(
        titleRef.current,
        { opacity: 0, y: 48, filter: 'blur(18px)' },
        { opacity: 1, y: 0, filter: 'blur(0px)', duration: 1, delay: 0.2, ease: 'expo.out' }
      );

      gsap.fromTo(
        subtitleRef.current,
        { opacity: 0, y: 24, filter: 'blur(10px)' },
        { opacity: 1, y: 0, filter: 'blur(0px)', duration: 0.85, delay: 0.4, ease: 'power3.out' }
      );

      gsap.fromTo(
        ctaRef.current,
        { opacity: 0, y: 22, scale: 0.96 },
        { opacity: 1, y: 0, scale: 1, duration: 0.8, delay: 0.52, ease: 'power3.out' }
      );
    }, heroRef);

    return () => ctx.revert();
  }, []);

  return (
    <section
      ref={heroRef}
      className="hero-landing relative min-h-screen w-full overflow-hidden bg-transparent"
    >
      <div className="fixed inset-0 z-0 overflow-hidden pointer-events-none">
        <div className="hero-landing-image absolute inset-0 z-0" />
      </div>

      <div
        ref={titleRef}
        className="hero-landing-cutout pointer-events-none absolute inset-0 z-[1] opacity-0"
      >
        <svg
          className="hero-landing-cutout-svg"
          viewBox="0 0 1600 900"
          aria-hidden="true"
          preserveAspectRatio="xMidYMid slice"
        >
          <defs>
            <mask id={maskId}>
              <rect width="1600" height="900" fill="white" />
              <rect x="120" y="96" width="1360" height="708" rx="32" fill="black" />
            </mask>

            <mask id={`${maskId}-outline`}>
              <rect width="1600" height="900" fill="white" />
              <text
                x="50%"
                y="38%"
                fill="black"
                fontFamily="Outfit, sans-serif"
                fontSize="232"
                fontWeight="700"
                letterSpacing="56"
                textAnchor="middle"
                dominantBaseline="middle"
              >
                MAGPIE
              </text>
            </mask>

            <linearGradient id={`${maskId}-shade`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor="#050505" stopOpacity="0.62" />
              <stop offset="100%" stopColor="#050505" stopOpacity="0.84" />
            </linearGradient>

            <radialGradient id={`${maskId}-glow`} cx="50%" cy="18%" r="44%">
              <stop offset="0%" stopColor="#ffffff" stopOpacity="0.06" />
              <stop offset="100%" stopColor="#ffffff" stopOpacity="0" />
            </radialGradient>
          </defs>

          <rect width="1600" height="900" fill={`url(#${maskId}-shade)`} mask={`url(#${maskId})`} />
          <rect width="1600" height="900" fill={`url(#${maskId}-glow)`} mask={`url(#${maskId})`} />
          <text
            x="50%"
            y="38%"
            fill="rgba(255,255,255,0.84)"
            letterSpacing="56"
            textAnchor="middle"
            dominantBaseline="middle"
            fontFamily="Outfit, sans-serif"
            fontSize="232"
            fontWeight="700"
          >
            MAGPIE
          </text>
          <text
            x="50%"
            y="38%"
            fill="none"
            stroke="rgba(255,255,255,0.16)"
            strokeWidth="4"
            strokeLinejoin="round"
            letterSpacing="56"
            textAnchor="middle"
            dominantBaseline="middle"
            fontFamily="Outfit, sans-serif"
            fontSize="232"
            fontWeight="700"
            mask={`url(#${maskId}-outline)`}
          >
            MAGPIE
          </text>
        </svg>
      </div>

      <div className="relative z-10 mx-auto flex min-h-screen w-full max-w-7xl items-center justify-center px-4 pb-16 pt-24 sm:px-6 lg:px-8">
        <div className="relative flex w-full max-w-5xl flex-col items-center text-center">
          <div className="relative z-10 flex w-full flex-col items-center gap-5 px-6 pb-10 pt-[11rem] sm:gap-7 sm:px-10 sm:pb-12 sm:pt-[14rem] lg:px-14 lg:pb-16 lg:pt-[17rem]">
            <h1 className="sr-only">MAGPIE</h1>

            <p
              ref={subtitleRef}
              className="max-w-2xl px-2 text-sm font-medium tracking-[0.08em] text-white/72 opacity-0 sm:text-base"
            >
              A self-hostable multi-user proxy manager for teams, rotators, and scraping workloads.
            </p>

            <div
              ref={ctaRef}
              className="flex flex-col items-center gap-3 pt-2 opacity-0 sm:flex-row"
            >
              <Button
                size="lg"
                className="min-w-[12rem] bg-white text-[#101010] transition-all duration-300 hover:scale-[1.02] hover:bg-white/92"
                onClick={scrollToInstall}
              >
                Get Started
                <ArrowRight className="ml-2 h-4 w-4" />
              </Button>

              <Button
                size="lg"
                variant="outline"
                className="min-w-[12rem] border-white/16 bg-black/18 text-white backdrop-blur-md transition-all duration-300 hover:border-white/28 hover:bg-white/10"
                asChild
              >
                <a href="/docs">
                  <BookOpen className="mr-2 h-4 w-4" />
                  Read the Docs
                </a>
              </Button>
            </div>
          </div>
        </div>
      </div>

      <button
        type="button"
        onClick={scrollToFeatures}
        className="hero-scroll-cue absolute bottom-6 left-1/2 z-20 flex h-12 w-12 -translate-x-1/2 items-center justify-center rounded-full border border-white/10 bg-black/22 text-white/72 backdrop-blur-sm transition-[background-color,border-color,color,box-shadow] duration-300 hover:border-white/24 hover:bg-black/32 hover:text-white hover:shadow-[0_14px_32px_rgba(0,0,0,0.28)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/30 sm:bottom-8"
        aria-label="Scroll to features"
      >
        <ChevronDown className="hero-scroll-cue-icon h-5 w-5 transition-transform duration-300" />
      </button>
    </section>
  );
}

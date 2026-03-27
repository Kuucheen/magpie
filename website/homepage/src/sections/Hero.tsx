import { useEffect, useRef } from 'react';
import { ArrowRight, BookOpen } from 'lucide-react';
import gsap from 'gsap';

import { Button } from '@/components/ui/button';

export default function Hero() {
  const heroRef = useRef<HTMLElement>(null);
  const logoRef = useRef<HTMLImageElement>(null);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const subtitleRef = useRef<HTMLParagraphElement>(null);
  const ctaRef = useRef<HTMLDivElement>(null);

  const scrollToInstall = () => {
    const element = document.querySelector('#install');
    if (element) {
      element.scrollIntoView({ behavior: 'smooth' });
    }
  };

  useEffect(() => {
    const ctx = gsap.context(() => {
      gsap.fromTo(
        logoRef.current,
        {
          opacity: 0,
          scale: 0.84,
          rotate: -6,
          filter: 'blur(18px)',
          xPercent: -50,
          yPercent: -50,
        },
        {
          opacity: 0.18,
          scale: 1,
          rotate: 0,
          filter: 'blur(0px)',
          xPercent: -50,
          yPercent: -50,
          duration: 1.2,
          delay: 0.15,
          ease: 'expo.out',
        }
      );

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
      className="hero-landing relative flex min-h-screen w-full items-center justify-center overflow-hidden bg-[#050505]"
    >
      <div className="hero-landing-image absolute inset-0 z-0" />
      <div className="hero-landing-overlay absolute inset-0 z-[1]" />

      <div className="relative z-10 mx-auto flex min-h-screen w-full max-w-7xl items-center justify-center px-4 pb-16 pt-24 sm:px-6 lg:px-8">
        <div className="relative flex max-w-5xl flex-col items-center text-center">
          <img
            ref={logoRef}
            src="/magpie-light-green.svg"
            alt=""
            aria-hidden="true"
            className="hero-landing-logo pointer-events-none absolute left-1/2 top-1/2 z-0 w-[18rem] max-w-none opacity-0 sm:w-[24rem] lg:w-[30rem]"
          />

          <div className="relative z-10 flex flex-col items-center gap-5 sm:gap-7">
            <h1
              ref={titleRef}
              className="hero-landing-wordmark text-[clamp(3.6rem,16vw,11.5rem)] font-bold uppercase leading-none tracking-[0.22em] opacity-0 sm:tracking-[0.28em]"
            >
              MAGPIE
            </h1>

            <p
              ref={subtitleRef}
              className="max-w-2xl px-2 text-sm font-medium tracking-[0.08em] text-white/68 opacity-0 sm:text-base"
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
    </section>
  );
}

import { useEffect, useRef, useState } from 'react';
import { Github, Menu, X } from 'lucide-react';

import { Button } from '@/components/ui/button';

const REVEAL_ALL_DEFERRED_SECTIONS_EVENT = 'magpie:reveal-deferred-sections';
const navLinks = [
  { label: 'Features', href: '#features' },
  { label: 'How It Works', href: '#how-it-works' },
  { label: 'Dashboard', href: '#dashboard' },
  { label: 'Install', href: '#install' },
];

export default function Navigation() {
  const navRef = useRef<HTMLElement>(null);
  const [isScrolled, setIsScrolled] = useState(false);
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const isFloating = isScrolled || isMobileMenuOpen;

  useEffect(() => {
    const handleScroll = () => {
      setIsScrolled(window.scrollY > 72);
    };

    window.addEventListener('scroll', handleScroll, { passive: true });
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  const scrollToSection = (href: string) => {
    window.dispatchEvent(new Event(REVEAL_ALL_DEFERRED_SECTIONS_EVENT));
    setIsMobileMenuOpen(false);

    const element = document.querySelector(href);
    if (!element) return;

    const scrollToTarget = (behavior: ScrollBehavior) => {
      const navRect = navRef.current?.getBoundingClientRect();
      const navbarOffset = navRect ? navRect.bottom - 50 : isFloating ? 0 : 0;
      const top = Math.max(0, window.scrollY + element.getBoundingClientRect().top - navbarOffset);
      window.scrollTo({ top, behavior });
    };

    scrollToTarget('smooth');

    window.setTimeout(() => scrollToTarget('auto'), 250);
    window.setTimeout(() => scrollToTarget('auto'), 700);
  };

  return (
    <>
      <div
        className={`fixed inset-x-0 top-0 z-50 transition-all duration-500 ${
          isFloating ? 'px-4 pb-3 pt-4 sm:px-6' : 'px-4 pb-0 pt-0 sm:px-6'
        }`}
      >
        <nav
          ref={navRef}
          data-site-nav="true"
          className={`mx-auto flex w-full items-center justify-between gap-4 border bg-[#090a09]/78 px-4 py-3 backdrop-blur-xl transition-all duration-500 sm:px-5 ${
            isFloating
              ? 'max-w-6xl rounded-full border-white/10 shadow-[0_20px_60px_rgba(0,0,0,0.26)]'
              : 'max-w-6xl rounded-b-[28px] border-white/8 border-t-0 shadow-none'
          }`}
        >
          <a
            href="#"
            className="flex items-center gap-3"
            onClick={(event) => {
              event.preventDefault();
              window.scrollTo({ top: 0, behavior: 'smooth' });
            }}
          >
            <div className="flex h-10 w-10 items-center justify-center rounded-full border border-white/10 bg-white/4">
              <img src="/magpie-light-green.svg" alt="Magpie" className="h-6 w-6 object-contain" />
            </div>
            <div className="hidden sm:block">
              <div className="text-sm font-semibold tracking-[0.2em] text-white">MAGPIE</div>
            </div>
          </a>

          <div className="hidden items-center gap-1 md:flex">
            {navLinks.map((link) => (
              <button
                key={link.label}
                onClick={() => scrollToSection(link.href)}
                className="rounded-full px-4 py-2 text-sm text-white/58 transition-colors hover:bg-white/6 hover:text-white"
              >
                {link.label}
              </button>
            ))}
          </div>

          <div className="hidden items-center gap-3 md:flex">
            <Button
              variant="outline"
              size="sm"
              className="border-white/10 bg-white/4 text-white hover:border-white/16 hover:bg-white/8"
              asChild
            >
              <a href="https://github.com/Kuucheen/magpie" target="_blank" rel="noopener noreferrer">
                <Github className="mr-2 h-4 w-4" />
                GitHub
              </a>
            </Button>
            <Button
              size="sm"
              className="bg-white text-[#101010] hover:bg-white/92"
              onClick={() => scrollToSection('#install')}
            >
              Get Started
            </Button>
          </div>

          <button
            className="flex h-10 w-10 items-center justify-center rounded-full border border-white/10 bg-white/4 text-white md:hidden"
            onClick={() => setIsMobileMenuOpen((value) => !value)}
            aria-label={isMobileMenuOpen ? 'Close navigation menu' : 'Open navigation menu'}
          >
            {isMobileMenuOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
          </button>
        </nav>
      </div>

      <div
        className={`fixed inset-0 z-40 md:hidden ${
          isMobileMenuOpen ? 'pointer-events-auto' : 'pointer-events-none'
        }`}
      >
        <div
          className={`absolute inset-0 bg-black/68 backdrop-blur-sm transition-opacity duration-300 ${
            isMobileMenuOpen ? 'opacity-100' : 'opacity-0'
          }`}
          onClick={() => setIsMobileMenuOpen(false)}
        />

        <div
          className={`absolute left-4 right-4 top-24 rounded-[28px] border border-white/10 bg-[#090a09]/92 p-5 shadow-[0_24px_80px_rgba(0,0,0,0.34)] backdrop-blur-xl transition-[transform,opacity] duration-300 ${
            isMobileMenuOpen
              ? 'translate-y-0 scale-100 opacity-100'
              : '-translate-y-3 scale-[0.98] opacity-0'
          }`}
        >
          <div className="space-y-2">
            {navLinks.map((link) => (
              <button
                key={link.label}
                onClick={() => scrollToSection(link.href)}
                className="block w-full rounded-2xl px-4 py-3 text-left text-base text-white/64 transition-colors hover:bg-white/6 hover:text-white"
              >
                {link.label}
              </button>
            ))}
          </div>

          <div className="mt-5 space-y-3 border-t border-white/8 pt-5">
            <Button
              variant="outline"
              className="w-full border-white/10 bg-white/4 text-white hover:border-white/16 hover:bg-white/8"
              asChild
            >
              <a href="https://github.com/Kuucheen/magpie" target="_blank" rel="noopener noreferrer">
                <Github className="mr-2 h-4 w-4" />
                GitHub
              </a>
            </Button>
            <Button className="w-full bg-white text-[#101010] hover:bg-white/92" onClick={() => scrollToSection('#install')}>
              Get Started
            </Button>
          </div>
        </div>
      </div>
    </>
  );
}

import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Github, Menu, X } from 'lucide-react';

const navLinks = [
  { label: 'Features', href: '#features' },
  { label: 'How It Works', href: '#how-it-works' },
  { label: 'Dashboard', href: '#dashboard' },
  { label: 'Install', href: '#install' },
];

export default function Navigation() {
  const [isScrolled, setIsScrolled] = useState(false);
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

  useEffect(() => {
    const handleScroll = () => {
      setIsScrolled(window.scrollY > 100);
    };

    window.addEventListener('scroll', handleScroll, { passive: true });
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  const scrollToSection = (href: string) => {
    const element = document.querySelector(href);
    if (element) {
      element.scrollIntoView({ behavior: 'smooth' });
    }
    setIsMobileMenuOpen(false);
  };

  return (
    <>
      <nav 
        className={`fixed top-0 left-0 right-0 z-50 transition-all duration-500 ${
          isScrolled 
            ? 'bg-[#0e0e0e]/90 backdrop-blur-lg border-b border-[#1a1a1a]' 
            : 'bg-transparent'
        }`}
      >
        <div className="container mx-auto px-4 sm:px-6 lg:px-8 xl:px-12">
          <div className="flex items-center justify-between h-16">
            {/* Logo */}
            <a 
              href="#" 
              className="flex items-center gap-2 group"
              onClick={(e) => {
                e.preventDefault();
                window.scrollTo({ top: 0, behavior: 'smooth' });
              }}
            >
              <div className="w-10 h-10 flex items-center justify-center group-hover:shadow-lg group-hover:shadow-[#3fa37a]/30 transition-shadow duration-300">
                <img 
                  src="/magpie-light-green.svg" 
                  alt="Magpie" 
                  className="w-7 h-7 object-contain"
                />
              </div>
              <span className="text-lg font-semibold text-white">Magpie</span>
            </a>

            {/* Desktop Navigation */}
            <div className="hidden md:flex items-center gap-1">
              {navLinks.map((link) => (
                <button
                  key={link.label}
                  onClick={() => scrollToSection(link.href)}
                  className="px-4 py-2 text-sm text-[#a0a0a0] hover:text-white transition-colors duration-300"
                >
                  {link.label}
                </button>
              ))}
            </div>

            {/* CTA */}
            <div className="hidden md:flex items-center gap-3">
              <Button
                variant="outline"
                size="sm"
                className="border-[#2a2a2a] bg-transparent text-white hover:bg-[#1a1a1a] hover:border-[#3fa37a]/50"
                asChild
              >
                <a 
                  href="https://github.com/Kuucheen/magpie"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <Github className="w-4 h-4 mr-2" />
                  GitHub
                </a>
              </Button>
              <Button
                size="sm"
                className="bg-[#3fa37a] hover:bg-[#44b381] text-white"
                onClick={() => scrollToSection('#install')}
              >
                Get Started
              </Button>
            </div>

            {/* Mobile Menu Button */}
            <button
              className="md:hidden p-2 text-white"
              onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
            >
              {isMobileMenuOpen ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
            </button>
          </div>
        </div>
      </nav>

      {/* Mobile Menu */}
      <div 
        className={`fixed inset-0 z-40 md:hidden transition-all duration-300 ${
          isMobileMenuOpen ? 'opacity-100 pointer-events-auto' : 'opacity-0 pointer-events-none'
        }`}
      >
        {/* Backdrop */}
        <div 
          className="absolute inset-0 bg-black/80 backdrop-blur-sm"
          onClick={() => setIsMobileMenuOpen(false)}
        />
        
        {/* Menu */}
        <div 
          className={`absolute top-16 left-0 right-0 bg-[#151515] border-b border-[#2a2a2a] transition-transform duration-300 ${
            isMobileMenuOpen ? 'translate-y-0' : '-translate-y-full'
          }`}
        >
          <div className="container mx-auto px-4 py-6 space-y-4">
            {navLinks.map((link) => (
              <button
                key={link.label}
                onClick={() => scrollToSection(link.href)}
                className="block w-full text-left px-4 py-3 text-lg text-[#a0a0a0] hover:text-white hover:bg-[#1a1a1a] rounded-lg transition-colors duration-300"
              >
                {link.label}
              </button>
            ))}
            
            <div className="pt-4 border-t border-[#2a2a2a] space-y-3">
              <Button
                variant="outline"
                className="w-full border-[#2a2a2a] bg-transparent text-white hover:bg-[#1a1a1a]"
                asChild
              >
                <a 
                  href="https://github.com/Kuucheen/magpie"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <Github className="w-4 h-4 mr-2" />
                  GitHub
                </a>
              </Button>
              <Button
                className="w-full bg-[#3fa37a] hover:bg-[#44b381] text-white"
                onClick={() => scrollToSection('#install')}
              >
                Get Started
              </Button>
            </div>
          </div>
        </div>
      </div>
    </>
  );
}

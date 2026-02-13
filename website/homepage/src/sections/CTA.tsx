import { useEffect, useRef } from 'react';
import { Button } from '@/components/ui/button';
import { BookOpen, Github, Heart } from 'lucide-react';
import gsap from 'gsap';
import { ScrollTrigger } from 'gsap/ScrollTrigger';

gsap.registerPlugin(ScrollTrigger);

export default function CTA() {
  const sectionRef = useRef<HTMLDivElement>(null);
  const contentRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const triggers: ScrollTrigger[] = [];

    gsap.fromTo(contentRef.current,
      { opacity: 0, y: 50, scale: 0.95 },
      {
        opacity: 1,
        y: 0,
        scale: 1,
        duration: 1,
        ease: 'expo.out',
        scrollTrigger: {
          trigger: sectionRef.current,
          start: 'top 75%',
          toggleActions: 'play none none none',
          onEnter: (self) => triggers.push(self),
        },
      }
    );

    return () => {
      triggers.forEach(t => t.kill());
    };
  }, []);

  return (
    <section 
      ref={sectionRef}
      className="relative w-full py-24 lg:py-32 bg-[#0e0e0e] overflow-hidden"
    >
      {/* Background effects */}
      <div className="absolute inset-0">
        <div 
          className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] opacity-30"
          style={{
            background: 'radial-gradient(circle, rgba(63, 163, 122, 0.2) 0%, transparent 70%)',
          }}
        />
      </div>

      <div className="relative z-10 container mx-auto px-4 sm:px-6 lg:px-8 xl:px-12">
        <div 
          ref={contentRef}
          className="max-w-3xl mx-auto text-center space-y-8"
        >
          {/* Headline */}
          <h2 className="text-3xl sm:text-4xl lg:text-5xl font-bold text-white leading-tight">
            Ready to Take Control of Your Proxies?
          </h2>

          {/* Subheadline */}
          <p className="text-lg text-[#a0a0a0] max-w-xl mx-auto">
            Join the community of teams and individuals who trust Magpie for their proxy management needs. Free, open source, and self-hosted.
          </p>

          {/* CTAs */}
          <div className="flex flex-col sm:flex-row gap-4 justify-center">
            <Button 
              size="lg" 
              className="bg-[#3fa37a] hover:bg-[#44b381] text-white px-8 py-6 text-base font-medium transition-all duration-300 hover:scale-105 hover:shadow-lg hover:shadow-[#3fa37a]/25"
              asChild
            >
              <a href="https://github.com/Kuucheen/magpie" target="_blank" rel="noopener noreferrer">
                <Github className="w-5 h-5 mr-2" />
                Star on GitHub
              </a>
            </Button>
            <Button 
              size="lg" 
              variant="outline" 
              className="border-[#2a2a2a] bg-[#1a1a1a]/50 text-white hover:bg-[#1a1a1a] hover:border-[#3fa37a]/50 px-8 py-6 text-base font-medium transition-all duration-300"
              asChild
            >
              <a href="/docs">
                <BookOpen className="w-5 h-5 mr-2" />
                Read the Docs
              </a>
            </Button>
          </div>

          {/* Stats */}
          <div className="flex flex-wrap justify-center gap-8 pt-8">
            <div className="text-center">
              <div className="text-3xl font-bold text-[#3fa37a]">3+</div>
              <div className="text-sm text-[#a0a0a0]">Stars</div>
            </div>
            <div className="text-center">
              <div className="text-3xl font-bold text-[#3fa37a]">4</div>
              <div className="text-sm text-[#a0a0a0]">Forks</div>
            </div>
            <div className="text-center">
              <div className="text-3xl font-bold text-[#3fa37a]">AGPL-3.0</div>
              <div className="text-sm text-[#a0a0a0]">License</div>
            </div>
          </div>

          {/* Made with love */}
          <div className="flex items-center justify-center gap-2 text-sm text-[#a0a0a0]">
            <span>Made with</span>
            <Heart className="w-4 h-4 text-red-500 fill-red-500 animate-pulse" />
            <span>by the open source community</span>
          </div>
        </div>
      </div>
    </section>
  );
}

import { useEffect, useRef } from 'react';
import gsap from 'gsap';
import { ScrollTrigger } from 'gsap/ScrollTrigger';
import { Eye, BarChart3, MapPin, Clock } from 'lucide-react';

gsap.registerPlugin(ScrollTrigger);

export default function DashboardPreview() {
  const sectionRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const imageRef = useRef<HTMLImageElement>(null);
  const statsRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const triggers: ScrollTrigger[] = [];

    // Clip-path expand animation
    gsap.fromTo(containerRef.current,
      { 
        clipPath: 'inset(40% 10% 40% 10% round 24px)',
        opacity: 0.5,
      },
      {
        clipPath: 'inset(0% 0% 0% 0% round 16px)',
        opacity: 1,
        duration: 1.2,
        ease: 'expo.out',
        scrollTrigger: {
          trigger: sectionRef.current,
          start: 'top 70%',
          toggleActions: 'play none none none',
          onEnter: (self) => triggers.push(self),
        },
      }
    );

    // Image scale animation
    gsap.fromTo(imageRef.current,
      { scale: 1.2 },
      {
        scale: 1,
        duration: 1.5,
        ease: 'power2.out',
        scrollTrigger: {
          trigger: sectionRef.current,
          start: 'top 70%',
          toggleActions: 'play none none none',
          onEnter: (self) => triggers.push(self),
        },
      }
    );

    // Stats cards animation
    const statCards = statsRef.current?.querySelectorAll('.stat-card');
    if (statCards) {
      gsap.fromTo(statCards,
        { opacity: 0, y: 30, scale: 0.9 },
        {
          opacity: 1,
          y: 0,
          scale: 1,
          duration: 0.6,
          stagger: 0.1,
          ease: 'expo.out',
          scrollTrigger: {
            trigger: statsRef.current,
            start: 'top 85%',
            toggleActions: 'play none none none',
            onEnter: (self) => triggers.push(self),
          },
        }
      );
    }

    // Parallax effect on scroll
    gsap.to(imageRef.current, {
      y: -50,
      ease: 'none',
      scrollTrigger: {
        trigger: sectionRef.current,
        start: 'top bottom',
        end: 'bottom top',
        scrub: 1,
        onEnter: (self) => triggers.push(self),
      },
    });

    return () => {
      triggers.forEach(t => t.kill());
    };
  }, []);

  return (
    <section 
      ref={sectionRef}
      className="relative w-full py-24 lg:py-32 bg-[#0e0e0e] overflow-hidden"
    >
      {/* Background glow */}
      <div className="absolute inset-0">
        <div 
          className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[1200px] h-[600px] opacity-20"
          style={{
            background: 'radial-gradient(ellipse, rgba(63, 163, 122, 0.3) 0%, transparent 70%)',
          }}
        />
      </div>

      <div className="relative z-10 container mx-auto px-4 sm:px-6 lg:px-8 xl:px-12">
        {/* Section Header */}
        <div className="text-center mb-12 space-y-4">
          <h2 className="text-3xl sm:text-4xl lg:text-5xl font-bold text-white">
            Beautiful Dashboard
          </h2>
          <p className="text-lg text-[#a0a0a0] max-w-2xl mx-auto">
            Monitor your proxy pool with real-time statistics, geographic distribution, and health metrics.
          </p>
        </div>

        {/* Dashboard Preview */}
        <div 
          ref={containerRef}
          className="relative rounded-2xl overflow-hidden border border-[#2a2a2a] shadow-2xl"
        >
          {/* Glow effect */}
          <div className="absolute -inset-1 bg-gradient-to-r from-[#3fa37a]/20 via-[#44b381]/10 to-[#3fa37a]/20 rounded-2xl blur-xl opacity-50" />
          
          {/* Image container */}
          <div className="relative overflow-hidden bg-[#151515]">
            <img 
              ref={imageRef}
              src="/dashboard-full.png" 
              alt="Magpie Dashboard Full Preview"
              className="w-full h-auto"
              loading="lazy"
            />
            
            {/* Bottom gradient overlay */}
            <div className="absolute inset-x-0 bottom-0 h-32 bg-gradient-to-t from-[#151515] to-transparent" />
          </div>
        </div>

        {/* Stats Row */}
        <div 
          ref={statsRef}
          className="grid grid-cols-2 lg:grid-cols-4 gap-4 mt-8"
        >
          <div className="stat-card bg-[#1a1a1a] border border-[#2a2a2a] rounded-xl p-4 flex items-center gap-4 hover:border-[#3fa37a]/50 transition-colors duration-300">
            <div className="w-12 h-12 rounded-lg bg-[#3fa37a]/10 flex items-center justify-center text-[#3fa37a]">
              <Eye className="w-6 h-6" />
            </div>
            <div>
              <div className="text-2xl font-bold text-white">Real-time</div>
              <div className="text-sm text-[#a0a0a0]">Monitoring</div>
            </div>
          </div>

          <div className="stat-card bg-[#1a1a1a] border border-[#2a2a2a] rounded-xl p-4 flex items-center gap-4 hover:border-[#3fa37a]/50 transition-colors duration-300">
            <div className="w-12 h-12 rounded-lg bg-[#44b381]/10 flex items-center justify-center text-[#44b381]">
              <BarChart3 className="w-6 h-6" />
            </div>
            <div>
              <div className="text-2xl font-bold text-white">Detailed</div>
              <div className="text-sm text-[#a0a0a0]">Analytics</div>
            </div>
          </div>

          <div className="stat-card bg-[#1a1a1a] border border-[#2a2a2a] rounded-xl p-4 flex items-center gap-4 hover:border-[#3fa37a]/50 transition-colors duration-300">
            <div className="w-12 h-12 rounded-lg bg-[#3fa37a]/10 flex items-center justify-center text-[#3fa37a]">
              <MapPin className="w-6 h-6" />
            </div>
            <div>
              <div className="text-2xl font-bold text-white">Global</div>
              <div className="text-sm text-[#a0a0a0]">Coverage</div>
            </div>
          </div>

          <div className="stat-card bg-[#1a1a1a] border border-[#2a2a2a] rounded-xl p-4 flex items-center gap-4 hover:border-[#3fa37a]/50 transition-colors duration-300">
            <div className="w-12 h-12 rounded-lg bg-[#44b381]/10 flex items-center justify-center text-[#44b381]">
              <Clock className="w-6 h-6" />
            </div>
            <div>
              <div className="text-2xl font-bold text-white">24/7</div>
              <div className="text-sm text-[#a0a0a0]">Health Checks</div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

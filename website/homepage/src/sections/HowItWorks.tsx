import { useEffect, useRef } from 'react';
import { Globe, CheckCircle, TrendingUp, RefreshCw } from 'lucide-react';
import gsap from 'gsap';
import { ScrollTrigger } from 'gsap/ScrollTrigger';

gsap.registerPlugin(ScrollTrigger);

const steps = [
  {
    icon: <Globe className="w-8 h-8" />,
    title: 'Scrape',
    description: 'Collect proxies from multiple public sources automatically. Magpie continuously discovers new proxies from configured sources.',
    color: '#3fa37a',
  },
  {
    icon: <CheckCircle className="w-8 h-8" />,
    title: 'Check',
    description: 'Verify proxy health, anonymity level, and response time. Dead proxies are automatically filtered out.',
    color: '#44b381',
  },
  {
    icon: <TrendingUp className="w-8 h-8" />,
    title: 'Score',
    description: 'Assign reputation scores based on uptime, latency, and historical performance. Only the best proxies make the cut.',
    color: '#3fa37a',
  },
  {
    icon: <RefreshCw className="w-8 h-8" />,
    title: 'Rotate',
    description: 'Use healthy proxies in your applications through rotating endpoints. Distribute traffic across your best performers.',
    color: '#44b381',
  },
];

export default function HowItWorks() {
  const sectionRef = useRef<HTMLDivElement>(null);
  const titleRef = useRef<HTMLDivElement>(null);
  const pathRef = useRef<SVGPathElement>(null);
  const stepsRef = useRef<(HTMLDivElement | null)[]>([]);

  useEffect(() => {
    const triggers: ScrollTrigger[] = [];

    // Title animation
    gsap.fromTo(titleRef.current,
      { opacity: 0, y: 40 },
      {
        opacity: 1,
        y: 0,
        duration: 0.8,
        ease: 'expo.out',
        scrollTrigger: {
          trigger: titleRef.current,
          start: 'top 85%',
          toggleActions: 'play none none none',
          onEnter: (self) => triggers.push(self),
        },
      }
    );

    // Path draw animation
    if (pathRef.current) {
      const pathLength = pathRef.current.getTotalLength();
      gsap.set(pathRef.current, {
        strokeDasharray: pathLength,
        strokeDashoffset: pathLength,
      });

      gsap.to(pathRef.current, {
        strokeDashoffset: 0,
        duration: 2,
        ease: 'none',
        scrollTrigger: {
          trigger: sectionRef.current,
          start: 'top 60%',
          end: 'bottom 80%',
          scrub: 1,
          onEnter: (self) => triggers.push(self),
        },
      });
    }

    // Steps animation
    stepsRef.current.forEach((step, index) => {
      if (!step) return;

      const isEven = index % 2 === 0;
      
      gsap.fromTo(step,
        { 
          opacity: 0, 
          x: isEven ? -60 : 60,
          scale: 0.9,
        },
        {
          opacity: 1,
          x: 0,
          scale: 1,
          duration: 0.8,
          ease: 'expo.out',
          scrollTrigger: {
            trigger: step,
            start: 'top 80%',
            toggleActions: 'play none none none',
            onEnter: (self) => triggers.push(self),
          },
        }
      );

      // Icon pulse animation
      const icon = step.querySelector('.step-icon');
      if (icon) {
        gsap.fromTo(icon,
          { scale: 0.5, opacity: 0 },
          {
            scale: 1,
            opacity: 1,
            duration: 0.6,
            delay: 0.2,
            ease: 'elastic.out(1, 0.5)',
            scrollTrigger: {
              trigger: step,
              start: 'top 80%',
              toggleActions: 'play none none none',
              onEnter: (self) => triggers.push(self),
            },
          }
        );
      }
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
      {/* Background gradient */}
      <div className="absolute inset-0 bg-gradient-to-b from-[#0e0e0e] via-[#111111] to-[#0e0e0e]" />

      <div className="relative z-10 container mx-auto px-4 sm:px-6 lg:px-8 xl:px-12">
        {/* Section Header */}
        <div ref={titleRef} className="text-center mb-20 space-y-4">
          <h2 className="text-3xl sm:text-4xl lg:text-5xl font-bold text-white">
            How It Works
          </h2>
          <p className="text-lg text-[#a0a0a0] max-w-2xl mx-auto">
            From discovery to deployment, Magpie handles the entire proxy lifecycle automatically.
          </p>
        </div>

        {/* Timeline */}
        <div className="relative max-w-4xl mx-auto">
          {/* SVG Path - Hidden on mobile */}
          <svg 
            className="absolute left-1/2 -translate-x-1/2 top-16 bottom-24 w-4 hidden lg:block"
            viewBox="0 0 16 800"
            preserveAspectRatio="none"
          >
            {/* Background line */}
            <line 
              x1="8" y1="0" x2="8" y2="720" 
              stroke="#2a2a2a" 
              strokeWidth="2"
              strokeDasharray="8 8"
            />
            {/* Animated line */}
            <path
              ref={pathRef}
              d="M 8 0 L 8 720"
              stroke="#3fa37a"
              strokeWidth="2"
              fill="none"
              strokeLinecap="round"
            />
            {/* Flowing dots */}
            <circle r="4" fill="#3fa37a" opacity="0.8">
              <animate 
                attributeName="cy" 
                from="0" to="720" 
                dur="3s" 
                repeatCount="indefinite"
              />
              <animate 
                attributeName="opacity" 
                values="0;1;0" 
                dur="3s" 
                repeatCount="indefinite"
              />
            </circle>
            <circle r="4" fill="#44b381" opacity="0.6">
              <animate 
                attributeName="cy" 
                from="0" to="720" 
                dur="3s" 
                begin="1s"
                repeatCount="indefinite"
              />
              <animate 
                attributeName="opacity" 
                values="0;1;0" 
                dur="3s" 
                begin="1s"
                repeatCount="indefinite"
              />
            </circle>
            <circle r="4" fill="#3fa37a" opacity="0.4">
              <animate 
                attributeName="cy" 
                from="0" to="720" 
                dur="3s" 
                begin="2s"
                repeatCount="indefinite"
              />
              <animate 
                attributeName="opacity" 
                values="0;1;0" 
                dur="3s" 
                begin="2s"
                repeatCount="indefinite"
              />
            </circle>
          </svg>

          {/* Steps */}
          <div className="space-y-16 lg:space-y-24">
            {steps.map((step, index) => {
              const isEven = index % 2 === 0;
              
              return (
                <div
                  key={step.title}
                  ref={el => { stepsRef.current[index] = el; }}
                  className={`relative flex flex-col lg:flex-row items-center gap-8 ${
                    isEven ? 'lg:flex-row' : 'lg:flex-row-reverse'
                  }`}
                >
                  {/* Content */}
                  <div className={`flex-1 text-center ${isEven ? 'lg:text-right' : 'lg:text-left'}`}>
                    <div className={`inline-flex items-center gap-3 mb-4 ${isEven ? 'lg:flex-row-reverse' : ''}`}>
                      <span 
                        className="text-5xl font-bold opacity-20"
                        style={{ color: step.color }}
                      >
                        0{index + 1}
                      </span>
                      <h3 className="text-2xl font-semibold text-white">
                        {step.title}
                      </h3>
                    </div>
                    <p className="text-[#a0a0a0] leading-relaxed max-w-md mx-auto lg:mx-0">
                      {step.description}
                    </p>
                  </div>

                  {/* Icon - Center */}
                  <div className="relative flex-shrink-0">
                    <div 
                      className="step-icon w-20 h-20 rounded-2xl flex items-center justify-center text-white shadow-lg"
                      style={{ 
                        backgroundColor: step.color,
                        boxShadow: `0 0 30px ${step.color}40`,
                      }}
                    >
                      {step.icon}
                    </div>
                    {/* Pulse ring */}
                    <div 
                      className="absolute inset-0 rounded-2xl animate-ping opacity-30"
                      style={{ backgroundColor: step.color }}
                    />
                  </div>

                  {/* Spacer for alternating layout */}
                  <div className="flex-1 hidden lg:block" />
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </section>
  );
}

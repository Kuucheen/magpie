import { useEffect, useRef, useState } from 'react';
import gsap from 'gsap';
import { ScrollTrigger } from 'gsap/ScrollTrigger';

gsap.registerPlugin(ScrollTrigger);

interface TechItem {
  name: string;
  color: string;
  description: string;
  logo: React.ReactNode;
}

// Go Logo
const GoLogo = () => (
  <picture>
    <source srcSet="/go-logo.webp" type="image/webp" />
    <img
      src="/go-logo.png"
      alt="Go"
      className="w-8 h-8 object-contain"
      width={32}
      height={32}
      loading="lazy"
      decoding="async"
    />
  </picture>
);

// TypeScript Logo SVG
const TypeScriptLogo = () => (
  <svg viewBox="0 0 128 128" className="w-8 h-8">
    <path fill="#3178C6" d="M2 63.91v62.5h125v-125H2zm100.73-5a15.56 15.56 0 017.82 4.5 20.58 20.58 0 013 4c0 .16-5.4 3.81-8.69 5.85-.12.08-.6-.44-1.13-1.23a7.09 7.09 0 00-5.87-3.53c-3.79-.26-6.23 1.73-6.21 5a4.58 4.58 0 00.54 2.34c.83 1.73 2.38 2.76 7.24 4.86 8.95 3.85 12.78 6.39 15.16 10 2.66 4 3.25 10.46 1.45 15.24-2 5.2-6.9 8.73-13.83 9.9a38.32 38.32 0 01-9.52-.1A23 23 0 0180 109.19c-1.15-1.27-3.39-4.58-3.25-4.82a9.34 9.34 0 011.15-.73L82.79 101l3.59-2.08.75 1.11a16.78 16.78 0 004.74 4.54c4 2.1 9.46 1.81 12.16-.62a5.43 5.43 0 00.69-6.92c-1-1.39-3-2.56-8.59-5-6.45-2.78-9.23-4.5-11.77-7.24a16.48 16.48 0 01-3.43-6.25 25 25 0 01-.22-8c1.33-6.23 6-10.58 12.82-11.87a31.66 31.66 0 019.49.26zm-29.34 5.24v5.12H57.16v46.23H45.65V69.26H29.38v-5a49.19 49.19 0 01.14-5.16c.06-.08 10-.12 22-.1h21.81z"/>
  </svg>
);

// Angular Logo SVG
const AngularLogo = () => (
  <svg viewBox="0 0 250 250" className="w-8 h-8">
    <path fill="#DD0031" d="M125 30L31.9 63.2l14.2 123.1L125 230l78.9-43.7 14.2-123.1z"/>
    <path fill="#C3002F" d="M125 30v22.2-.1l23.2-23.2L125 30zM125 230v-54.1l.1.1-.1.1V230zm78.9-43.7l-14.2-123.1L125 52.1v.1l78.9 134.1z"/>
    <path fill="#FFF" d="M125 52.1L66.8 182.6h21.7l11.7-29.2h49.4l11.7 29.2h21.7L125 52.1zm17 83.3h-34l17-40.9 17 40.9z"/>
  </svg>
);

// Docker Logo SVG
const DockerLogo = () => (
  <svg viewBox="0 0 128 128" className="w-8 h-8">
    <path fill="#2496ED" d="M124.8 52.1c-4.1-2.7-13.4-3.7-20.5-1.7-.9-6.6-5.9-12.4-11.4-14.6l-2.3-1-1.3 2.2c-1.6 2.8-2.4 6-2.1 9.2.2 1.8.8 4.1 2.4 5.7-1.6.9-4.8 2-8.9 2H16.7c-2.1 0-3.9 1.7-3.9 3.9 0 7.8 1.2 15.7 3.6 23.3 2.3 7.3 6.5 13.6 11.9 18.1 6.6 5.3 15.8 8.1 26.4 8.1 2.4 0 4.8-.2 7.2-.7 5.3-1 10.3-3.1 14.7-6.2 3.8-2.6 7.1-6 9.6-9.9 4.6-6.8 7.3-14.4 8-22.1.8 0 1.6.1 2.4.1 7.4 0 13.6-3 17.8-8.3 1.3-1.7 2.4-3.6 3.2-5.6l.9-2.5-2.1-1.3zM29.4 55.3h12.5v10.4H29.4V55.3zm0 13.8h12.5v10.4H29.4V69.1zm0 13.8h12.5v10.4H29.4V82.9zm14.6-27.6H56.5v10.4H44V55.3zm0 13.8H56.5v10.4H44V69.1zm14.6-13.8h12.5v10.4H58.6V55.3zm0 13.8h12.5v10.4H58.6V69.1zm14.6-13.8h12.5v10.4H73.2V55.3zm0 13.8h12.5v10.4H73.2V69.1z"/>
  </svg>
);

const techStack: TechItem[] = [
  {
    name: 'Go',
    color: '#00ADD8',
    description: 'High-performance backend',
    logo: <GoLogo />,
  },
  {
    name: 'TypeScript',
    color: '#3178C6',
    description: 'Type-safe frontend',
    logo: <TypeScriptLogo />,
  },
  {
    name: 'Angular',
    color: '#DD0031',
    description: 'Modern UI framework',
    logo: <AngularLogo />,
  },
  {
    name: 'Docker',
    color: '#2496ED',
    description: 'Easy deployment',
    logo: <DockerLogo />,
  },
];

const hexToRgba = (hex: string, alpha: number) => {
  const normalized = hex.replace('#', '');
  const full = normalized.length === 3
    ? normalized.split('').map((c) => c + c).join('')
    : normalized;
  const value = parseInt(full, 16);
  const r = (value >> 16) & 255;
  const g = (value >> 8) & 255;
  const b = value & 255;
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
};

export default function TechStack() {
  const sectionRef = useRef<HTMLDivElement>(null);
  const orbitRef = useRef<HTMLDivElement>(null);
  const [hoveredIndex, setHoveredIndex] = useState<number | null>(null);
  const [rotation, setRotation] = useState(0);
  const animationRef = useRef<number | null>(null);
  const lastTimeRef = useRef<number>(0);

  // Smooth rotation animation using requestAnimationFrame
  useEffect(() => {
    const animate = (time: number) => {
      if (lastTimeRef.current === 0) {
        lastTimeRef.current = time;
      }
      
      if (hoveredIndex === null) {
        const delta = time - lastTimeRef.current;
        setRotation(prev => (prev + delta * 0.01) % 360);
      }
      
      lastTimeRef.current = time;
      animationRef.current = requestAnimationFrame(animate);
    };

    animationRef.current = requestAnimationFrame(animate);

    return () => {
      if (animationRef.current) {
        cancelAnimationFrame(animationRef.current);
      }
    };
  }, [hoveredIndex]);

  // Entrance animation
  useEffect(() => {
    const triggers: ScrollTrigger[] = [];

    gsap.fromTo(orbitRef.current,
      { opacity: 0, scale: 0.8 },
      {
        opacity: 1,
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

  const handleMouseEnter = (index: number) => {
    setHoveredIndex(index);
  };

  const handleMouseLeave = () => {
    setHoveredIndex(null);
  };

  return (
    <section 
      ref={sectionRef}
      className="relative w-full py-24 lg:py-32 bg-[#0e0e0e] overflow-hidden"
    >
      {/* Background */}
      <div className="absolute inset-0">
        <div 
          className="absolute inset-0 opacity-[0.02]"
          style={{
            backgroundImage: `radial-gradient(circle at 1px 1px, #3fa37a 1px, transparent 0)`,
            backgroundSize: '40px 40px',
          }}
        />
      </div>

      <div className="relative z-10 container mx-auto px-4 sm:px-6 lg:px-8 xl:px-12">
        {/* Section Header */}
        <div className="text-center mb-16 space-y-4">
          <h2 className="text-3xl sm:text-4xl lg:text-5xl font-bold text-white">
            Built With
          </h2>
          <p className="text-lg text-[#a0a0a0] max-w-2xl mx-auto">
            Modern technologies for a modern proxy manager.
          </p>
        </div>

        {/* Orbital System */}
        <div 
          ref={orbitRef}
          className="relative w-full max-w-xl mx-auto aspect-square"
        >
          {/* Orbit rings */}
          <div className="absolute inset-0 flex items-center justify-center">
            <div className="absolute w-[90%] h-[90%] rounded-full border border-[#2a2a2a]" />
            <div className="absolute w-[70%] h-[70%] rounded-full border border-[#2a2a2a]" />
            <div className="absolute w-[50%] h-[50%] rounded-full border border-[#2a2a2a]" />
          </div>

          {/* Center - Magpie Logo */}
          <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 z-20">
            <div
              className="w-28 h-28 rounded-full flex items-center justify-center"
              style={{
                backgroundColor: hexToRgba('#3fa37a', 0.18),
                borderColor: '#3fa37a',
                borderWidth: '1px',
                boxShadow: `0 0 30px ${hexToRgba('#3fa37a', 0.35)}`,
              }}
            >
              <img 
                src="/magpie-light-green.svg" 
                alt="Magpie" 
                className="w-16 h-16 object-contain drop-shadow-[0_0_6px_rgba(63,163,122,0.6)]"
              />
            </div>
          </div>

          {/* Orbiting tech items */}
          <div 
            className="absolute inset-0"
            style={{ transform: `rotate(${rotation}deg)` }}
          >
            {techStack.map((tech, index) => {
              const angle = (index * 90) * (Math.PI / 180);
              const radius = 42; // percentage
              const x = 50 + radius * Math.cos(angle);
              const y = 50 + radius * Math.sin(angle);
              const isHovered = hoveredIndex === index;

              return (
                <div
                  key={tech.name}
                  className="absolute"
                  style={{
                    left: `${x}%`,
                    top: `${y}%`,
                    transform: `translate(-50%, -50%)`,
                  }}
                >
                  {/* Counter-rotation to keep logo upright */}
                  <div 
                    style={{ transform: `rotate(${-rotation}deg)` }}
                  >
                    <div 
                      className={`w-16 h-16 rounded-xl flex flex-col items-center justify-center cursor-pointer transition-all duration-300 ${
                        isHovered 
                          ? 'shadow-lg scale-125' 
                          : 'shadow-md scale-100'
                      }`}
                      style={{ 
                        backgroundColor: isHovered ? hexToRgba(tech.color, 0.18) : '#1a1a1a',
                        borderColor: isHovered ? tech.color : '#2a2a2a',
                        borderWidth: '1px',
                        boxShadow: isHovered ? `0 0 30px ${hexToRgba(tech.color, 0.35)}` : 'none',
                      }}
                      onMouseEnter={() => handleMouseEnter(index)}
                      onMouseLeave={handleMouseLeave}
                    >
                      <div
                        className={`transition-all duration-300 ${isHovered ? 'scale-110' : ''}`}
                        style={{
                          filter: isHovered ? `drop-shadow(0 0 6px ${hexToRgba(tech.color, 0.6)})` : 'none',
                        }}
                      >
                        {tech.logo}
                      </div>
                    </div>

                    {/* Tooltip */}
                    {isHovered && (
                      <div className="absolute left-1/2 -translate-x-1/2 -bottom-24 whitespace-nowrap z-30">
                        <div className="bg-[#1a1a1a] border border-[#2a2a2a] rounded-lg px-4 py-2 text-center">
                          <div className="font-semibold text-white">{tech.name}</div>
                          <div className="text-sm text-[#a0a0a0]">{tech.description}</div>
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              );
            })}
          </div>

          {/* Decorative dots on orbits */}
          <div className="absolute inset-0 pointer-events-none">
            {[...Array(12)].map((_, i) => {
              const angle = (i * 30) * (Math.PI / 180);
              const radius = 45;
              const x = 50 + radius * Math.cos(angle);
              const y = 50 + radius * Math.sin(angle);
              
              return (
                <div
                  key={i}
                  className="absolute w-1 h-1 rounded-full bg-[#3fa37a]/30"
                  style={{
                    left: `${x}%`,
                    top: `${y}%`,
                    transform: 'translate(-50%, -50%)',
                  }}
                />
              );
            })}
          </div>
        </div>

        {/* Tech list below */}
        <div className="flex flex-wrap justify-center gap-4 mt-12">
          {techStack.map((tech) => (
            <div 
              key={tech.name}
              className="flex items-center gap-2 px-4 py-2 bg-[#1a1a1a] border border-[#2a2a2a] rounded-full"
            >
              <div 
                className="w-3 h-3 rounded-full"
                style={{ backgroundColor: tech.color }}
              />
              <span className="text-sm text-[#a0a0a0]">{tech.name}</span>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

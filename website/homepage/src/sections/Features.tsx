import { useEffect, useRef, useState } from 'react';
import { Card } from '@/components/ui/card';
import { 
  Globe, 
  Activity, 
  Award, 
  RefreshCw,
  Zap,
  Lock
} from 'lucide-react';
import gsap from 'gsap';
import { ScrollTrigger } from 'gsap/ScrollTrigger';

gsap.registerPlugin(ScrollTrigger);

interface FeatureCardProps {
  icon: React.ReactNode;
  title: string;
  description: string;
  image: string;
  index: number;
}

function FeatureCard({ icon, title, description, image, index }: FeatureCardProps) {
  const cardRef = useRef<HTMLDivElement>(null);
  const [mousePos, setMousePos] = useState({ x: 0, y: 0 });
  const [isHovered, setIsHovered] = useState(false);

  const handleMouseMove = (e: React.MouseEvent<HTMLDivElement>) => {
    if (!cardRef.current) return;
    const rect = cardRef.current.getBoundingClientRect();
    const x = (e.clientX - rect.left) / rect.width - 0.5;
    const y = (e.clientY - rect.top) / rect.height - 0.5;
    setMousePos({ x, y });
  };

  useEffect(() => {
    const card = cardRef.current;
    if (!card) return;

    const triggers: ScrollTrigger[] = [];

    gsap.fromTo(card,
      { opacity: 0, rotateX: 45, y: 60 },
      {
        opacity: 1,
        rotateX: 0,
        y: 0,
        duration: 0.8,
        delay: index * 0.1,
        ease: 'expo.out',
        scrollTrigger: {
          trigger: card,
          start: 'top 85%',
          toggleActions: 'play none none none',
          onEnter: (self) => triggers.push(self),
        },
      }
    );

    return () => {
      triggers.forEach(t => t.kill());
    };
  }, [index]);

  return (
    <div 
      ref={cardRef}
      className="perspective-1000 h-full"
      onMouseMove={handleMouseMove}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => {
        setIsHovered(false);
        setMousePos({ x: 0, y: 0 });
      }}
    >
      <Card 
        className={`relative h-full overflow-hidden bg-[#1a1a1a] border-[#2a2a2a] hover:border-[#3fa37a]/50 transition-all duration-500 preserve-3d card-shine group py-0 gap-0 ${
          isHovered ? 'shadow-xl shadow-[#3fa37a]/10' : ''
        }`}
        style={{
          transform: isHovered 
            ? `rotateY(${mousePos.x * 10}deg) rotateX(${-mousePos.y * 10}deg) translateZ(20px)` 
            : 'rotateY(0deg) rotateX(0deg) translateZ(0)',
          transition: isHovered ? 'transform 0.1s ease-out' : 'transform 0.5s ease-out',
        }}
      >
        {/* Image */}
        <div className="relative h-48 overflow-hidden">
          <img 
            src={image} 
            alt={`${title} feature diagram`}
            className="w-full h-full object-cover transition-transform duration-700 group-hover:scale-110"
            loading="lazy"
          />
          <div className="absolute inset-0 bg-gradient-to-t from-[#1a1a1a] via-transparent to-transparent" />
          
          {/* Icon badge */}
          <div className="absolute top-4 left-4 w-12 h-12 rounded-xl bg-[#151515]/90 backdrop-blur-sm border border-[#2a2a2a] flex items-center justify-center text-[#3fa37a] group-hover:bg-[#3fa37a] group-hover:text-white transition-all duration-300">
            {icon}
          </div>
        </div>

        {/* Content */}
        <div className="p-6 space-y-3">
          <h3 className="text-xl font-semibold text-white group-hover:text-[#3fa37a] transition-colors duration-300">
            {title}
          </h3>
          <p className="text-[#a0a0a0] text-sm leading-relaxed">
            {description}
          </p>
        </div>

        {/* Holographic sheen effect */}
        <div 
          className="absolute inset-0 pointer-events-none opacity-0 group-hover:opacity-100 transition-opacity duration-500"
          style={{
            background: `radial-gradient(circle at ${(mousePos.x + 0.5) * 100}% ${(mousePos.y + 0.5) * 100}%, rgba(63, 163, 122, 0.15) 0%, transparent 50%)`,
          }}
        />
      </Card>
    </div>
  );
}

const features = [
  {
    icon: <Globe className="w-6 h-6" />,
    title: 'Auto-scraping',
    description: 'Automatically scrape proxies from various public sources. Keep your pool fresh without manual intervention.',
    image: '/feature-scraping.svg',
  },
  {
    icon: <Activity className="w-6 h-6" />,
    title: 'Proxy Checking',
    description: 'Continuously verify which proxies are alive. Health checks run automatically to ensure reliability.',
    image: '/feature-checking.svg',
  },
  {
    icon: <Award className="w-6 h-6" />,
    title: 'Reputation System',
    description: 'Assign reputation scores based on uptime, latency, and anonymity. Filter out poor performers automatically.',
    image: '/feature-reputation.svg',
  },
  {
    icon: <RefreshCw className="w-6 h-6" />,
    title: 'Rotating Endpoints',
    description: 'Create your own rotating proxy endpoints from the healthy pool. Distribute traffic intelligently.',
    image: '/feature-rotating.svg',
  },
  {
    icon: <Zap className="w-6 h-6" />,
    title: 'Multiple Protocols',
    description: 'Support for HTTP, HTTPS, SOCKS4, and SOCKS5 proxies. TCP and QUIC/HTTP3 transport protocols.',
    image: '/feature-protocols.svg',
  },
  {
    icon: <Lock className="w-6 h-6" />,
    title: 'Self-hosted',
    description: 'Your data stays on your servers. Full privacy with end-to-end encryption for stored credentials.',
    image: '/feature-privacy.svg',
  },
];

export default function Features() {
  const sectionRef = useRef<HTMLDivElement>(null);
  const titleRef = useRef<HTMLDivElement>(null);
  const gridRef = useRef<HTMLDivElement>(null);

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

    return () => {
      triggers.forEach(t => t.kill());
    };
  }, []);

  return (
    <section 
      ref={sectionRef}
      className="relative w-full py-24 lg:py-32 bg-[#0e0e0e] overflow-hidden"
    >
      {/* Background grid pattern */}
      <div className="absolute inset-0 opacity-[0.03]">
        <div 
          className="w-full h-full"
          style={{
            backgroundImage: `
              linear-gradient(rgba(63, 163, 122, 0.5) 1px, transparent 1px),
              linear-gradient(90deg, rgba(63, 163, 122, 0.5) 1px, transparent 1px)
            `,
            backgroundSize: '60px 60px',
          }}
        />
      </div>

      <div className="relative z-10 container mx-auto px-4 sm:px-6 lg:px-8 xl:px-12">
        {/* Section Header */}
        <div ref={titleRef} className="text-center mb-16 space-y-4">
          <h2 className="text-3xl sm:text-4xl lg:text-5xl font-bold text-white">
            Everything You Need
          </h2>
          <p className="text-lg text-[#a0a0a0] max-w-2xl mx-auto">
            A complete proxy management solution with powerful features for scraping, checking, and rotating proxies.
          </p>
        </div>

        {/* Features Grid */}
        <div 
          ref={gridRef}
          className="grid sm:grid-cols-2 lg:grid-cols-3 gap-6"
        >
          {features.map((feature, index) => (
            <FeatureCard
              key={feature.title}
              {...feature}
              index={index}
            />
          ))}
        </div>
      </div>
    </section>
  );
}

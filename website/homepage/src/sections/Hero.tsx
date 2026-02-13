import { useEffect, useRef, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { BookOpen, Github, ArrowRight, Shield, Sparkles } from 'lucide-react';
import gsap from 'gsap';

interface Particle {
  x: number;
  y: number;
  vx: number;
  vy: number;
  radius: number;
}

export default function Hero() {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const heroRef = useRef<HTMLDivElement>(null);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const subtitleRef = useRef<HTMLParagraphElement>(null);
  const ctaRef = useRef<HTMLDivElement>(null);
  const imageRef = useRef<HTMLDivElement>(null);
  const [mousePos, setMousePos] = useState({ x: 0, y: 0 });

  const scrollToInstall = () => {
    const element = document.querySelector('#install');
    if (element) {
      element.scrollIntoView({ behavior: 'smooth' });
    }
  };

  // Particle network animation
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const resizeCanvas = () => {
      canvas.width = window.innerWidth;
      canvas.height = window.innerHeight;
    };
    resizeCanvas();
    window.addEventListener('resize', resizeCanvas);

    const particles: Particle[] = [];
    const particleCount = 80;
    const connectionDistance = 150;
    const maxConnections = 3;

    for (let i = 0; i < particleCount; i++) {
      particles.push({
        x: Math.random() * canvas.width,
        y: Math.random() * canvas.height,
        vx: (Math.random() - 0.5) * 0.5,
        vy: (Math.random() - 0.5) * 0.5,
        radius: Math.random() * 2 + 1,
      });
    }

    let animationId: number;
    let frameCount = 0;

    const animate = () => {
      frameCount++;
      // Render every 2nd frame for performance (30fps)
      if (frameCount % 2 === 0) {
        ctx.fillStyle = 'rgba(14, 14, 14, 0.15)';
        ctx.fillRect(0, 0, canvas.width, canvas.height);

        particles.forEach((particle, i) => {
          // Update position
          particle.x += particle.vx;
          particle.y += particle.vy;

          // Bounce off edges
          if (particle.x < 0 || particle.x > canvas.width) particle.vx *= -1;
          if (particle.y < 0 || particle.y > canvas.height) particle.vy *= -1;

          // Draw particle
          ctx.beginPath();
          ctx.arc(particle.x, particle.y, particle.radius, 0, Math.PI * 2);
          ctx.fillStyle = 'rgba(63, 163, 122, 0.6)';
          ctx.fill();

          // Draw connections (only check every 5th particle for performance)
          if (i % 5 === 0) {
            let connections = 0;
            for (let j = i + 1; j < particles.length && connections < maxConnections; j++) {
              const dx = particles[j].x - particle.x;
              const dy = particles[j].y - particle.y;
              const distance = Math.sqrt(dx * dx + dy * dy);

              if (distance < connectionDistance) {
                ctx.beginPath();
                ctx.moveTo(particle.x, particle.y);
                ctx.lineTo(particles[j].x, particles[j].y);
                ctx.strokeStyle = `rgba(63, 163, 122, ${0.15 * (1 - distance / connectionDistance)})`;
                ctx.lineWidth = 0.5;
                ctx.stroke();
                connections++;
              }
            }
          }
        });
      }

      animationId = requestAnimationFrame(animate);
    };

    animate();

    return () => {
      window.removeEventListener('resize', resizeCanvas);
      cancelAnimationFrame(animationId);
    };
  }, []);

  // GSAP entrance animations
  useEffect(() => {
    const ctx = gsap.context(() => {
      // Title animation - split chars (preserve line structure)
      if (titleRef.current) {
        const lineNodes = Array.from(
          titleRef.current.querySelectorAll<HTMLElement>('[data-split="chars"]')
        );
        const targets = lineNodes.length ? lineNodes : [titleRef.current];

        targets.forEach((node) => {
          const text = node.textContent ?? '';
          node.innerHTML = text
            .split('')
            .map((char) =>
              char === ' '
                ? ' '
                : `<span class="title-char inline-block opacity-0 translate-y-full">${char}</span>`
            )
            .join('');
        });

        gsap.to(titleRef.current.querySelectorAll('.title-char'), {
          opacity: 1,
          y: 0,
          duration: 0.8,
          stagger: 0.03,
          ease: 'expo.out',
          delay: 0.3,
        });
      }

      // Subtitle fade in
      gsap.fromTo(subtitleRef.current,
        { opacity: 0, filter: 'blur(10px)' },
        { opacity: 1, filter: 'blur(0px)', duration: 1, delay: 0.6, ease: 'power2.out' }
      );

      // CTA buttons
      gsap.fromTo(ctaRef.current,
        { opacity: 0, scale: 0.8 },
        { opacity: 1, scale: 1, duration: 0.6, delay: 0.9, ease: 'elastic.out(1, 0.5)' }
      );

      // Dashboard image 3D entrance
      gsap.fromTo(imageRef.current,
        { opacity: 0, rotateX: 45, y: 100, scale: 0.9 },
        { opacity: 1, rotateX: 0, y: 0, scale: 1, duration: 1.5, delay: 0.7, ease: 'expo.out' }
      );
    }, heroRef);

    return () => ctx.revert();
  }, []);

  // Mouse parallax effect for dashboard
  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      setMousePos({
        x: (e.clientX / window.innerWidth - 0.5) * 2,
        y: (e.clientY / window.innerHeight - 0.5) * 2,
      });
    };

    window.addEventListener('mousemove', handleMouseMove, { passive: true });
    return () => window.removeEventListener('mousemove', handleMouseMove);
  }, []);

  return (
    <section 
      ref={heroRef}
      className="relative min-h-screen w-full overflow-hidden bg-[#0e0e0e]"
    >
      {/* Particle Network Canvas */}
      <canvas
        ref={canvasRef}
        className="absolute inset-0 z-0"
      />

      {/* Gradient Overlay */}
      <div className="absolute inset-0 z-[1] bg-gradient-to-b from-transparent via-transparent to-[#0e0e0e]" />
      
      {/* Radial glow */}
      <div 
        className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[800px] h-[800px] rounded-full opacity-20 z-[1]"
        style={{
          background: 'radial-gradient(circle, rgba(63, 163, 122, 0.3) 0%, transparent 70%)',
        }}
      />

      {/* Content Container */}
      <div className="relative z-10 container mx-auto px-4 sm:px-6 lg:px-8 xl:px-12 pt-24 pb-16 min-h-screen flex items-center">
        <div className="grid lg:grid-cols-2 gap-12 lg:gap-8 items-center w-full">
          
          {/* Left Column - Text */}
          <div className="space-y-8 text-center lg:text-left">
            {/* Badge */}
            <div className="flex justify-center lg:justify-start">
              <Badge 
                variant="outline" 
                className="border-[#3fa37a]/50 text-[#3fa37a] bg-[#3fa37a]/10 px-4 py-1.5 text-sm font-medium animate-pulse-glow"
              >
                <Sparkles className="w-3.5 h-3.5 mr-2" />
                Free & Open Source
              </Badge>
            </div>

            {/* Headline */}
            <h1 
              ref={titleRef}
              className="text-4xl sm:text-5xl lg:text-6xl xl:text-7xl font-bold text-white leading-tight tracking-tight"
            >
              <span className="block" data-split="chars">Multi-user AIO</span>
              <span className="block whitespace-nowrap" data-split="chars">Proxy Manager</span>
            </h1>

            {/* Subheadline */}
            <p 
              ref={subtitleRef}
              className="text-lg sm:text-xl text-[#a0a0a0] max-w-xl mx-auto lg:mx-0 leading-relaxed"
            >
              Magpie is a <span className="text-[#3fa37a] font-medium">self-hosted proxy manager</span> that turns messy proxy lists into <span className="text-[#3fa37a] font-medium">something you can actually use</span>.
            </p>

            {/* CTAs */}
            <div ref={ctaRef} className="flex flex-col sm:flex-row gap-4 justify-center lg:justify-start">
              <Button 
                size="lg" 
                className="bg-[#3fa37a] hover:bg-[#44b381] text-white px-8 py-6 text-base font-medium transition-all duration-300 hover:scale-105 hover:shadow-lg hover:shadow-[#3fa37a]/25 group"
                onClick={scrollToInstall}
              >
                Get Started
                <ArrowRight className="w-4 h-4 ml-2 group-hover:translate-x-1 transition-transform" />
              </Button>
              <Button
                size="lg"
                variant="outline"
                className="border-[#2a2a2a] bg-[#1a1a1a]/50 text-white hover:bg-[#1a1a1a] hover:border-[#3fa37a]/50 px-8 py-6 text-base font-medium transition-all duration-300"
                asChild
              >
                <a
                  href="https://github.com/Kuucheen/magpie"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <Github className="w-4 h-4 mr-2" />
                  View on GitHub
                </a>
              </Button>
              <Button
                size="lg"
                variant="outline"
                className="border-[#2a2a2a] bg-[#1a1a1a]/50 text-white hover:bg-[#1a1a1a] hover:border-[#3fa37a]/50 px-8 py-6 text-base font-medium transition-all duration-300"
                asChild
              >
                <a href="/docs">
                  <BookOpen className="w-4 h-4 mr-2" />
                  Read the Docs
                </a>
              </Button>
            </div>

            {/* Trust indicators */}
            <div className="flex flex-wrap gap-6 justify-center lg:justify-start text-sm text-[#a0a0a0]">
              <div className="flex items-center gap-2">
                <Shield className="w-4 h-4 text-[#3fa37a]" />
                <span>Self-hosted</span>
              </div>
              <div className="flex items-center gap-2">
                <Sparkles className="w-4 h-4 text-[#3fa37a]" />
                <span>AGPL-3.0 Licensed</span>
              </div>
            </div>
          </div>

          {/* Right Column - Dashboard Preview */}
          <div 
            ref={imageRef}
            className="perspective-1000 relative"
            style={{
              transform: `rotateY(${mousePos.x * 8}deg) rotateX(${-mousePos.y * 8}deg)`,
              transition: 'transform 0.1s ease-out',
            }}
          >
            <div className="relative animate-float preserve-3d">
              {/* Glow effect behind image */}
              <div className="absolute -inset-4 bg-gradient-to-r from-[#3fa37a]/20 via-[#44b381]/10 to-[#3fa37a]/20 rounded-2xl blur-2xl opacity-60" />
              
              {/* Dashboard image */}
              <div className="relative rounded-xl overflow-hidden border border-[#2a2a2a] shadow-2xl bg-[#151515]">
                <img 
                  src="/dashboard-hero.png" 
                  alt="Magpie dashboard preview showing proxy stats and charts"
                  className="w-full h-auto"
                  loading="eager"
                />
                
                {/* Overlay gradient */}
                <div className="absolute inset-0 bg-gradient-to-t from-[#151515]/50 to-transparent pointer-events-none" />
              </div>

              {/* Floating stats cards */}
              <div className="absolute -left-4 top-1/4 bg-[#1a1a1a] border border-[#2a2a2a] rounded-lg p-3 shadow-xl animate-breathe"
                style={{ animationDelay: '0s' }}
              >
                <div className="text-xs text-[#a0a0a0]">Alive Proxies</div>
                <div className="text-xl font-bold text-[#3fa37a]">793</div>
              </div>
              
              <div className="absolute -right-4 top-1/3 bg-[#1a1a1a] border border-[#2a2a2a] rounded-lg p-3 shadow-xl animate-breathe"
                style={{ animationDelay: '1s' }}
              >
                <div className="text-xs text-[#a0a0a0]">Total Proxies</div>
                <div className="text-xl font-bold text-white">9,069</div>
              </div>
              
              <div className="absolute -left-6 bottom-1/4 bg-[#1a1a1a] border border-[#2a2a2a] rounded-lg p-3 shadow-xl animate-breathe"
                style={{ animationDelay: '2s' }}
              >
                <div className="text-xs text-[#a0a0a0]">Scraped</div>
                <div className="text-xl font-bold text-[#44b381]">7,937</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

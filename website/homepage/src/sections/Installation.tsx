import { useEffect, useRef, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Copy, Check, Terminal, Apple, Monitor } from 'lucide-react';
import gsap from 'gsap';
import { ScrollTrigger } from 'gsap/ScrollTrigger';

gsap.registerPlugin(ScrollTrigger);

const installCommands = {
  macos: `curl -fsSL https://raw.githubusercontent.com/Kuucheen/magpie/refs/heads/master/scripts/install.sh | bash`,
  windows: `iwr -useb https://raw.githubusercontent.com/Kuucheen/magpie/refs/heads/master/scripts/install.ps1 | iex`,
  docker: `git clone https://github.com/Kuucheen/magpie.git && cd magpie && cp .env.example .env && docker compose up -d`,
};

export default function Installation() {
  const sectionRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<HTMLDivElement>(null);
  const [activeTab, setActiveTab] = useState<'macos' | 'windows' | 'docker'>('macos');
  const [typedText, setTypedText] = useState('');
  const [isTyping, setIsTyping] = useState(false);
  const [typingEnabled, setTypingEnabled] = useState(false);
  const [copied, setCopied] = useState(false);

  // Typing animation
  useEffect(() => {
    if (!typingEnabled) {
      setTypedText('');
      setIsTyping(false);
      return;
    }

    const command = installCommands[activeTab];
    setTypedText('');
    setIsTyping(true);
    
    let currentIndex = 0;
    const typeInterval = setInterval(() => {
      if (currentIndex <= command.length) {
        setTypedText(command.slice(0, currentIndex));
        currentIndex++;
      } else {
        clearInterval(typeInterval);
        setIsTyping(false);
      }
    }, 30);

    return () => clearInterval(typeInterval);
  }, [activeTab, typingEnabled]);

  // Entrance animation
  useEffect(() => {
    const triggers: ScrollTrigger[] = [];

    gsap.fromTo(terminalRef.current,
      { opacity: 0, y: 60, scale: 0.95 },
      {
        opacity: 1,
        y: 0,
        scale: 1,
        duration: 1,
        ease: 'expo.out',
        scrollTrigger: {
          trigger: sectionRef.current,
          start: 'top 70%',
          toggleActions: 'play none none none',
          onEnter: (self) => {
            triggers.push(self);
            setTypingEnabled(true);
          },
          onEnterBack: () => setTypingEnabled(true),
        },
      }
    );

    return () => {
      triggers.forEach(t => t.kill());
    };
  }, []);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(installCommands[activeTab]);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <section 
      ref={sectionRef}
      id="install"
      className="relative w-full py-24 lg:py-32 bg-[#0e0e0e] overflow-hidden"
    >
      {/* Background gradient */}
      <div className="absolute inset-0">
        <div 
          className="absolute bottom-0 left-1/2 -translate-x-1/2 w-[800px] h-[400px] opacity-20"
          style={{
            background: 'radial-gradient(ellipse, rgba(63, 163, 122, 0.3) 0%, transparent 70%)',
          }}
        />
      </div>

      <div className="relative z-10 container mx-auto px-4 sm:px-6 lg:px-8 xl:px-12">
        {/* Section Header */}
        <div className="text-center mb-12 space-y-4">
          <h2 className="text-3xl sm:text-4xl lg:text-5xl font-bold text-white">
            Get Started in Seconds
          </h2>
          <p className="text-lg text-[#a0a0a0] max-w-2xl mx-auto">
            One command to install Magpie. No complex configuration required.
          </p>
        </div>

        {/* Terminal */}
        <div 
          ref={terminalRef}
          className="max-w-3xl mx-auto"
        >
          {/* Terminal Window */}
          <div className="bg-[#151515] rounded-xl overflow-hidden border border-[#2a2a2a] shadow-2xl">
            {/* Terminal Header */}
            <div className="flex items-center justify-between px-4 py-3 bg-[#1a1a1a] border-b border-[#2a2a2a]">
              <div className="flex items-center gap-2">
                <div className="w-3 h-3 rounded-full bg-[#ff5f56]" />
                <div className="w-3 h-3 rounded-full bg-[#ffbd2e]" />
                <div className="w-3 h-3 rounded-full bg-[#27ca40]" />
              </div>
              <div className="flex items-center gap-2 text-[#a0a0a0]">
                <Terminal className="w-4 h-4" />
                <span className="text-sm font-mono">install.sh</span>
              </div>
              <div className="w-16" />
            </div>

            {/* Tabs */}
            <div className="flex border-b border-[#2a2a2a]">
              <button
                onClick={() => setActiveTab('macos')}
                className={`flex items-center gap-2 px-4 py-2 text-sm font-medium transition-colors duration-300 ${
                  activeTab === 'macos'
                    ? 'bg-[#1a1a1a] text-[#3fa37a] border-b-2 border-[#3fa37a]'
                    : 'text-[#a0a0a0] hover:text-white'
                }`}
              >
                <Apple className="w-4 h-4" />
                macOS / Linux
              </button>
              <button
                onClick={() => setActiveTab('windows')}
                className={`flex items-center gap-2 px-4 py-2 text-sm font-medium transition-colors duration-300 ${
                  activeTab === 'windows'
                    ? 'bg-[#1a1a1a] text-[#3fa37a] border-b-2 border-[#3fa37a]'
                    : 'text-[#a0a0a0] hover:text-white'
                }`}
              >
                <Monitor className="w-4 h-4" />
                Windows
              </button>
              <button
                onClick={() => setActiveTab('docker')}
                className={`flex items-center gap-2 px-4 py-2 text-sm font-medium transition-colors duration-300 ${
                  activeTab === 'docker'
                    ? 'bg-[#1a1a1a] text-[#3fa37a] border-b-2 border-[#3fa37a]'
                    : 'text-[#a0a0a0] hover:text-white'
                }`}
              >
                <svg className="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M13.983 11.078h2.119a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.119a.185.185 0 00-.185.185v1.888c0 .102.083.185.185.185m-2.954-5.43h2.119a.186.186 0 00.186-.186V3.574a.186.186 0 00-.186-.185h-2.119a.185.185 0 00-.185.185v1.888c0 .102.083.186.185.186m0 2.716h2.119a.187.187 0 00.186-.186V6.29a.186.186 0 00-.186-.185h-2.119a.185.185 0 00-.185.185v1.887c0 .102.083.186.185.186m-2.93 0h2.12a.186.186 0 00.184-.186V6.29a.185.185 0 00-.185-.185H8.1a.185.185 0 00-.185.185v1.887c0 .102.083.186.185.186m-2.964 0h2.119a.186.186 0 00.185-.186V6.29a.185.185 0 00-.185-.185H5.136a.186.186 0 00-.186.185v1.887c0 .102.084.186.186.186m5.893 2.715h2.118a.186.186 0 00.186-.185V9.006a.186.186 0 00-.186-.186h-2.118a.185.185 0 00-.185.185v1.888c0 .102.083.185.185.185m-2.93 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.185.185 0 00-.184.185v1.888c0 .102.083.185.185.185m-2.964 0h2.119a.185.185 0 00.185-.185V9.006a.185.185 0 00-.185-.186h-2.119a.186.186 0 00-.186.185v1.888c0 .102.084.185.186.185m-2.92 0h2.12a.185.185 0 00.184-.185V9.006a.185.185 0 00-.184-.186h-2.12a.185.185 0 00-.184.185v1.888c0 .102.083.185.185.185m20.744 1.3c-.527-.73-1.304-1.063-2.12-1.063-.757 0-1.387.283-1.884.766-.483-.483-1.113-.766-1.87-.766-.816 0-1.593.333-2.12 1.063-.483-.73-1.26-1.063-2.076-1.063-.816 0-1.593.333-2.12 1.063-.483-.73-1.26-1.063-2.076-1.063-.816 0-1.593.333-2.12 1.063-.483-.73-1.26-1.063-2.076-1.063-.816 0-1.593.333-2.12 1.063-.497-.73-1.274-1.063-2.09-1.063-.816 0-1.593.333-2.12 1.063C2.65 9.5 1.873 9.833 1.057 9.833c-.816 0-1.593.333-2.12 1.063-.483.73-.483 1.63 0 2.36.527.73 1.304 1.063 2.12 1.063.816 0 1.593-.333 2.12-1.063.527.73 1.304 1.063 2.12 1.063.816 0 1.593-.333 2.12-1.063.527.73 1.304 1.063 2.12 1.063.816 0 1.593-.333 2.12-1.063.527.73 1.304 1.063 2.12 1.063.816 0 1.593-.333 2.12-1.063.527.73 1.304 1.063 2.12 1.063.816 0 1.593-.333 2.12-1.063.527.73 1.304 1.063 2.12 1.063.816 0 1.593-.333 2.12-1.063.483-.73.483-1.63 0-2.36z"/>
                </svg>
                Docker
              </button>
            </div>

            {/* Terminal Content */}
            <div className="p-6 font-mono text-sm">
              {/* Prompt */}
              <div className="flex items-start gap-2">
                <span className="text-[#3fa37a]">$</span>
                <div className="flex-1 min-w-0">
                  <span className="text-white whitespace-pre-wrap break-words leading-5">
                    {typedText}
                  </span>
                  <span className={`inline-block w-2 h-5 bg-[#3fa37a] ml-0.5 align-baseline ${isTyping ? '' : 'cursor-blink'}`} />
                </div>
              </div>

              {/* Copy button */}
              <div className="flex justify-end mt-4">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleCopy}
                  className={`border-[#2a2a2a] bg-[#1a1a1a] hover:bg-[#252525] hover:border-[#3fa37a]/50 transition-all duration-300 ${
                    copied ? 'border-[#3fa37a] text-[#3fa37a]' : 'text-[#a0a0a0]'
                  }`}
                >
                  {copied ? (
                    <>
                      <Check className="w-4 h-4 mr-2" />
                      Copied!
                    </>
                  ) : (
                    <>
                      <Copy className="w-4 h-4 mr-2" />
                      Copy Command
                    </>
                  )}
                </Button>
              </div>
            </div>
          </div>

          {/* Additional info */}
          <div className="mt-6 text-center">
            <p className="text-sm text-[#a0a0a0]">
              Requires <a href="https://www.docker.com/" target="_blank" rel="noopener noreferrer" className="text-[#3fa37a] hover:underline">Docker Desktop</a> or Docker Engine + Compose
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}

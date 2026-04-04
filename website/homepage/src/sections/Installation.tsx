import { useEffect, useRef, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Copy, Check, Terminal, Apple, Monitor } from 'lucide-react';
import gsap from 'gsap';
import { ScrollTrigger } from 'gsap/ScrollTrigger';

gsap.registerPlugin(ScrollTrigger);

const installCommands = {
  macos: `curl -fsSL https://raw.githubusercontent.com/Kuucheen/magpie/refs/heads/master/scripts/install.sh | bash`,
  windows: `iwr -useb https://raw.githubusercontent.com/Kuucheen/magpie/refs/heads/master/scripts/install.ps1 | iex`,
};

export default function Installation() {
  const sectionRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<HTMLDivElement>(null);
  const [activeTab, setActiveTab] = useState<'macos' | 'windows'>('macos');
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
                  <span className={`inline-block w-2 h-5 bg-[#3fa37a] ml-0.5 align-text-bottom ${isTyping ? '' : 'cursor-blink'}`} />
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

        </div>
      </div>
    </section>
  );
}

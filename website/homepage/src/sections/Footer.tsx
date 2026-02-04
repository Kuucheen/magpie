import { Github, FileText, Scale, Heart } from 'lucide-react';

const footerLinks = [
  {
    label: 'GitHub',
    href: 'https://github.com/Kuucheen/magpie',
    icon: <Github className="w-4 h-4" />,
  },
  {
    label: 'Documentation',
    href: 'https://github.com/Kuucheen/magpie#readme',
    icon: <FileText className="w-4 h-4" />,
  },
  {
    label: 'License',
    href: 'https://github.com/Kuucheen/magpie/blob/master/LICENSE',
    icon: <Scale className="w-4 h-4" />,
  },
];

export default function Footer() {
  return (
    <footer className="relative w-full py-12 bg-[#0e0e0e] border-t border-[#1a1a1a]">
      <div className="container mx-auto px-4 sm:px-6 lg:px-8 xl:px-12">
        <div className="flex flex-col md:flex-row items-center justify-between gap-6">
          {/* Logo */}
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 flex items-center justify-center">
              <img 
                src="/magpie-light-green.svg" 
                alt="Magpie" 
                className="w-7 h-7 object-contain"
              />
            </div>
            <div>
              <div className="text-lg font-semibold text-white">Magpie</div>
              <div className="text-xs text-[#a0a0a0]">Multi-user AIO Proxy Manager</div>
            </div>
          </div>

          {/* Links */}
          <nav className="flex flex-wrap justify-center gap-6">
            {footerLinks.map((link) => (
              <a
                key={link.label}
                href={link.href}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 text-sm text-[#a0a0a0] hover:text-[#3fa37a] transition-colors duration-300"
              >
                {link.icon}
                {link.label}
              </a>
            ))}
          </nav>

          {/* Copyright */}
          <div className="text-sm text-[#a0a0a0] text-center md:text-right">
            <div>&copy; {new Date().getFullYear()} Magpie</div>
            <div className="flex items-center justify-center md:justify-end gap-1 mt-1">
              <span>Open source under</span>
              <a 
                href="https://github.com/Kuucheen/magpie/blob/master/LICENSE"
                target="_blank"
                rel="noopener noreferrer"
                className="text-[#3fa37a] hover:underline"
              >
                AGPL-3.0
              </a>
            </div>
          </div>
        </div>

        {/* Bottom bar */}
        <div className="mt-8 pt-6 border-t border-[#1a1a1a] text-center">
          <p className="text-xs text-[#606060] flex items-center justify-center gap-1">
            Crafted with <Heart className="w-3 h-3 text-red-500 fill-red-500" /> by 
            <a 
              href="https://github.com/Kuucheen"
              target="_blank"
              rel="noopener noreferrer"
              className="text-[#a0a0a0] hover:text-[#3fa37a] transition-colors"
            >
              Kuucheen
            </a>
            and contributors
          </p>
        </div>
      </div>
    </footer>
  );
}

import { useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';

interface DeferredSectionProps {
  id?: string;
  className?: string;
  children: ReactNode;
  minHeight?: number;
  rootMargin?: string;
}

export default function DeferredSection({
  id,
  className,
  children,
  minHeight = 420,
  rootMargin = '400px 0px',
}: DeferredSectionProps) {
  const sectionRef = useRef<HTMLElement | null>(null);
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    const section = sectionRef.current;
    if (!section) return;

    if (typeof IntersectionObserver === 'undefined') {
      setIsVisible(true);
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setIsVisible(true);
          observer.disconnect();
        }
      },
      { rootMargin }
    );

    observer.observe(section);
    return () => observer.disconnect();
  }, [rootMargin]);

  return (
    <section id={id} className={className} ref={sectionRef}>
      {isVisible ? children : <div style={{ minHeight }} aria-hidden="true" />}
    </section>
  );
}

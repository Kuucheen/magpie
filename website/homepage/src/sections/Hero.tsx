import { useEffect, useMemo, useRef, useState } from 'react';
import { layoutNextLine, prepareWithSegments, type LayoutCursor } from '@chenglou/pretext';
import { ArrowRight, BookOpen, ChevronDown } from 'lucide-react';

import { Button } from '@/components/ui/button';

const PROXY_BREAK = '\u200b';
const LINE_START_CURSOR: LayoutCursor = { segmentIndex: 0, graphemeIndex: 0 };
const PROXY_TOKENS = createProxyTokens(2600);
const PROXY_TEXT = PROXY_TOKENS.map((token) => `${token};${PROXY_BREAK}`).join('');

type StageSize = {
  height: number;
  width: number;
};

type ProxyFragment = {
  proxyStartIndex: number;
  text: string;
  x: number;
  y: number;
};

type Interval = {
  left: number;
  right: number;
};

type Rect = {
  height: number;
  width: number;
  x: number;
  y: number;
};

type LogoMask = {
  data: Uint8ClampedArray;
  height: number;
  width: number;
};

function createSeededRandom(seed: number) {
  let value = seed >>> 0;

  return () => {
    value = (value * 1664525 + 1013904223) >>> 0;
    return value / 4294967296;
  };
}

function createProxyTokens(count: number) {
  const random = createSeededRandom(37);
  const firstOctets = [23, 31, 37, 45, 62, 77, 85, 91, 104, 128, 143, 154, 172, 185, 198];
  const portPool = [80, 443, 554, 8080, 8443, 8888, 9000, 1080, 3128, 4145, 5001, 7000];

  return Array.from({ length: count }, (_, index) => {
    const first = firstOctets[index % firstOctets.length]!;
    const second = Math.floor(random() * 256);
    const third = 1 + Math.floor(random() * 254);
    const fourth = 1 + Math.floor(random() * 254);
    const port = portPool[Math.floor(random() * portPool.length)]!;

    return `${first}.${second}.${third}.${fourth}:${port}`;
  });
}

function getFontSize(stageWidth: number) {
  if (stageWidth >= 1280) {
    return 14;
  }

  if (stageWidth >= 768) {
    return 13;
  }

  return 11;
}

function splitProxyTokens(text: string) {
  return text
    .split(PROXY_BREAK)
    .map((token) => token.trim())
    .filter(Boolean);
}

function createLogoMask(image: HTMLImageElement, width: number, height: number): LogoMask | null {
  const canvasWidth = Math.max(1, Math.round(width));
  const canvasHeight = Math.max(1, Math.round(height));
  const canvas = document.createElement('canvas');
  canvas.width = canvasWidth;
  canvas.height = canvasHeight;

  const context = canvas.getContext('2d', { willReadFrequently: true });
  if (!context) {
    return null;
  }

  context.clearRect(0, 0, canvasWidth, canvasHeight);
  context.drawImage(image, 0, 0, canvasWidth, canvasHeight);
  const imageData = context.getImageData(0, 0, canvasWidth, canvasHeight);

  return {
    data: imageData.data,
    height: canvasHeight,
    width: canvasWidth,
  };
}

function mergeIntervals(intervals: Interval[]) {
  if (intervals.length <= 1) {
    return intervals;
  }

  const sorted = [...intervals].sort((left, right) => left.left - right.left);
  const merged: Interval[] = [sorted[0]!];

  for (let index = 1; index < sorted.length; index += 1) {
    const current = sorted[index]!;
    const previous = merged[merged.length - 1]!;

    if (current.left <= previous.right) {
      previous.right = Math.max(previous.right, current.right);
      continue;
    }

    merged.push({ ...current });
  }

  return merged;
}

function getBlockedIntervalsForBand(mask: LogoMask, rect: Rect, bandTop: number, bandBottom: number) {
  const localTop = Math.max(0, Math.floor(bandTop - rect.y));
  const localBottom = Math.min(mask.height, Math.ceil(bandBottom - rect.y));
  if (localBottom <= localTop) {
    return [];
  }

  const occupied = new Uint8Array(mask.width);
  for (let y = localTop; y < localBottom; y += 1) {
    const rowOffset = y * mask.width * 4;
    for (let x = 0; x < mask.width; x += 1) {
      if (mask.data[rowOffset + x * 4 + 3] > 24) {
        occupied[x] = 1;
      }
    }
  }

  const intervals: Interval[] = [];
  const padding = 8;
  let x = 0;

  while (x < mask.width) {
    while (x < mask.width && occupied[x] === 0) {
      x += 1;
    }

    if (x >= mask.width) {
      break;
    }

    const start = x;
    while (x < mask.width && occupied[x] === 1) {
      x += 1;
    }

    intervals.push({
      left: rect.x + start - padding,
      right: rect.x + x + padding,
    });
  }

  return mergeIntervals(intervals);
}

function carveSlots(region: Interval, blocked: Interval[]) {
  if (blocked.length === 0) {
    return [region];
  }

  const slots: Interval[] = [];
  let cursor = region.left;

  for (let index = 0; index < blocked.length; index += 1) {
    const interval = blocked[index]!;
    const clippedLeft = Math.max(region.left, interval.left);
    const clippedRight = Math.min(region.right, interval.right);

    if (clippedRight <= clippedLeft) {
      continue;
    }

    if (clippedLeft > cursor) {
      slots.push({ left: cursor, right: clippedLeft });
    }

    cursor = Math.max(cursor, clippedRight);
  }

  if (cursor < region.right) {
    slots.push({ left: cursor, right: region.right });
  }

  return slots;
}

function createBackgroundLayout(
  prepared: ReturnType<typeof prepareWithSegments>,
  stage: StageSize,
  logoRect: Rect | null,
  logoImage: HTMLImageElement | null,
  lineHeight: number
) {
  const paddingX = stage.width >= 1024 ? 18 : 12;
  const paddingY = stage.width >= 768 ? 12 : 10;
  const region = { left: paddingX, right: stage.width - paddingX };
  const fragments: ProxyFragment[] = [];
  const mask =
    logoImage && logoRect && logoRect.width > 0 && logoRect.height > 0
      ? createLogoMask(logoImage, logoRect.width, logoRect.height)
      : null;
  let cursor: LayoutCursor = { ...LINE_START_CURSOR };
  let proxyIndex = 0;

  for (let lineTop = paddingY; lineTop + lineHeight <= stage.height - paddingY; lineTop += lineHeight) {
    const blocked =
      mask && logoRect
        ? getBlockedIntervalsForBand(mask, logoRect, lineTop, lineTop + lineHeight)
        : [];
    const slots = carveSlots(region, blocked);

    for (let slotIndex = 0; slotIndex < slots.length; slotIndex += 1) {
      const slot = slots[slotIndex]!;
      const width = Math.max(0, Math.floor(slot.right - slot.left));
      if (width < 144) {
        continue;
      }

      let line = layoutNextLine(prepared, cursor, width);
      if (line === null && (cursor.segmentIndex !== 0 || cursor.graphemeIndex !== 0)) {
        cursor = { ...LINE_START_CURSOR };
        proxyIndex = 0;
        line = layoutNextLine(prepared, cursor, width);
      }

      if (line === null) {
        continue;
      }

      const tokenCount = splitProxyTokens(line.text).length;
      fragments.push({
        proxyStartIndex: proxyIndex,
        text: line.text,
        x: Math.round(slot.left),
        y: Math.round(lineTop),
      });

      cursor = line.end;
      proxyIndex += tokenCount;
    }
  }

  return fragments;
}

export default function Hero() {
  const backgroundRef = useRef<HTMLDivElement>(null);
  const logoRef = useRef<HTMLImageElement>(null);
  const [stageSize, setStageSize] = useState<StageSize>({ height: 0, width: 0 });
  const [fontsReady, setFontsReady] = useState(
    () => typeof document === 'undefined' || !('fonts' in document)
  );
  const [logoImage, setLogoImage] = useState<HTMLImageElement | null>(null);
  const [logoLoaded, setLogoLoaded] = useState(false);
  const [logoRect, setLogoRect] = useState<Rect | null>(null);

  const scrollToInstall = () => {
    const element = document.querySelector('#install');
    if (element) {
      element.scrollIntoView({ behavior: 'smooth' });
    }
  };

  const scrollToFeatures = () => {
    const element = document.querySelector('#features');
    if (element) {
      element.scrollIntoView({ behavior: 'smooth' });
    }
  };

  useEffect(() => {
    let cancelled = false;

    if (fontsReady || !('fonts' in document)) {
      return undefined;
    }

    document.fonts.ready.then(() => {
      if (!cancelled) {
        setFontsReady(true);
      }
    });

    return () => {
      cancelled = true;
    };
  }, [fontsReady]);

  useEffect(() => {
    const stageElement = backgroundRef.current;
    const logoElement = logoRef.current;
    if (!stageElement || !logoElement) {
      return undefined;
    }

    const syncMeasurements = () => {
      const stageBox = stageElement.getBoundingClientRect();
      const logoBox = logoElement.getBoundingClientRect();
      const nextWidth = Math.floor(stageBox.width);
      const nextHeight = Math.floor(stageBox.height);

      setStageSize((current) =>
        current.width === nextWidth && current.height === nextHeight
          ? current
          : { height: nextHeight, width: nextWidth }
      );

      const nextLogoRect: Rect = {
        height: Math.floor(logoBox.height),
        width: Math.floor(logoBox.width),
        x: Math.floor(logoBox.left - stageBox.left),
        y: Math.floor(logoBox.top - stageBox.top),
      };

      setLogoRect((current) => {
        if (
          current &&
          current.x === nextLogoRect.x &&
          current.y === nextLogoRect.y &&
          current.width === nextLogoRect.width &&
          current.height === nextLogoRect.height
        ) {
          return current;
        }

        return nextLogoRect;
      });
    };

    syncMeasurements();

    const observer = new ResizeObserver(() => {
      syncMeasurements();
    });

    observer.observe(stageElement);
    observer.observe(logoElement);
    window.addEventListener('resize', syncMeasurements);

    return () => {
      observer.disconnect();
      window.removeEventListener('resize', syncMeasurements);
    };
  }, [logoLoaded]);

  const proxyFontSize = getFontSize(stageSize.width);
  const proxyLineHeight = proxyFontSize + 8;
  const proxyFont = `400 ${proxyFontSize}px "Courier New"`;
  const preparedProxyText = useMemo(() => {
    if (!fontsReady || stageSize.width === 0) {
      return null;
    }

    return prepareWithSegments(PROXY_TEXT, proxyFont);
  }, [fontsReady, proxyFont, stageSize.width]);

  const proxyFragments = useMemo(() => {
    if (!preparedProxyText || stageSize.width === 0 || stageSize.height === 0) {
      return [];
    }

    return createBackgroundLayout(
      preparedProxyText,
      stageSize,
      logoRect,
      logoImage,
      proxyLineHeight
    );
  }, [logoImage, logoRect, preparedProxyText, proxyLineHeight, stageSize]);

  const logoClassName =
    'hero-bouncing-logo object-contain h-[170px] w-[170px] sm:h-[220px] sm:w-[220px] lg:h-[300px] lg:w-[300px]';

  return (
    <section className="hero-landing relative min-h-[100svh] w-full overflow-hidden bg-transparent sm:min-h-screen">
      <div
        ref={backgroundRef}
        className="pointer-events-none absolute inset-0 overflow-hidden bg-[#0e0e0e]"
      >
        <div className="absolute inset-0">
          {proxyFragments.map((fragment, fragmentIndex) => {
            const tokens = splitProxyTokens(fragment.text);

            return (
              <p
                key={`proxy-fragment-${fragmentIndex}`}
                className="absolute left-0 top-0 whitespace-pre leading-none"
                style={{
                  fontFamily: '"Courier New", monospace',
                  fontSize: `${proxyFontSize}px`,
                  left: `${fragment.x}px`,
                  lineHeight: `${proxyLineHeight}px`,
                  top: `${fragment.y}px`,
                }}
              >
                {tokens.map((token, tokenIndex) => {
                  const proxyIndex = fragment.proxyStartIndex + tokenIndex;
                  return (
                    <span
                      key={`${fragmentIndex}-${proxyIndex}`}
                      className={proxyIndex % 2 === 0 ? 'text-[#2f2f2f]' : 'text-[#3a3a3a]'}
                    >
                      {token}
                    </span>
                  );
                })}
              </p>
            );
          })}
        </div>
      </div>

      <div className="relative z-10 mx-auto flex min-h-[100svh] w-full max-w-7xl items-center px-4 pb-20 pt-28 sm:min-h-screen sm:px-6 sm:pb-16 sm:pt-24 lg:px-8">
        <div className="grid w-full items-center gap-12 lg:grid-cols-[minmax(0,1.05fr)_minmax(280px,0.95fr)] lg:gap-8">
          <div className="max-w-[34rem]">
            <h1 className="mt-2 max-w-[10ch] font-['Outfit',sans-serif] text-[clamp(3.35rem,10vw,5.8rem)] font-semibold uppercase leading-[0.94] tracking-[0.2em] text-[#f6f2ea]">
              MAGPIE
            </h1>

            <p className="mt-7 max-w-[31rem] text-lg leading-8 text-[#d8e1dc]">
              A self-hostable multi-user proxy manager for individuals, teams, rotators, and scraping workloads.
            </p>

            <div className="mt-10 flex flex-col items-start gap-3 sm:flex-row">
              <Button
                size="lg"
                className="min-w-[12rem] bg-[#f2ede2] text-[#0f1311] transition-all duration-300 hover:scale-[1.02] hover:bg-[#faf5ea]"
                onClick={scrollToInstall}
              >
                Get Started
                <ArrowRight className="ml-2 h-4 w-4" />
              </Button>

              <Button
                size="lg"
                variant="outline"
                className="min-w-[12rem] border-[#2f4939] bg-[#111814]/75 text-[#eff4f1] transition-all duration-300 hover:border-[#446a54] hover:bg-[#18221c]"
                asChild
              >
                <a href="/docs">
                  <BookOpen className="mr-2 h-4 w-4" />
                  Read the Docs
                </a>
              </Button>
            </div>
          </div>

          <div className="relative flex min-h-[260px] items-center justify-center lg:min-h-[560px]">
            <img
              ref={logoRef}
              src="/magpie-light-green.svg"
              alt="Magpie logo"
              className={logoClassName}
              onLoad={(event) => {
                setLogoImage(event.currentTarget);
                setLogoLoaded(true);
              }}
            />
          </div>
        </div>
      </div>

      <button
        type="button"
        onClick={scrollToFeatures}
        className="hero-scroll-cue absolute bottom-6 left-1/2 z-20 flex h-12 w-12 -translate-x-1/2 items-center justify-center rounded-full border border-white/10 bg-black/22 text-white/72 backdrop-blur-sm transition-[background-color,border-color,color,box-shadow] duration-300 hover:border-white/24 hover:bg-black/32 hover:text-white hover:shadow-[0_14px_32px_rgba(0,0,0,0.28)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/30 sm:bottom-8"
        aria-label="Scroll to features"
      >
        <ChevronDown className="hero-scroll-cue-icon h-5 w-5 transition-transform duration-300" />
      </button>
    </section>
  );
}

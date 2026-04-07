import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import type { AnalyticsConsentState } from '../hooks/useAnalyticsConsent';

interface AnalyticsConsentBannerProps {
  consent: AnalyticsConsentState;
  isOpen: boolean;
  onAccept: () => void;
  onDecline: () => void;
  onClose: () => void;
}

export default function AnalyticsConsentBanner({
  consent,
  isOpen,
  onAccept,
  onDecline,
  onClose,
}: AnalyticsConsentBannerProps) {
  if (!isOpen) {
    return null;
  }

  return (
    <aside
      className="fixed inset-x-4 bottom-4 z-50 mx-auto max-w-2xl rounded-2xl border border-white/10 bg-[#090a09]/78 px-4 py-3 text-white shadow-[0_20px_60px_rgba(0,0,0,0.32)] backdrop-blur-xl"
      aria-live="polite"
      aria-label="Analytics consent"
    >
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              className="min-w-0 cursor-help text-left outline-none"
              aria-label="Magpie uses Cloudflare Web Analytics to measure visits and performance."
            >
              <p className="text-sm font-medium text-white">Allow Analytics</p>
              <p className="mt-1 text-xs leading-5 text-[#9d9d9d]">
                It stays off until you opt in and only measures visits and performance.
              </p>
            </button>
          </TooltipTrigger>
          <TooltipContent
            side="top"
            align="start"
            sideOffset={10}
            className="w-[min(20rem,calc(100vw-3rem))] rounded-2xl border border-white/10 bg-[#090a09]/78 px-3 py-2 text-xs leading-5 text-[#b8b8b8] shadow-[0_20px_60px_rgba(0,0,0,0.32)] backdrop-blur-xl"
            style={{
              backdropFilter: 'blur(24px)',
              WebkitBackdropFilter: 'blur(24px)',
            }}
          >
            Magpie uses Cloudflare Web Analytics to measure visits and performance. It stays off
            until you opt in.
          </TooltipContent>
        </Tooltip>

        <div className="flex shrink-0 flex-wrap items-center gap-2 md:justify-end">
          {consent !== 'unknown' ? (
            <button
              type="button"
              onClick={onClose}
              className="rounded-full px-3 py-1.5 text-xs text-[#8f8f8f] transition-colors hover:text-white"
            >
              Close
            </button>
          ) : null}
          <button
            type="button"
            onClick={onDecline}
            className="rounded-full border border-white/10 px-3 py-1.5 text-xs text-[#d4d4d4] transition-colors hover:border-white/20 hover:bg-white/5"
          >
            Decline
          </button>
          <button
            type="button"
            onClick={onAccept}
            className="rounded-full bg-[#3fa37a] px-3 py-1.5 text-xs font-medium text-[#07110d] transition-colors hover:bg-[#4ab98a]"
          >
            Accept
          </button>
        </div>
      </div>
    </aside>
  );
}

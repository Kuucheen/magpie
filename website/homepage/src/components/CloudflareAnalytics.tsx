import { useEffect } from 'react';
import type { AnalyticsConsentState } from '../hooks/useAnalyticsConsent';

const CLOUDFLARE_SCRIPT_ID = 'cloudflare-web-analytics';
const CLOUDFLARE_BEACON_SRC = 'https://static.cloudflareinsights.com/beacon.min.js';

declare global {
  interface Window {
    __cfBeacon?: unknown;
  }
}

interface CloudflareAnalyticsProps {
  consent: AnalyticsConsentState;
  token?: string;
}

export default function CloudflareAnalytics({ consent, token }: CloudflareAnalyticsProps) {
  useEffect(() => {
    if (!token) {
      return;
    }

    const existingScript = document.getElementById(CLOUDFLARE_SCRIPT_ID);

    if (consent !== 'granted') {
      existingScript?.remove();
      return;
    }

    if (existingScript) {
      return;
    }

    const script = document.createElement('script');
    script.id = CLOUDFLARE_SCRIPT_ID;
    script.defer = true;
    script.src = CLOUDFLARE_BEACON_SRC;
    script.setAttribute('data-cf-beacon', JSON.stringify({ token }));

    document.head.appendChild(script);
  }, [consent, token]);

  return null;
}

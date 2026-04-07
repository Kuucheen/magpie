import { useEffect, useState } from 'react';

export type AnalyticsConsentState = 'unknown' | 'granted' | 'denied';

const ANALYTICS_CONSENT_KEY = 'magpie.analytics-consent';

function readStoredConsent(): AnalyticsConsentState {
  if (typeof window === 'undefined') {
    return 'unknown';
  }

  const storedValue = window.localStorage.getItem(ANALYTICS_CONSENT_KEY);

  if (storedValue === 'granted' || storedValue === 'denied') {
    return storedValue;
  }

  return 'unknown';
}

export function useAnalyticsConsent() {
  const [consent, setConsentState] = useState<AnalyticsConsentState>(readStoredConsent);

  useEffect(() => {
    if (typeof window === 'undefined' || consent === 'unknown') {
      return;
    }

    window.localStorage.setItem(ANALYTICS_CONSENT_KEY, consent);
  }, [consent]);

  const setConsent = (value: Exclude<AnalyticsConsentState, 'unknown'>) => {
    setConsentState(value);
  };

  return {
    consent,
    setConsent,
    hasStoredPreference: consent !== 'unknown',
  };
}

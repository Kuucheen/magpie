import {
  ApplicationConfig,
  LOCALE_ID,
  provideBrowserGlobalErrorListeners,
  provideZonelessChangeDetection
} from '@angular/core';
import { provideRouter } from '@angular/router';

import { routes } from './app.routes';
import { provideClientHydration } from '@angular/platform-browser';
import {
  HTTP_INTERCEPTORS,
  provideHttpClient,
  withFetch,
  withInterceptorsFromDi
} from '@angular/common/http';
import {AuthInterceptor} from './services/auth-interceptor.interceptor';
import {definePreset} from '@primeuix/themes';
import Aura from '@primeuix/themes/aura';
import {MessageService} from 'primeng/api';
import {providePrimeNG} from 'primeng/config';
import {provideAnimationsAsync} from '@angular/platform-browser/animations/async';

const CustomTheme = definePreset(Aura, {
  semantic: {
    primary: {
      50: 'var(--theme-primary-50)',
      100: 'var(--theme-primary-100)',
      200: 'var(--theme-primary-200)',
      300: 'var(--theme-primary-300)',
      400: 'var(--theme-primary-400)',
      500: 'var(--theme-primary-500)',
      600: 'var(--theme-primary-600)',
      700: 'var(--theme-primary-700)',
      800: 'var(--theme-primary-800)',
      900: 'var(--theme-primary-900)'
    },
    surface: {
      0:'#0f1113',50:'#15181b',100:'#1a1f23',200:'#21272d',300:'#2a3238',
      400:'#364048',500:'#aeb6bc',600:'#c6cdd3',700:'#dbe2e7',800:'#ecf0f3',900:'#ffffff'
    },
    border: { color:'rgba(255,255,255,.12)' },
    colorScheme: {
      // we always run in dark mode → define only dark
      dark: {
        primary: {
          color: 'var(--theme-primary-500)',
          inverseColor: 'var(--theme-primary-inverse)',
          hoverColor: 'var(--theme-primary-600)',
          activeColor: 'var(--theme-primary-700)'
        },
        highlight: {
          background: 'var(--theme-highlight-background)',
          focusBackground: 'var(--theme-highlight-focus-background)',
          color: 'var(--theme-highlight-color)',
          focusColor: 'var(--theme-highlight-color)'
        },
        text: {
          color: 'var(--theme-text-color)',
          hoverColor: 'var(--theme-text-hover-color)'
        }
      }
    }
  },

  components: {
    button: {
      // global button look
      root: {
        borderRadius: '12px',
        paddingX: '1rem',
        paddingY: '0.625rem',
        gap: '0.5rem',
        transitionDuration: '.2s',
        focusRing: { width: '2px' },
      },
      colorScheme: {
        dark: {
          // FILLED (severity="primary")
          root: {
            primary: {
              background: 'var(--theme-primary-500)',
              hoverBackground: 'var(--theme-primary-600)',
              activeBackground: 'var(--theme-primary-700)',
              color: 'var(--theme-primary-contrast)',
            },
            // neutral/secondary filled (great for “Add Sources” if not outlined)
            secondary: {
              background: 'rgba(255,255,255,.06)',
              color: 'rgba(255,255,255,.92)',
              borderColor: 'rgba(255,255,255,.10)',
              hoverBackground: 'rgba(255,255,255,.10)',
              activeBackground: 'rgba(255,255,255,.12)'
            }
          },

          // OUTLINED
          outlined: {
            primary: {
              color: 'var(--theme-primary-outline-color)',
              borderColor: 'var(--theme-primary-outline-border)',
              hoverBackground: 'var(--theme-primary-outline-hover-bg)',
              activeBackground: 'var(--theme-primary-outline-active-bg)'
            },
            plain: {
              color: 'rgba(236,240,243,.92)',
              borderColor: 'rgba(255,255,255,.14)',
              hoverBackground: 'rgba(255,255,255,.08)'
            }
          },

          // TEXT / LINK
          text: {
            primary: {
              color: 'var(--theme-primary-outline-color)',
              hoverBackground: 'var(--theme-primary-text-hover-bg)'
            },
            plain:   { color: 'rgba(236,240,243,.80)', hoverBackground: 'rgba(255,255,255,.06)' }
          },
          link: {
            color: 'var(--theme-link-color)',
            hoverColor: 'var(--theme-link-hover)'
          }
        }
      }
    }
  }
});

export const appConfig: ApplicationConfig = {
  providers: [
    { provide: LOCALE_ID, useValue: 'de-DE' },
    MessageService,
    provideAnimationsAsync(),
    providePrimeNG({
      theme: {
        preset: CustomTheme,
        options: {
          darkModeSelector: '.dark'
        }
      }
    }),
    provideBrowserGlobalErrorListeners(),
    provideZonelessChangeDetection(),
    provideRouter(routes),
    provideClientHydration(),
    provideHttpClient(withFetch(), withInterceptorsFromDi()),
    { provide: HTTP_INTERCEPTORS, useClass: AuthInterceptor, multi: true }]
};

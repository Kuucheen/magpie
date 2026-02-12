import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'Magpie Docs',
  tagline: 'Documentation for Magpie, the self-hosted proxy manager',
  favicon: 'img/magpie-light-green.ico',

  future: {
    v4: true,
  },

  url: 'https://magpie.tools',
  baseUrl: '/docs/',

  organizationName: 'Kuucheen',
  projectName: 'magpie',

  onBrokenLinks: 'throw',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          routeBasePath: '/',
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/Kuucheen/magpie/tree/master/website/docs/',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/docusaurus-social-card.jpg',
    colorMode: {
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'Magpie Docs',
      logo: {
        alt: 'Magpie Docs Logo',
        src: 'img/magpie-light-green.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'left',
          label: 'Docs',
        },
        {
          href: 'https://github.com/Kuucheen/magpie',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Docs',
          items: [
            {
              label: 'Introduction',
              to: '/',
            },
            {
              label: 'Installation',
              to: '/installation',
            },
          ],
        },
        {
          title: 'Community',
          items: [
            {
              label: 'Discord',
              href: 'https://discord.gg/7FWAGXzhkC',
            },
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'Main Website',
              href: 'https://magpie.tools',
            },
            {
              label: 'GitHub',
              href: 'https://github.com/Kuucheen/magpie',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Magpie Contributors. Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;

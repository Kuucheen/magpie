import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    'intro',
    {
      type: 'category',
      label: 'Getting Started',
      items: [
        'getting-started/quick-start',
        'getting-started/installation',
        'getting-started/first-login',
        'getting-started/local-development',
        'getting-started/updating',
      ],
    },
    {
      type: 'category',
      label: 'User Guide',
      items: [
        'user-guide/navigation',
        'user-guide/dashboard',
        'user-guide/proxies',
        'user-guide/rotating-proxies',
        'user-guide/scraping-sources',
        'user-guide/checker-and-judges',
        'user-guide/account',
        'user-guide/admin',
      ],
    },
    {
      type: 'category',
      label: 'Reference',
      items: [
        'reference/architecture',
        'reference/configuration',
        'reference/environment-variables',
        'reference/runtime-jobs',
        'reference/data-storage',
      ],
    },
    {
      type: 'category',
      label: 'API',
      items: [
        'api/authentication',
        'api/rest-overview',
        'api/rest-auth-and-user',
        'api/rest-proxies',
        'api/rest-rotating-proxies',
        'api/rest-scraping-sources',
        'api/rest-admin-and-system',
        'api/graphql',
        'api/errors',
      ],
    },
    {
      type: 'category',
      label: 'Operations',
      items: [
        'operations/deployment',
        'operations/security',
        'operations/troubleshooting',
        'operations/backup-and-recovery',
      ],
    },
    {
      type: 'category',
      label: 'Contributing',
      items: [
        'contributing/contributing',
        'contributing/testing',
        'contributing/docs',
      ],
    },
    'faq',
  ],
};

export default sidebars;

import gettingStarted from './getting-started.md?raw';
import flagManagement from './flag-management.md?raw';
import sdks from './sdks.md?raw';
import cli from './cli.md?raw';
import uiFeatures from './ui-features.md?raw';

export type DocCategory = 'Getting Started' | 'Configure & Manage' | 'Integrate';

export type DocEntry = {
  slug: string;
  title: string;
  category: DocCategory;
  summary: string;
  icon: string;
  source: string;
};

export const docsManifest: readonly DocEntry[] = [
  {
    slug: 'getting-started',
    title: 'Getting Started',
    category: 'Getting Started',
    summary: 'Zero to your first flag in production — choose your platform and ship.',
    icon: 'rocket_launch',
    source: gettingStarted,
  },
  {
    slug: 'ui-features',
    title: 'UI Features',
    category: 'Configure & Manage',
    summary: 'A tour of every page in the dashboard: projects, apps, flags, deployments.',
    icon: 'dashboard',
    source: uiFeatures,
  },
  {
    slug: 'flag-management',
    title: 'Flag Management',
    category: 'Configure & Manage',
    summary: 'Create flags, configure targeting rules, and manage the flag lifecycle.',
    icon: 'toggle_on',
    source: flagManagement,
  },
  {
    slug: 'sdks',
    title: 'SDKs',
    category: 'Integrate',
    summary: 'Seven first-party SDKs: instantiate a client, evaluate flags, register dispatch.',
    icon: 'extension',
    source: sdks,
  },
  {
    slug: 'cli',
    title: 'CLI',
    category: 'Integrate',
    summary:
      'Manage organizations, projects, applications, deployments, releases, and flags from the terminal.',
    icon: 'terminal',
    source: cli,
  },
] as const;

export const docCategoryOrder: readonly DocCategory[] = [
  'Getting Started',
  'Configure & Manage',
  'Integrate',
] as const;

export function findDoc(slug: string): DocEntry | undefined {
  return docsManifest.find((d) => d.slug === slug);
}

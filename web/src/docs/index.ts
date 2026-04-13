import gettingStarted from './getting-started.md?raw';
import sdks from './sdks.md?raw';
import cli from './cli.md?raw';
import uiFeatures from './ui-features.md?raw';

export type DocEntry = {
  slug: string;
  title: string;
  source: string;
};

export const docsManifest: readonly DocEntry[] = [
  { slug: 'getting-started', title: 'Getting Started', source: gettingStarted },
  { slug: 'sdks',            title: 'SDKs',            source: sdks },
  { slug: 'cli',             title: 'CLI',             source: cli },
  { slug: 'ui-features',     title: 'UI Features',     source: uiFeatures },
] as const;

export function findDoc(slug: string): DocEntry | undefined {
  return docsManifest.find((d) => d.slug === slug);
}

import { spawn, type ChildProcess } from 'child_process';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { fileURLToPath } from 'url';
import { type Browser, type BrowserContext, type Page } from '@playwright/test';

// ESM-safe __dirname. Playwright loads specs as ES modules, so the
// CommonJS `__dirname` global is not available.
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

export interface Observation {
  flagKey: string;
  value: unknown;
  ts: number;
}

export interface ProbeContext {
  apiUrl: string;
  apiKey: string;
  project: string;
  application: string;
  environment: string;
  flagKeys: string[];
  /**
   * User context passed to flag evaluation. Both probes accept the
   * { id, attributes } shape used by the React SDK's UserContext type.
   */
  user: { id: string; attributes?: Record<string, string> };
}

export interface Probe {
  name: string;
  /** Synchronous read for the Node probe; throws for the React probe (use observationsAsync). */
  observations(): Observation[];
  stop(): Promise<void>;
}

export interface ReactProbe extends Probe {
  page: Page;
  browserContext: BrowserContext;
  observationsAsync(): Promise<Observation[]>;
}

const PROBE_DIR = path.resolve(__dirname, '../sdk-probes/node-probe');

export async function startNodeProbe(ctx: ProbeContext): Promise<Probe> {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ds-node-probe-'));
  const obsFile = path.join(dir, 'observations.jsonl');
  fs.writeFileSync(obsFile, '');

  const child: ChildProcess = spawn('node', ['index.js'], {
    cwd: PROBE_DIR,
    env: {
      ...process.env,
      OBSERVATIONS_FILE: obsFile,
      DS_API_URL: ctx.apiUrl,
      DS_API_KEY: ctx.apiKey,
      DS_PROJECT: ctx.project,
      DS_APPLICATION: ctx.application,
      DS_ENVIRONMENT: ctx.environment,
      DS_FLAG_KEYS: ctx.flagKeys.join(','),
      // Node SDK's EvaluationContext shape: { userId, attributes }.
      // The driver's canonical ProbeContext.user uses { id, attributes }
      // (matching the React SDK's UserContext) — translate here.
      DS_CONTEXT_JSON: JSON.stringify({
        userId: ctx.user.id,
        attributes: ctx.user.attributes,
      }),
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  child.stderr?.on('data', (b: Buffer) => {
    process.stderr.write(`[node-probe] ${b}`);
  });

  return {
    name: 'node',
    observations(): Observation[] {
      const raw = fs.readFileSync(obsFile, 'utf8');
      return raw
        .split('\n')
        .filter(Boolean)
        .map((line) => JSON.parse(line) as Observation);
    },
    async stop(): Promise<void> {
      child.kill('SIGTERM');
      await new Promise<void>((resolve) => {
        if (child.exitCode !== null) return resolve();
        child.once('exit', () => resolve());
      });
      fs.rmSync(dir, { recursive: true, force: true });
    },
  };
}

export async function startReactProbe(
  browser: Browser,
  harnessBaseUrl: string,
  ctx: ProbeContext,
): Promise<ReactProbe> {
  const bctx = await browser.newContext();
  const page = await bctx.newPage();
  page.on('console', (msg) => {
    process.stderr.write(`[react-probe:${msg.type()}] ${msg.text()}\n`);
  });
  page.on('pageerror', (err) => {
    process.stderr.write(`[react-probe:pageerror] ${err.message}\n`);
  });
  const qs = new URLSearchParams({
    apiUrl: ctx.apiUrl,
    apiKey: ctx.apiKey,
    project: ctx.project,
    application: ctx.application,
    environment: ctx.environment,
    flagKeys: ctx.flagKeys.join(','),
    context: JSON.stringify(ctx.user),
  }).toString();
  await page.goto(`${harnessBaseUrl}/?${qs}`);

  const probe: ReactProbe = {
    name: 'react',
    page,
    browserContext: bctx,
    observations(): Observation[] {
      throw new Error('react probe: use observationsAsync() instead');
    },
    async observationsAsync(): Promise<Observation[]> {
      return page.evaluate(() => {
        const obs = (window as unknown as { __ds_observations?: Observation[] }).__ds_observations;
        return obs ?? [];
      });
    },
    async stop(): Promise<void> {
      await bctx.close();
    },
  };
  return probe;
}

export interface WaitForValueOptions {
  timeoutMs?: number;
  /** For react probes, pass () => probe.observationsAsync(). Default reads probe.observations() synchronously. */
  getObservations?: () => Promise<Observation[]> | Observation[];
}

export async function waitForValue(
  probe: Probe,
  flagKey: string,
  expected: unknown,
  opts: WaitForValueOptions = {},
): Promise<number> {
  const timeoutMs = opts.timeoutMs ?? 3_000;
  const deadline = Date.now() + timeoutMs;
  const start = Date.now();
  const expectedJson = JSON.stringify(expected);

  let last: Observation[] = [];
  while (Date.now() < deadline) {
    const observed = opts.getObservations
      ? await opts.getObservations()
      : probe.observations();
    last = observed;
    const match = observed.find(
      (o) => o.flagKey === flagKey && JSON.stringify(o.value) === expectedJson,
    );
    if (match) return Date.now() - start;
    await new Promise((r) => setTimeout(r, 25));
  }
  throw new Error(
    `probe="${probe.name}" flag="${flagKey}" never observed value=${expectedJson} within ${timeoutMs}ms.\n` +
      `observations: ${JSON.stringify(last, null, 2)}`,
  );
}

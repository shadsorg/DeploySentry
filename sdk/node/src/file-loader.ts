import * as fs from 'fs';
import * as path from 'path';
import * as yaml from 'js-yaml';
import type { FlagConfig } from './types';

const DEFAULT_PATH = '.deploysentry/flags.yaml';

export function loadFlagConfig(filePath?: string): FlagConfig {
  const resolved = path.resolve(filePath ?? DEFAULT_PATH);
  if (!fs.existsSync(resolved)) {
    throw new Error(`Flag config file not found: ${resolved}`);
  }
  const content = fs.readFileSync(resolved, 'utf-8');
  const parsed = yaml.load(content) as FlagConfig;
  if (!parsed || typeof parsed !== 'object' || !parsed.version || !parsed.flags) {
    throw new Error(`Invalid flag config file: ${resolved}`);
  }
  return parsed;
}

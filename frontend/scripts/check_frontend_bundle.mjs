import { readdir, stat } from 'node:fs/promises';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';

export const ASSETS_DIR = join(process.cwd(), 'dist', 'assets');
export const MAX_JS_CHUNK_BYTES = 600 * 1024;
export const MAX_TOTAL_JS_BYTES = 1700 * 1024;

export async function collectJsAssets(assetsDir = ASSETS_DIR) {
  const entries = await readdir(assetsDir);
  const jsAssets = [];

  for (const name of entries) {
    if (!name.endsWith('.js')) continue;
    const file = join(assetsDir, name);
    const fileStat = await stat(file);
    jsAssets.push({ name, bytes: fileStat.size });
  }

  return jsAssets;
}

export function evaluateBundleBudget(
  jsAssets,
  {
    maxJsChunkBytes = MAX_JS_CHUNK_BYTES,
    maxTotalJsBytes = MAX_TOTAL_JS_BYTES,
  } = {},
) {
  if (jsAssets.length === 0) {
    throw new Error('No JavaScript assets found in dist/assets');
  }

  const assets = [...jsAssets].sort((a, b) => b.bytes - a.bytes);
  const largest = assets[0];
  const totalBytes = assets.reduce((sum, asset) => sum + asset.bytes, 0);

  const violations = [];
  if (largest.bytes > maxJsChunkBytes) {
    violations.push(`largest JS chunk ${largest.name} is ${formatBytes(largest.bytes)}, budget ${formatBytes(maxJsChunkBytes)}`);
  }
  if (totalBytes > maxTotalJsBytes) {
    violations.push(`total JS is ${formatBytes(totalBytes)}, budget ${formatBytes(maxTotalJsBytes)}`);
  }

  return { assets, largest, totalBytes, violations };
}

export async function main({
  assetsDir = ASSETS_DIR,
  log = console.log,
} = {}) {
  const result = evaluateBundleBudget(await collectJsAssets(assetsDir));
  log(
    `Bundle budget: largest=${result.largest.name} ${formatBytes(result.largest.bytes)}, `
      + `total_js=${formatBytes(result.totalBytes)}, chunks=${result.assets.length}`,
  );
  if (result.violations.length > 0) {
    throw new Error(`Bundle budget exceeded: ${result.violations.join('; ')}`);
  }
  return result;
}

export function formatBytes(bytes) {
  return `${(bytes / 1024).toFixed(1)} KiB`;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await main();
}

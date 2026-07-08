import assert from 'node:assert/strict';
import { mkdtempSync, mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import {
  checkFrontendRoutes,
  EXPECTED_ROUTES,
} from '../scripts/check_frontend_routes.mjs';
import {
  collectJsAssets,
  evaluateBundleBudget,
  formatBytes,
} from '../scripts/check_frontend_bundle.mjs';


const ROOT = fileURLToPath(new URL('..', import.meta.url));

test('route gate passes current App.tsx', () => {
  const result = checkFrontendRoutes({
    appFile: join(ROOT, 'src', 'App.tsx'),
    pagesDir: join(ROOT, 'src', 'pages'),
  });

  assert.deepEqual(result.failures, []);
  assert.equal(result.routeConfig.size, EXPECTED_ROUTES.size);
  assert.equal(result.renderedRoutes.get('*'), 'Navigate');
});

test('route gate detects wildcard redirect regression in synthetic app', () => {
  const expectedRoutes = new Map([
    ['/dashboard', { component: 'DashboardPage', roles: ['admin'] }],
  ]);
  const source = `
import { lazy } from 'react';
import { Route, Routes } from 'react-router-dom';

const DashboardPage = lazy(() => import('./pages/DashboardPage'));

const routes = [
  { path: '/dashboard', label: 'Dashboard', roles: ['admin'] },
];

export default function App() {
  return (
    <Routes>
      <Route path="/dashboard" element={<DashboardPage />} />
    </Routes>
  );
}
`;

  const result = checkFrontendRoutes({
    appFile: 'App.tsx',
    pagesDir: '/virtual/pages',
    expectedRoutes,
    readFile: () => source,
    exists: () => true,
  });

  assert.ok(result.failures.includes('wildcard route must redirect with Navigate'));
});

test('bundle gate helpers detect budget violations', () => {
  const result = evaluateBundleBudget(
    [
      { name: 'vendor-react.js', bytes: 700 * 1024 },
      { name: 'vendor-map.js', bytes: 1101 * 1024 },
    ],
    {
      maxJsChunkBytes: 600 * 1024,
      maxTotalJsBytes: 1700 * 1024,
    },
  );

  assert.equal(formatBytes(1024), '1.0 KiB');
  assert.equal(result.largest.name, 'vendor-map.js');
  assert.ok(result.violations[0].includes('largest JS chunk vendor-map.js'));
  assert.ok(result.violations[1].includes('total JS is 1801.0 KiB'));
});

test('bundle gate asset collector only returns js assets', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'shipsystem-frontend-assets-'));
  mkdirSync(dir, { recursive: true });
  writeFileSync(join(dir, 'alpha.js'), 'console.log("a")', 'utf8');
  writeFileSync(join(dir, 'beta.css'), 'body {}', 'utf8');
  writeFileSync(join(dir, 'gamma.js'), 'console.log("b")', 'utf8');

  const assets = await collectJsAssets(dir);

  assert.deepEqual(
    assets.map((item) => item.name).sort(),
    ['alpha.js', 'gamma.js'],
  );
});

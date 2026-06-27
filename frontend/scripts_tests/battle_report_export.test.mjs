import assert from 'node:assert/strict';
import test from 'node:test';
import ts from 'typescript';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';


const ROOT = fileURLToPath(new URL('..', import.meta.url));
const source = readFileSync(join(ROOT, 'src', 'utils', 'battleReportExport.ts'), 'utf8');
const transpiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2022,
  },
}).outputText;

const battleReportExport = await import(`data:text/javascript;charset=utf-8,${encodeURIComponent(transpiled)}`);

const sampleReport = {
  session: {
    id: 1,
    sessionId: 'battle-1',
    name: 'Demo Battle',
    scenarioCode: 'open-water-duel',
    status: 'stopped',
    startedAt: '2026-06-24T10:00:00.000Z',
    stoppedAt: '2026-06-24T10:08:00.000Z',
    lastScanAt: '2026-06-24T10:07:59.000Z',
    createdAt: '2026-06-24T10:00:00.000Z',
    updatedAt: '2026-06-24T10:08:00.000Z',
  },
  winner: 'blue',
  firedCount: 3,
  hitCount: 2,
  destroyedCount: 1,
  damageRanking: [
    { unitId: 'u-1', name: 'Alpha', side: 'blue', maxHp: 100, hp: 80, damageTaken: 20, status: 'active' },
  ],
  keyEvents: [
    {
      id: 1,
      sessionId: 'battle-1',
      eventId: 'e-1',
      type: 'WEAPON_FIRED',
      severity: 'INFO',
      message: 'Weapon launched',
      sourceUnitId: 'u-1',
      targetUnitId: 'u-2',
      occurredAt: '2026-06-24T10:01:00.000Z',
      createdAt: '2026-06-24T10:01:00.000Z',
    },
  ],
};

const sampleTimeline = [
  {
    tick: 1,
    snapshotTime: '2026-06-24T10:01:00.000Z',
    eventCount: 1,
    events: [
      {
        type: 'WEAPON_FIRED',
        severity: 'INFO',
        message: 'Weapon launched',
        sourceUnitId: 'u-1',
        targetUnitId: 'u-2',
      },
    ],
  },
];

test('battle report filename is stable and sanitized', () => {
  assert.equal(
    battleReportExport.battleReportDownloadFilename('battle 1/../x', 'csv'),
    'battle-battle-1-x-report.csv',
  );
});

test('battle report export renders json csv and html', () => {
  const jsonExport = battleReportExport.buildBattleReportExport(sampleReport, sampleTimeline, 'json');
  const csvExport = battleReportExport.buildBattleReportExport(sampleReport, sampleTimeline, 'csv');
  const htmlExport = battleReportExport.buildBattleReportExport(sampleReport, sampleTimeline, 'html');

  assert.equal(jsonExport.mimeType, 'application/json');
  assert.match(jsonExport.text, /"sessionId": "battle-1"/);

  assert.equal(csvExport.mimeType, 'text/csv');
  assert.match(csvExport.text, /summary,session_id,battle-1/);
  assert.match(csvExport.text, /damage,1,u-1,Alpha,blue,100,80,20,active/);
  assert.match(csvExport.text, /timeline,1,2026-06-24T10:01:00.000Z,1,Weapon launched/);
  assert.match(csvExport.text, /event,2026-06-24T10:01:00.000Z,WEAPON_FIRED,INFO,Weapon launched,u-1,u-2/);

  assert.equal(htmlExport.mimeType, 'text/html');
  assert.match(htmlExport.text, /战报导出/);
  assert.match(htmlExport.text, /Weapon launched/);
  assert.match(htmlExport.text, /Demo Battle/);
});

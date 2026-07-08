import type { BattleEvent, BattleReport, BattleTimelineItem } from '../types';

export type BattleReportExportFormat = 'json' | 'csv' | 'html';

export function battleReportDownloadFilename(sessionId: string, format: BattleReportExportFormat) {
  const safeSessionId = sanitizeFilenamePart(sessionId || 'battle-report');
  return `battle-${safeSessionId}-report.${format}`;
}

export function buildBattleReportExport(report: BattleReport, timeline: BattleTimelineItem[], format: BattleReportExportFormat) {
  if (format === 'json') {
    return {
      mimeType: 'application/json',
      text: JSON.stringify({ report, timeline }, null, 2),
    };
  }

  if (format === 'csv') {
    return {
      mimeType: 'text/csv',
      text: buildCsv(report, timeline),
    };
  }

  return {
    mimeType: 'text/html',
    text: buildHtml(report, timeline),
  };
}

function buildCsv(report: BattleReport, timeline: BattleTimelineItem[]) {
  const lines: string[] = [];
  lines.push('section,key,value');
  lines.push(csvRow('summary', 'session_id', report.session.sessionId));
  lines.push(csvRow('summary', 'session_name', report.session.name));
  lines.push(csvRow('summary', 'winner', report.winner));
  lines.push(csvRow('summary', 'fired_count', report.firedCount));
  lines.push(csvRow('summary', 'hit_count', report.hitCount));
  lines.push(csvRow('summary', 'destroyed_count', report.destroyedCount));
  lines.push(csvRow('summary', 'damage_ranking_count', report.damageRanking.length));
  lines.push(csvRow('summary', 'key_events_count', report.keyEvents.length));
  lines.push(csvRow('summary', 'timeline_frames', timeline.length));

  lines.push('');
  lines.push('section,rank,unit_id,name,side,max_hp,hp,damage_taken,status');
  report.damageRanking.forEach((item, index) => {
    lines.push(
      [
        'damage',
        String(index + 1),
        item.unitId,
        item.name,
        item.side,
        String(item.maxHp),
        String(item.hp),
        String(item.damageTaken),
        item.status,
      ]
        .map(csvEscape)
        .join(','),
    );
  });

  lines.push('');
  lines.push('section,tick,snapshot_time,event_count,event_messages');
  timeline.forEach((item) => {
    lines.push(
      [
        'timeline',
        String(item.tick),
        item.snapshotTime,
        String(item.eventCount),
        item.events.map((event) => event.message).join(' | '),
      ]
        .map(csvEscape)
        .join(','),
    );
  });

  lines.push('');
  lines.push('section,event_time,type,severity,message,source_unit,target_unit');
  report.keyEvents.forEach((event) => {
    lines.push(csvEventRow(event));
  });

  return lines.join('\n');
}

function buildHtml(report: BattleReport, timeline: BattleTimelineItem[]) {
  const damageRows = report.damageRanking
    .map(
      (item, index) => `
        <tr>
          <td>${index + 1}</td>
          <td>${escapeHtml(item.name)}</td>
          <td>${escapeHtml(item.side)}</td>
          <td>${escapeHtml(String(item.maxHp))}</td>
          <td>${escapeHtml(String(item.hp))}</td>
          <td>${escapeHtml(String(item.damageTaken))}</td>
          <td>${escapeHtml(item.status)}</td>
        </tr>`,
    )
    .join('');

  const eventRows = report.keyEvents
    .map(
      (event) => `
        <tr>
          <td>${escapeHtml(formatTime(event.occurredAt))}</td>
          <td>${escapeHtml(event.type)}</td>
          <td>${escapeHtml(event.severity)}</td>
          <td>${escapeHtml(event.message)}</td>
          <td>${escapeHtml(event.sourceUnitId)}</td>
          <td>${escapeHtml(event.targetUnitId)}</td>
        </tr>`,
    )
    .join('');

  const timelineRows = timeline
    .map(
      (item) => `
        <tr>
          <td>${escapeHtml(String(item.tick))}</td>
          <td>${escapeHtml(formatTime(item.snapshotTime))}</td>
          <td>${escapeHtml(String(item.eventCount))}</td>
          <td>${escapeHtml(item.events.map((event) => event.message).join(' | '))}</td>
        </tr>`,
    )
    .join('');

  return `<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Battle Report ${escapeHtml(report.session.sessionId)}</title>
    <style>
      body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 24px; color: #10233f; background: #f6f8fb; }
      h1, h2 { margin: 0 0 12px; }
      .card { background: #fff; border: 1px solid #dbe3ee; border-radius: 14px; padding: 16px; margin: 16px 0; box-shadow: 0 8px 24px rgba(16, 35, 63, 0.06); }
      .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 12px; }
      .stat { background: #f9fbff; border: 1px solid #e4ebf5; border-radius: 12px; padding: 12px; }
      .stat span { display: block; color: #58708c; font-size: 12px; margin-bottom: 4px; }
      table { width: 100%; border-collapse: collapse; }
      th, td { border-bottom: 1px solid #e6edf5; padding: 8px 10px; text-align: left; vertical-align: top; }
      th { background: #f4f7fb; font-size: 12px; text-transform: uppercase; letter-spacing: .04em; color: #52637a; }
      .notice { color: #5f6f82; font-size: 13px; }
    </style>
  </head>
  <body>
    <h1>战报导出</h1>
    <div class="notice">仅用于训练/演练复盘，不构成任何实战建议。</div>
    <div class="card">
      <h2>摘要</h2>
      <div class="stats">
        <div class="stat"><span>会话</span><strong>${escapeHtml(report.session.name)}</strong></div>
        <div class="stat"><span>会话 ID</span><strong>${escapeHtml(report.session.sessionId)}</strong></div>
        <div class="stat"><span>胜负</span><strong>${escapeHtml(report.winner)}</strong></div>
        <div class="stat"><span>发射数</span><strong>${report.firedCount}</strong></div>
        <div class="stat"><span>命中数</span><strong>${report.hitCount}</strong></div>
        <div class="stat"><span>摧毁数</span><strong>${report.destroyedCount}</strong></div>
      </div>
    </div>
    <div class="card">
      <h2>损伤排行</h2>
      <table>
        <thead>
          <tr><th>#</th><th>名称</th><th>阵营</th><th>最大血量</th><th>当前血量</th><th>伤害</th><th>状态</th></tr>
        </thead>
        <tbody>${damageRows || '<tr><td colspan="7">暂无损伤数据</td></tr>'}</tbody>
      </table>
    </div>
    <div class="card">
      <h2>关键事件</h2>
      <table>
        <thead>
          <tr><th>时间</th><th>类型</th><th>级别</th><th>消息</th><th>来源</th><th>目标</th></tr>
        </thead>
        <tbody>${eventRows || '<tr><td colspan="6">暂无关键事件</td></tr>'}</tbody>
      </table>
    </div>
    <div class="card">
      <h2>时间轴摘要</h2>
      <table>
        <thead>
          <tr><th>Tick</th><th>时间</th><th>事件数</th><th>摘要</th></tr>
        </thead>
        <tbody>${timelineRows || '<tr><td colspan="4">暂无时间轴数据</td></tr>'}</tbody>
      </table>
    </div>
  </body>
</html>`;
}

function csvEventRow(event: BattleEvent) {
  return [
    'event',
    formatTime(event.occurredAt),
    event.type,
    event.severity,
    event.message,
    event.sourceUnitId,
    event.targetUnitId,
  ]
    .map(csvEscape)
    .join(',');
}

function csvRow(section: string, key: string, value: string | number) {
  return [section, key, String(value)].map(csvEscape).join(',');
}

function csvEscape(value: string) {
  const text = String(value ?? '');
  if (/[",\n\r]/.test(text)) {
    return `"${text.replace(/"/g, '""')}"`;
  }
  return text;
}

function escapeHtml(value: string) {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function formatTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toISOString();
}

function sanitizeFilenamePart(value: string) {
  return value
    .trim()
    .replace(/[^a-zA-Z0-9._-]+/g, "-")
    .replace(/\.{2,}/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-+|-+$/g, "")
    .slice(0, 80);
}

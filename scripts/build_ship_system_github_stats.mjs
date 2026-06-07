import fs from "node:fs/promises";
import path from "node:path";
import { SpreadsheetFile, Workbook } from "@oai/artifact-tool";

const outputDir = path.resolve("outputs", "ship_system_github_stats_20260605");
const cachePath = path.join(outputDir, "github_api_raw.json");
const workbookPath = path.join(outputDir, "github_ship_systems_stats.xlsx");

const searchSpecs = [
  { label: "中文-船舶管理系统", query: "船舶 管理 系统" },
  { label: "中文-船舶监造系统", query: "船舶 监造 系统" },
  { label: "中文-船舶维保系统", query: "船舶 维保 管理 系统" },
  { label: "英文-Ship Management", query: "\"ship management system\"" },
  { label: "英文-Vessel Management", query: "\"vessel management system\"" },
  { label: "英文-Marine Vessel Management", query: "\"marine vessel management system\"" },
  { label: "船舶监控", query: "\"vessel monitoring system\"" },
  { label: "船舶跟踪", query: "\"vessel tracking\"" },
  { label: "Ship Tracking", query: "\"ship tracking\"" },
  { label: "AIS-Ship Tracking", query: "\"AIS ship tracking\"" },
  { label: "AIS-Vessel Tracking", query: "\"AIS vessel tracking\"" },
  { label: "AIS-自动识别系统", query: "\"automatic identification system\" ship" },
  { label: "海事导航", query: "\"marine navigation system\"" },
  { label: "海事船队管理", query: "\"maritime fleet management\"" },
  { label: "港口船舶管理", query: "\"port ship management\"" },
  { label: "Topic-AIS-Ship", query: "topic:ais ship" },
  { label: "Topic-Vessel-Tracking", query: "topic:vessel-tracking" },
  { label: "Topic-Ship-Tracking", query: "topic:ship-tracking" },
];

const headers = {
  "Accept": "application/vnd.github+json",
  "User-Agent": "codex-ship-system-research",
  "X-GitHub-Api-Version": "2022-11-28",
};

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function exists(filePath) {
  try {
    await fs.access(filePath);
    return true;
  } catch {
    return false;
  }
}

async function fetchJsonWithRetry(url, attempt = 1) {
  const response = await fetch(url, { headers });
  if ((response.status === 403 || response.status === 429) && attempt <= 3) {
    const remaining = response.headers.get("x-ratelimit-remaining");
    const reset = response.headers.get("x-ratelimit-reset");
    if (remaining === "0" && reset) {
      const waitMs = Math.min(
        Math.max(Number(reset) * 1000 - Date.now() + 1500, 1500),
        70000,
      );
      console.log(`GitHub rate limit reached; waiting ${Math.ceil(waitMs / 1000)}s...`);
      await sleep(waitMs);
      return fetchJsonWithRetry(url, attempt + 1);
    }
  }
  if (!response.ok) {
    const body = await response.text();
    throw new Error(`GitHub request failed (${response.status}): ${body.slice(0, 500)}`);
  }
  return response.json();
}

async function loadGithubData() {
  await fs.mkdir(outputDir, { recursive: true });
  if ((await exists(cachePath)) && process.env.REFRESH_GITHUB !== "1") {
    return JSON.parse(await fs.readFile(cachePath, "utf8"));
  }

  const merged = new Map();
  const searchStats = [];
  for (const spec of searchSpecs) {
    const url = new URL("https://api.github.com/search/repositories");
    const scopedQuery = spec.query.includes("topic:")
      ? spec.query
      : `${spec.query} in:name,description`;
    url.searchParams.set("q", scopedQuery);
    url.searchParams.set("sort", "stars");
    url.searchParams.set("order", "desc");
    url.searchParams.set("per_page", "25");

    console.log(`Searching GitHub: ${spec.label}`);
    const data = await fetchJsonWithRetry(url);
    searchStats.push({
      label: spec.label,
      query: spec.query,
      total_count: data.total_count,
      returned_count: data.items?.length ?? 0,
      api_url: url.toString(),
    });

    for (const item of data.items ?? []) {
      const current = merged.get(item.id);
      if (current) {
        current.matched_searches.push(spec.label);
      } else {
        merged.set(item.id, { ...item, matched_searches: [spec.label] });
      }
    }
    await sleep(1200);
  }

  const payload = {
    retrieved_at: new Date().toISOString(),
    api_source: "https://api.github.com/search/repositories",
    search_stats: searchStats,
    repositories: Array.from(merged.values()),
  };
  await fs.writeFile(cachePath, JSON.stringify(payload, null, 2), "utf8");
  return payload;
}

function textFor(repo) {
  return [
    repo.full_name,
    repo.name,
    repo.description,
    repo.homepage,
    ...(repo.topics ?? []),
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
}

function analyzeRepo(repo, retrievedAt) {
  const text = textFor(repo);
  const conciseText = [
    repo.full_name,
    repo.name,
    repo.description,
    ...(repo.topics ?? []),
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
  const notes = [];
  let semanticScore = 0;

  const signals = [
    { name: "AIS", regex: /\bais\b|automatic identification system/i, score: 6 },
    { name: "vessel", regex: /\bvessels?\b/i, score: 5 },
    { name: "ship", regex: /\bships?\b|\bshipboard\b/i, score: 4 },
    { name: "maritime", regex: /\bmaritime\b|\bnautical\b/i, score: 4 },
    { name: "marine", regex: /\bmarine\b/i, score: 3 },
    { name: "port/harbor", regex: /\bports?\b|\bharbou?rs?\b/i, score: 3 },
    { name: "fleet", regex: /\bfleet\b/i, score: 2 },
    { name: "tracking/monitoring", regex: /\btracking\b|\bmonitoring\b|\btraffic\b|\bsurveillance\b/i, score: 2 },
    { name: "navigation/routing", regex: /\bnavigation\b|\brouting\b|\broute\b|\bautopilot\b/i, score: 2 },
    { name: "management", regex: /\bmanagement\b|\bmanager\b|\boperations?\b/i, score: 1 },
    { name: "中文船舶/海事", regex: /船舶|船队|航运|海事|港口|码头|船载|船只/i, score: 5 },
  ];

  for (const signal of signals) {
    if (signal.regex.test(text)) {
      semanticScore += signal.score;
      notes.push(signal.name);
    }
  }

  const hasStrongMaritime =
    /\bais\b|automatic identification system|\bvessels?\b|\bships?\b|\bmaritime\b|\bmarine\b|\bports?\b|\bharbou?rs?\b|船舶|船队|航运|海事|港口|码头/i.test(
      text,
    );
  const logisticsOnly =
    /\bshipment\b|\bparcel\b|\bcourier\b|\bdelivery\b|\be-?commerce\b|\bwoocommerce\b|\bshopify\b|\bwarehouse\b/i.test(
      text,
    ) && !hasStrongMaritime;

  if (logisticsOnly) {
    semanticScore -= 8;
    notes.push("剔除信号: 物流/包裹运输");
  }
  if (repo.archived) {
    semanticScore -= 1;
    notes.push("已归档");
  }

  const strongMaritime =
    /\bais\b|automatic identification system|\bvms\b|\bvessels?\b|\bships?\b|\bboats?\b|\bmaritime\b|\bmarine\b|\bports?\b|\bharbou?rs?\b|船舶|船队|渔船|航运|海事|港口|码头/i.test(
      conciseText,
    );
  const systemIntent =
    /\b(system|platform|server|dashboard|tracker|tracking|monitoring|management|navigator|navigation|receiver|feeder|decoder|database|pipeline|visualization|simulator)\b|系统|平台|管理|监控|跟踪|导航|接收|识别|数据库|维保|监造|仿真/i.test(
      conciseText,
    );
  const precisePhrase =
    /ship management system|vessel management system|marine vessel management system|vessel monitoring system|vessel tracking|ship tracking|ais ship tracking|ais vessel tracking|marine navigation system|port ship management|automatic identification system|船舶.{0,12}(系统|管理|监控|跟踪|监造|维保)|渔船.{0,12}(识别|监控|轨迹)|港口.{0,12}船舶/i.test(
      conciseText,
    );
  const listOrDatasetOnly =
    /\b(awesome|list|books?|ebooks?|paper|papers|arxiv|datasets?|resources|collection|public apis?|public-apis|osint)\b|书籍|电子书|题库|论文|目录|合集|公共 api/i.test(
      conciseText,
    ) && !precisePhrase;

  const includeCandidate = strongMaritime && (systemIntent || precisePhrase) && !listOrDatasetOnly;
  if (!includeCandidate) {
    notes.push("未入选: 相关性不足或偏列表/资料库");
  }

  const updatedAt = repo.updated_at ? new Date(repo.updated_at) : null;
  const retrieved = new Date(retrievedAt);
  const daysSinceUpdate = updatedAt
    ? Math.round((retrieved.getTime() - updatedAt.getTime()) / 86400000)
    : null;
  const activityScore =
    daysSinceUpdate == null ? 0 : daysSinceUpdate <= 365 ? 2 : daysSinceUpdate <= 1095 ? 1 : 0;
  const popularityScore = Math.min(5, Math.log10((repo.stargazers_count ?? 0) + 1) * 1.8);
  const relevanceScore = semanticScore + activityScore + popularityScore;

  let category = "其他船舶系统";
  if (/\bais\b|automatic identification system/i.test(text)) {
    category = "AIS/船舶跟踪";
  } else if (/\btracking\b|\bmonitoring\b|\btraffic\b|\bsurveillance\b/i.test(text)) {
    category = "监控/交通态势";
  } else if (/\bnavigation\b|\brouting\b|\broute\b|\bautopilot\b/i.test(text)) {
    category = "导航/航线";
  } else if (/\bports?\b|\bharbou?rs?\b|港口|码头/i.test(text)) {
    category = "港口/泊位管理";
  } else if (/\bmanagement\b|\bmanager\b|\bfleet\b|\boperations?\b|船队|管理/i.test(text)) {
    category = "管理/运营";
  } else if (/\bsimulation\b|\bsimulator\b|\bcontrol\b|\bdynamics\b/i.test(text)) {
    category = "仿真/控制";
  }

  return {
    ...repo,
    category,
    semanticScore,
    relevanceScore,
    daysSinceUpdate,
    includeCandidate,
    relevance_notes: Array.from(new Set(notes)).join("; "),
  };
}

function selectRepositories(payload) {
  const analyzed = payload.repositories.map((repo) => analyzeRepo(repo, payload.retrieved_at));
  const filtered = analyzed
    .filter((repo) => repo.includeCandidate && repo.semanticScore >= 4)
    .sort((a, b) => {
      if ((b.stargazers_count ?? 0) !== (a.stargazers_count ?? 0)) {
        return (b.stargazers_count ?? 0) - (a.stargazers_count ?? 0);
      }
      return b.relevanceScore - a.relevanceScore;
    });

  const top = filtered.slice(0, 50);
  return top.map((repo, index) => ({ ...repo, rank: index + 1 }));
}

function summarizeBy(rows, keyFn) {
  const map = new Map();
  for (const row of rows) {
    const key = keyFn(row) || "未标注";
    const current =
      map.get(key) ?? {
        key,
        count: 0,
        stars: 0,
        forks: 0,
        openIssues: 0,
        active12m: 0,
        topRepo: "",
        topStars: -1,
      };
    current.count += 1;
    current.stars += row.stargazers_count ?? 0;
    current.forks += row.forks_count ?? 0;
    current.openIssues += row.open_issues_count ?? 0;
    if ((row.daysSinceUpdate ?? Infinity) <= 365) current.active12m += 1;
    if ((row.stargazers_count ?? 0) > current.topStars) {
      current.topStars = row.stargazers_count ?? 0;
      current.topRepo = row.full_name;
    }
    map.set(key, current);
  }
  return Array.from(map.values()).sort((a, b) => b.count - a.count || b.stars - a.stars);
}

function addTitle(sheet, title, subtitle) {
  sheet.getRange("A1:F1").merge();
  sheet.getRange("A1").values = [[title]];
  sheet.getRange("A1").format = {
    fill: "#17324D",
    font: { bold: true, color: "#FFFFFF", size: 16 },
  };
  sheet.getRange("A2:F2").merge();
  sheet.getRange("A2").values = [[subtitle]];
  sheet.getRange("A2").format = { fill: "#E7EEF5", font: { color: "#243B53" } };
}

function styleHeader(range) {
  range.format = {
    fill: "#2F5D7C",
    font: { bold: true, color: "#FFFFFF" },
    wrapText: true,
  };
}

function safeDate(value) {
  return value ? new Date(value) : null;
}

function clip(value, maxLength = 500) {
  if (typeof value !== "string") return value ?? "";
  return value.length > maxLength ? `${value.slice(0, maxLength - 3)}...` : value;
}

async function buildWorkbook(payload, rows) {
  const workbook = Workbook.create();
  const summary = workbook.worksheets.add("汇总");
  const details = workbook.worksheets.add("仓库明细");
  const method = workbook.worksheets.add("检索方法");

  const retrievedDate = new Date(payload.retrieved_at);
  const cutoffDate = new Date(retrievedDate);
  cutoffDate.setFullYear(cutoffDate.getFullYear() - 1);
  const lastRow = rows.length + 1;
  const detailRef = "'仓库明细'";

  addTitle(
    summary,
    "GitHub 船舶系统开源仓库统计",
    `数据源: GitHub REST API Search | 抓取时间: ${payload.retrieved_at}`,
  );
  summary.showGridLines = false;

  summary.getRange("A4:B9").values = [
    ["指标", "数值"],
    ["入选仓库数", null],
    ["Stars 合计", null],
    ["Forks 合计", null],
    ["最近 12 个月更新", null],
    ["涉及语言数", null],
  ];
  styleHeader(summary.getRange("A4:B4"));
  summary.getRange("B5:B9").formulas = [
    [`=COUNTA(${detailRef}!C2:C${lastRow})`],
    [`=SUM(${detailRef}!G2:G${lastRow})`],
    [`=SUM(${detailRef}!H2:H${lastRow})`],
    [
      `=COUNTIF(${detailRef}!M2:M${lastRow},">="&DATE(${cutoffDate.getFullYear()},${cutoffDate.getMonth() + 1},${cutoffDate.getDate()}))`,
    ],
    [`=COUNTA(UNIQUE(FILTER(${detailRef}!F2:F${lastRow},${detailRef}!F2:F${lastRow}<>"")))`],
  ];
  summary.getRange("B5:B9").setNumberFormat("#,##0");

  const categoryRows = summarizeBy(rows, (row) => row.category);
  summary.getRange("A12:F12").values = [["类别", "仓库数", "Stars 合计", "Forks 合计", "近 12 月更新", "Stars最高仓库"]];
  styleHeader(summary.getRange("A12:F12"));
  summary.getRangeByIndexes(12, 0, categoryRows.length, 6).values = categoryRows.map((item) => [
    item.key,
    item.count,
    item.stars,
    item.forks,
    item.active12m,
    item.topRepo,
  ]);
  summary.getRange(`B13:E${12 + categoryRows.length}`).setNumberFormat("#,##0");

  const languageRows = summarizeBy(rows, (row) => row.language || "未标注");
  const langStart = 13 + categoryRows.length + 3;
  summary.getRangeByIndexes(langStart - 1, 0, 1, 5).values = [["语言", "仓库数", "Stars 合计", "Forks 合计", "近 12 月更新"]];
  styleHeader(summary.getRangeByIndexes(langStart - 1, 0, 1, 5));
  summary.getRangeByIndexes(langStart, 0, languageRows.length, 5).values = languageRows.map((item) => [
    item.key,
    item.count,
    item.stars,
    item.forks,
    item.active12m,
  ]);
  summary.getRangeByIndexes(langStart, 1, languageRows.length, 4).setNumberFormat("#,##0");

  const topStart = 4;
  summary.getRange("H4:L4").values = [["Top Stars 仓库", "类别", "语言", "Stars", "更新时间"]];
  styleHeader(summary.getRange("H4:L4"));
  summary.getRangeByIndexes(topStart, 7, Math.min(rows.length, 10), 5).values = rows.slice(0, 10).map((row) => [
    row.full_name,
    row.category,
    row.language || "未标注",
    row.stargazers_count ?? 0,
    safeDate(row.updated_at),
  ]);
  summary.getRange(`K5:K${4 + Math.min(rows.length, 10)}`).setNumberFormat("#,##0");
  summary.getRange(`L5:L${4 + Math.min(rows.length, 10)}`).setNumberFormat("yyyy-mm-dd");

  const chart = summary.charts.add("bar", summary.getRange(`A12:B${12 + categoryRows.length}`));
  chart.title = "仓库数量按类别";
  chart.hasLegend = false;
  chart.xAxis = { axisType: "textAxis" };
  chart.setPosition("H15", "M30");

  const summaryWidths = [150, 90, 100, 90, 95, 220, 28, 260, 120, 95, 80, 100, 80];
  summaryWidths.forEach((width, col) => {
    summary.getRangeByIndexes(0, col, 40, 1).format.columnWidthPx = width;
  });

  details.showGridLines = false;
  const detailHeaders = [
    "排名",
    "类别",
    "仓库",
    "URL",
    "描述",
    "语言",
    "Stars",
    "Forks",
    "Open Issues",
    "许可证",
    "Topics",
    "创建时间",
    "更新时间",
    "最近推送",
    "Archived",
    "匹配检索",
    "相关性备注",
  ];
  details.getRangeByIndexes(0, 0, 1, detailHeaders.length).values = [detailHeaders];
  styleHeader(details.getRangeByIndexes(0, 0, 1, detailHeaders.length));
  details.getRangeByIndexes(1, 0, rows.length, detailHeaders.length).values = rows.map((row) => [
    row.rank,
    row.category,
    row.full_name,
    row.html_url,
    clip(row.description || ""),
    row.language || "未标注",
    row.stargazers_count ?? 0,
    row.forks_count ?? 0,
    row.open_issues_count ?? 0,
    row.license?.spdx_id || row.license?.name || "未标注",
    (row.topics ?? []).join(", "),
    safeDate(row.created_at),
    safeDate(row.updated_at),
    safeDate(row.pushed_at),
    row.archived ? "Yes" : "No",
    row.matched_searches.join("; "),
    row.relevance_notes,
  ]);
  details.tables.add(`A1:Q${lastRow}`, true, "RepoDetails");
  details.freezePanes.freezeRows(1);
  details.freezePanes.freezeColumns(2);
  details.getRange(`G2:I${lastRow}`).setNumberFormat("#,##0");
  details.getRange(`L2:N${lastRow}`).setNumberFormat("yyyy-mm-dd");
  details.getRange(`A1:Q${lastRow}`).format.wrapText = false;
  details.getRange(`E1:E${lastRow}`).format.wrapText = true;
  details.getRange(`K1:K${lastRow}`).format.wrapText = true;
  details.getRange(`P1:Q${lastRow}`).format.wrapText = true;
  details.getRange(`A1:Q${lastRow}`).format.rowHeightPx = 42;
  details.getRange("A1:Q1").format.rowHeightPx = 34;
  const detailWidths = [54, 118, 245, 340, 430, 95, 70, 70, 90, 110, 230, 100, 100, 100, 75, 170, 220];
  detailWidths.forEach((width, col) => {
    details.getRangeByIndexes(0, col, lastRow, 1).format.columnWidthPx = width;
  });

  method.showGridLines = false;
  addTitle(
    method,
    "检索与筛选方法",
    "说明: 本表按 GitHub API 当前返回数据整理，适合做开源项目调研初筛。",
  );
  method.getRange("A4:D4").values = [["检索词", "GitHub 返回总数", "本次取回", "API URL"]];
  styleHeader(method.getRange("A4:D4"));
  method.getRangeByIndexes(4, 0, payload.search_stats.length, 4).values = payload.search_stats.map((item) => [
    `${item.label}: ${item.query}`,
    item.total_count,
    item.returned_count,
    item.api_url,
  ]);
  const noteStart = 7 + payload.search_stats.length;
  method.getRangeByIndexes(noteStart - 1, 0, 7, 2).values = [
    ["筛选规则", "名称、描述、homepage、topics 中需出现 ship/vessel/AIS/maritime/marine/port/fleet 或中文船舶/航运/海事/港口等信号。"],
    ["排除规则", "明显是 parcel/courier/delivery/e-commerce/shipment 且缺少海事信号的物流包裹系统被扣分或排除。"],
    ["排序规则", "按 Stars 降序展示；同 Stars 时按相关性分数排序。"],
    ["覆盖范围", "本表为 GitHub 搜索结果初筛，不代表 GitHub 上全部船舶系统项目。"],
    ["API 数据源", payload.api_source],
    ["抓取时间", payload.retrieved_at],
    ["入选仓库", rows.length],
  ];
  method.getRangeByIndexes(0, 0, noteStart + 7, 1).format.columnWidthPx = 170;
  method.getRangeByIndexes(0, 1, noteStart + 7, 1).format.columnWidthPx = 760;
  method.getRangeByIndexes(0, 2, noteStart + 7, 1).format.columnWidthPx = 100;
  method.getRangeByIndexes(0, 3, noteStart + 7, 1).format.columnWidthPx = 760;
  method.getRangeByIndexes(0, 0, noteStart + 7, 4).format.wrapText = true;
  method.getRangeByIndexes(0, 0, noteStart + 7, 4).format.rowHeightPx = 42;

  const formulaErrors = await workbook.inspect({
    kind: "match",
    searchTerm: "#REF!|#DIV/0!|#VALUE!|#NAME\\?|#N/A",
    options: { useRegex: true, maxResults: 300 },
    summary: "final formula error scan",
    maxChars: 1000,
  });
  console.log("Formula error scan:");
  console.log(formulaErrors.ndjson);

  const summaryInspect = await workbook.inspect({
    kind: "table",
    range: "汇总!A1:L28",
    include: "values,formulas",
    tableMaxRows: 28,
    tableMaxCols: 12,
    maxChars: 5000,
  });
  console.log("Summary inspect:");
  console.log(summaryInspect.ndjson);

  const renderTargets = [
    { sheetName: "汇总", range: "A1:M30" },
    { sheetName: "仓库明细", range: `A1:Q${Math.min(lastRow, 25)}` },
    { sheetName: "检索方法", range: `A1:F${Math.min(noteStart + 7, 28)}` },
  ];
  for (const target of renderTargets) {
    const preview = await workbook.render({
      sheetName: target.sheetName,
      range: target.range,
      scale: 1,
      format: "png",
    });
    const bytes = new Uint8Array(await preview.arrayBuffer());
    await fs.writeFile(path.join(outputDir, `${target.sheetName}.png`), bytes);
  }

  const output = await SpreadsheetFile.exportXlsx(workbook);
  await output.save(workbookPath);
  return workbookPath;
}

const payload = await loadGithubData();
const rows = selectRepositories(payload);
if (rows.length === 0) {
  throw new Error("No relevant repositories were selected from the GitHub search results.");
}

const saved = await buildWorkbook(payload, rows);
console.log(JSON.stringify({ saved, selected_repositories: rows.length }, null, 2));

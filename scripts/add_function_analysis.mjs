import fs from "node:fs/promises";
import path from "node:path";
import { FileBlob, SpreadsheetFile } from "@oai/artifact-tool";

const outputDir = path.resolve("outputs", "ship_system_github_stats_20260605");
const workbookPath = path.join(outputDir, "github_ship_systems_stats.xlsx");
const rawGithubPath = path.join(outputDir, "github_api_raw.json");
const sourceCachePath = path.join(outputDir, "function_analysis_sources.json");
const previewPath = path.join(outputDir, "功能分析.png");

const headers = {
  Accept: "application/vnd.github+json",
  "User-Agent": "codex-ship-function-analysis",
  "X-GitHub-Api-Version": "2022-11-28",
};

const selectedProjects = [
  {
    category: "AIS/船舶跟踪",
    repo: "OpenCPN/OpenCPN",
    position: "开源船载导航与海图显示平台，面向实际航行中的驾驶台导航界面。",
    core: "支持栅格海图和 S57 矢量海图显示，接入 GPS/NMEA 与 AIS 信息，提供航点、航线、自动舵导航和天气/GRIB 等航海辅助能力。",
    inputs: "GPS/NMEA、AIS、BSB 栅格海图、S57 ENC 矢量海图、天气/GRIB 等航海数据。",
    tech: "C/C++ 桌面/移动跨平台应用，基于 wxWidgets，支持 Windows、macOS、Linux、Android。",
    scenario: "适合游艇、商船辅助导航、船载 chartplotter、离线海图浏览和 AIS 航行态势展示。",
    highlights: "项目成熟度最高，功能覆盖导航、海图、AIS 和航线操作，是完整船舶导航系统代表。",
    caveats: "README 强调更偏主导航界面；离线路线规划、潮汐预报、日志等能力可能需要插件或其他工具配合。",
  },
  {
    category: "AIS/船舶跟踪",
    repo: "schwehr/libais",
    position: "AIS 消息解析库，服务于船舶自动识别系统数据解码。",
    core: "解码海事 Automatic Identification System 报文，将 AIS/NMEA 原始消息转换为可被上层系统使用的结构化船舶动态和静态信息。",
    inputs: "AIS/NMEA 报文、AIVDM/AIVDO 等 AIS 句子。",
    tech: "C++/Python 库，提供构建、安装和测试流程。",
    scenario: "适合作为 AIS 接收器、船舶跟踪平台、海事数据分析系统的底层解析组件。",
    highlights: "定位清晰，适合作为多个船舶系统的数据入口模块。",
    caveats: "它不是完整业务系统；地图展示、告警、数据库和前端需要由其他系统实现。",
  },
  {
    category: "AIS/船舶跟踪",
    repo: "tg12/phantomtide",
    position: "跨海事与空域的地理空间 OSINT/态势分析平台。",
    core: "对多源信号进行热点排序，呈现实时船舶跟踪、AIS 数据分析、港口活动、航线、异常、制裁监测和区域情报上下文。",
    inputs: "AIS、海事通信、空域/ADS-B、港口活动、地理空间和开放情报数据源。",
    tech: "地理空间分析平台，README 强调 analyst workspace、freshness semantics、proximity query 和 area intel。",
    scenario: "适合海事情报、港口与航线态势研判、异常目标筛查和跨域安全分析。",
    highlights: "不只是画点位，而是把多源重叠、数据新鲜度和异常上下文转成分析工作流。",
    caveats: "偏分析平台和 OSINT 场景；依赖外部数据源质量，业务落地需关注数据授权和持续接入。",
  },
  {
    category: "港口/泊位管理",
    repo: "tanshuimaohenquejiao/WeDanger",
    position: "巡检和隐患上报小程序，可覆盖港口船舶设施等场景。",
    core: "支持隐患工单提交、照片上传、状态跟踪、管理员派工、工作人员处置记录、历史日志和用户评价。",
    inputs: "用户上报信息、隐患类型、地点、联系人、图片、处置过程和处理结果。",
    tech: "微信小程序 + 腾讯云开发，包含管理员、处置工作人员、用户三端。",
    scenario: "适合港口船舶设施、仓储、管线、施工现场等设施巡检和隐患闭环处置。",
    highlights: "工单闭环完整，角色清楚，云开发部署门槛低。",
    caveats: "不是专门船舶业务系统，港口船舶只是其支持的巡检对象之一。",
  },
  {
    category: "港口/泊位管理",
    repo: "YuGo-up/Port-Ship-Management-System",
    position: "面向港口管理员的船舶信息管理课程项目。",
    core: "使用 Qt 前端与 MySQL 后端进行船舶信息管理，提供界面交互、数据库维护、数据存取和基础管理维护能力。",
    inputs: "船舶信息、港口管理员维护的数据、MySQL 数据库记录。",
    tech: "C++、Qt、MySQL、ODBC，分为前端与后台服务器端。",
    scenario: "适合教学、港口船舶信息管理原型、Qt+MySQL 船舶管理系统参考。",
    highlights: "船舶/港口定位明确，技术栈适合桌面管理工具。",
    caveats: "README 明确说明为一周课程大作业且未优化，不宜直接视为生产级系统。",
  },
  {
    category: "管理/运营",
    repo: "budaLi/pship",
    position: "基于 Django 和 Bootstrap 的船舶信息管理系统。",
    core: "提供航次、燃油、货物、港口信息的增删改查，支持船名联想输入和总预算自动计算。",
    inputs: "航次信息、燃油信息、货物信息、港口信息、船名和预算相关数据。",
    tech: "Django + Bootstrap Web 管理系统。",
    scenario: "适合小型船舶运营信息台账、航次成本和港口货物数据管理。",
    highlights: "业务字段与船舶运营高度相关，功能边界简洁。",
    caveats: "README 较短，未看到权限、审计、部署、安全等生产能力说明。",
  },
  {
    category: "管理/运营",
    repo: "nature924/No108Ship-supervision-system",
    position: "基于 Spring Boot 的船舶监造流程管理系统。",
    core: "覆盖管理员、员工、用户角色，支持项目管理、开工准备、项目图纸、监造过程验收、质量管理、公告和用户管理。",
    inputs: "监造项目、图纸文件、开工准备记录、验收记录、质量问题、公告和用户账号。",
    tech: "Spring Boot Web 系统，README 描述前台/后台和多角色功能模块。",
    scenario: "适合船舶建造/监造过程的信息化管理、质量跟踪和资料归档。",
    highlights: "围绕监造生命周期展开，功能模块比普通 CRUD 更贴近船舶建造管理。",
    caveats: "仓库带有毕业设计和源码售卖说明，交付质量、授权和可维护性需要二次核查。",
  },
  {
    category: "管理/运营",
    repo: "nature924/No110Ship-Maintenance-Management-System",
    position: "基于 Spring Boot 的船舶维保管理系统。",
    core: "覆盖管理员、船家、维保公司、维保人员四类角色，支持船舶管理、维保公司/人员管理、维保计划、故障上报和维修成本管理。",
    inputs: "船舶信息、维保计划、故障上报、维修成本、维保公司与人员资料。",
    tech: "Spring Boot Web 系统，前后台分离的多角色管理模块。",
    scenario: "适合船舶维保计划编制、故障闭环、维保成本统计和维保服务协作。",
    highlights: "角色覆盖完整，能把船家、维保公司和维修人员的流程串起来。",
    caveats: "同样带毕业设计/源码售卖属性，上线前需评估代码完整性、安全性和许可。",
  },
  {
    category: "监控/交通态势",
    repo: "rhinonix/equasis-cli",
    position: "面向 Equasis 的船舶情报命令行查询工具。",
    core: "批量查询船舶和管理公司信息，自动收集 50+ 数据点，包括管理公司、PSC 检查、历史船名/船旗、船级等，并支持多格式导出。",
    inputs: "Equasis 网站数据、船舶/公司查询条件、网页解析结果。",
    tech: "Python CLI，依赖对 Equasis 页面结构的 HTML 解析。",
    scenario: "适合大量船舶尽调、船队分析、监管检查、所有权和管理公司追踪。",
    highlights: "把繁琐网页查询变成可脚本化流程，适合批量海事情报分析。",
    caveats: "README 提醒网页结构变化会导致解析失效；也需要关注 Equasis 使用条款。",
  },
  {
    category: "监控/交通态势",
    repo: "himanshukumar660/Vessel-Tracking",
    position: "实时船舶历史轨迹查询网站示例。",
    core: "用户输入 MMSI 或跟踪号后，通过 MarineTraffic 和 Google Maps API 展示船舶实时/历史轨迹，支持 45 天历史、检查点表和自定义地图标记。",
    inputs: "MMSI、跟踪号、MarineTraffic 数据、Google Maps 地理编码和地图数据。",
    tech: "JavaScript、CSS、HTML、Material Design Bootstrap，依赖 MarineTraffic 与 Google Maps API。",
    scenario: "适合船舶轨迹查询、物流/航运可视化原型和 API 使用演示。",
    highlights: "README 对用户输入、API 调用、JSON 轨迹处理和地图展示流程描述清晰。",
    caveats: "项目较像 API demo；生产使用需要处理 API 授权、错误、限流和数据缓存。",
  },
  {
    category: "监控/交通态势",
    repo: "jorgepsmatos/cft-otb",
    position: "海上航空影像中的船舶目标跟踪研究代码。",
    core: "在海事场景中基准测试通用跟踪算法，并提供基于 KCF 和 blob analysis 的船舶跟踪方法，可结合 CNN 或 HOG 特征评估。",
    inputs: "海洋航空影像、目标检测/跟踪数据、CNN/HOG 特征、OTB benchmark 数据。",
    tech: "Matlab、Python 2.7、OpenCV、Caffe、Dlib、VLFeat、Matconvnet 等研究栈。",
    scenario: "适合海上目标跟踪算法研究、遥感/航空影像船舶检测跟踪实验。",
    highlights: "提供的是算法评测和研究方法，能补充 AIS 之外的视觉监控能力。",
    caveats: "依赖较老且复杂，不是业务系统；需要算法工程改造才能落地。",
  },
  {
    category: "其他船舶系统",
    repo: "domie23/springboot-vue5220",
    position: "船舶监造系统的 Spring Boot/Vue 模板化实现。",
    core: "README 强调分层架构、RESTful API、业务层、持久层、实体映射、前后端分离、多环境配置和可扩展模块设计。",
    inputs: "船舶监造业务数据、用户请求、数据库记录和接口数据。",
    tech: "Spring Boot 3.x、Spring MVC、MyBatis/MyBatis-Plus、MySQL、Redis、Spring Security、JWT、Swagger/OpenAPI，前端可选 Vue/React。",
    scenario: "适合作为船舶监造 Web 系统的工程骨架或毕业设计参考。",
    highlights: "架构说明完整，包含安全、缓存、API 文档和模块化扩展思路。",
    caveats: "README 偏通用模板和商业源码说明，需进入源码核查实际功能模块是否完整。",
  },
  {
    category: "其他船舶系统",
    repo: "domie23/springboot-vue2450",
    position: "另一套船舶监造系统 Spring Boot/Vue 模板项目。",
    core: "功能定位与 springboot-vue5220 类似，突出 Controller-Service-DAO 分层、前后端分离、REST API、数据库持久化、安全认证和缓存能力。",
    inputs: "船舶监造业务记录、用户/角色数据、接口请求和 MySQL/Redis 数据。",
    tech: "Spring Boot、MyBatis、MySQL、Redis、Spring Security、JWT、Swagger/OpenAPI，前端可接 Vue 或 React。",
    scenario: "适合对比船舶监造系统模板、评估 Java Web 技术栈实现方式。",
    highlights: "技术栈现代，容易被 Java Web 团队二次开发。",
    caveats: "README 与 5220 高度相似，代表性有限；需避免把模板说明误判为完整产品能力。",
  },
  {
    category: "其他船舶系统",
    repo: "LiquidGalaxyLAB/LG-Ship-Automatic-Identification-System-visualization",
    position: "面向 Liquid Galaxy 的船舶 AIS 实时可视化应用。",
    core: "提供实时船舶跟踪、详细船舶信息、预测航线分析、碰撞风险管理，并支持应用与 Liquid Galaxy 多屏地图同步，也可独立运行。",
    inputs: "AIS 船舶位置/信息、地图数据、路线预测和风险分析相关数据。",
    tech: "Android/Tablet 应用，集成地图和 Liquid Galaxy 系统同步能力。",
    scenario: "适合海事展示大厅、教学演示、多屏态势可视化、AIS 船舶信息展示。",
    highlights: "重点在实时可视化和多屏沉浸式展示，交互表现力强。",
    caveats: "更偏展示和原型应用；实际生产监控需要验证数据源、延迟、告警和权限。",
  },
  {
    category: "其他船舶系统",
    repo: "EzeLLM/SeaPatriot",
    position: "基于 Selenium 的船舶跟踪工具。",
    core: "仓库描述显示其用于 vessel tracking，可能通过浏览器自动化抓取或查询船舶位置页面。",
    inputs: "网页船舶跟踪数据、Selenium 自动化访问结果。",
    tech: "README 缺失；从仓库描述判断为 Selenium 自动化工具。",
    scenario: "适合做船舶跟踪网页自动化采集或快速原型。",
    highlights: "轻量，可能用于绕过缺少 API 的网页数据查询场景。",
    caveats: "README 不可访问，功能细节证据弱；还需检查源码、目标网站许可和反爬风险。",
  },
];

const categorySummary = [
  [
    "AIS/船舶跟踪",
    "围绕 AIS 数据接入、解码、地图展示和航行态势分析。",
    "实时船位、AIS 报文解析、海图/地图展示、异常和热点分析。",
    "船载导航、船舶跟踪平台、海事情报、AIS 数据处理。",
    "OpenCPN/OpenCPN; schwehr/libais; tg12/phantomtide",
  ],
  [
    "港口/泊位管理",
    "偏港口设施巡检、船舶信息维护和港口管理员操作。",
    "隐患工单、派工处置、船舶信息数据库、前后台维护。",
    "港口船舶设施巡检、港口船舶台账、课程/原型系统。",
    "tanshuimaohenquejiao/WeDanger; YuGo-up/Port-Ship-Management-System",
  ],
  [
    "管理/运营",
    "偏船舶运营、监造和维保流程的信息化管理。",
    "航次/燃油/货物/港口 CRUD、项目图纸、验收、质量、维保计划、故障和成本。",
    "船舶运营台账、建造监造、维保服务协同。",
    "budaLi/pship; nature924/No108Ship-supervision-system; nature924/No110Ship-Maintenance-Management-System",
  ],
  [
    "监控/交通态势",
    "偏船舶情报查询、轨迹展示和视觉目标跟踪。",
    "Equasis 数据查询、实时/历史轨迹、地图检查点、航空影像船舶跟踪算法。",
    "船舶尽调、海事情报分析、轨迹可视化、视觉监控研究。",
    "rhinonix/equasis-cli; himanshukumar660/Vessel-Tracking; jorgepsmatos/cft-otb",
  ],
  [
    "其他船舶系统",
    "包含船舶监造模板、AIS 可视化和轻量跟踪工具。",
    "Spring Boot/Vue 工程骨架、AIS 多屏可视化、Selenium 跟踪。",
    "毕业设计参考、演示可视化、快速跟踪原型。",
    "domie23/springboot-vue5220; domie23/springboot-vue2450; LiquidGalaxyLAB/LG-Ship-Automatic-Identification-System-visualization; EzeLLM/SeaPatriot",
  ],
];

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

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
    return { __status: response.status, __body: await response.text() };
  }
  return response.json();
}

async function fetchReadme(repo) {
  const data = await fetchJsonWithRetry(`https://api.github.com/repos/${repo}/readme`);
  if (data.__status) {
    return {
      repo,
      status: data.__status,
      readmeUrl: "",
      evidence: "README 不可访问，回退到仓库描述和统计表元数据",
      length: 0,
    };
  }
  const content = Buffer.from((data.content ?? "").replace(/\n/g, ""), "base64").toString("utf8");
  return {
    repo,
    status: 200,
    readmeUrl: data.html_url || `https://github.com/${repo}#readme`,
    evidence: data.html_url || `https://github.com/${repo}#readme`,
    path: data.path,
    length: content.length,
    preview: content.slice(0, 1200),
  };
}

function loadRawMetadata() {
  return fs
    .readFile(rawGithubPath, "utf8")
    .then((text) => {
      const parsed = JSON.parse(text);
      return new Map((parsed.repositories ?? []).map((repo) => [repo.full_name, repo]));
    })
    .catch(() => new Map());
}

function truncate(value, maxLength = 320) {
  const text = String(value ?? "");
  return text.length > maxLength ? `${text.slice(0, maxLength - 3)}...` : text;
}

function formatDate(value) {
  return value ? new Date(value) : null;
}

function styleHeader(range) {
  range.format = {
    fill: "#2F5D7C",
    font: { bold: true, color: "#FFFFFF" },
    wrapText: true,
  };
}

async function getOrCreateAnalysisSheet(workbook) {
  let sheet;
  try {
    sheet = workbook.worksheets.getItem("功能分析");
    const used = sheet.getUsedRange();
    if (used) used.clear({ applyTo: "all" });
    sheet.deleteAllDrawings();
  } catch {
    sheet = workbook.worksheets.add("功能分析");
  }
  return sheet;
}

const metadataMap = await loadRawMetadata();
const readmeSources = [];
for (const project of selectedProjects) {
  console.log(`Fetching README: ${project.repo}`);
  readmeSources.push(await fetchReadme(project.repo));
  await sleep(500);
}
await fs.writeFile(
  sourceCachePath,
  JSON.stringify({ fetched_at: new Date().toISOString(), sources: readmeSources }, null, 2),
  "utf8",
);
const sourceMap = new Map(readmeSources.map((item) => [item.repo, item]));

const input = await FileBlob.load(workbookPath);
const workbook = await SpreadsheetFile.importXlsx(input);
const sheet = await getOrCreateAnalysisSheet(workbook);
sheet.showGridLines = false;

sheet.getRange("A1:M1").merge();
sheet.getRange("A1").values = [["船舶系统代表项目功能分析"]];
sheet.getRange("A1").format = {
  fill: "#17324D",
  font: { bold: true, color: "#FFFFFF", size: 16 },
};
sheet.getRange("A2:M2").merge();
sheet.getRange("A2").values = [[`数据来源: GitHub README + 现有统计表 | 分析项目: ${selectedProjects.length} 个 | 生成时间: ${new Date().toISOString()}`]];
sheet.getRange("A2").format = { fill: "#E7EEF5", font: { color: "#243B53" }, wrapText: true };

sheet.getRange("A4:E4").values = [["类别", "代表功能", "常见能力", "适合场景", "代表项目"]];
styleHeader(sheet.getRange("A4:E4"));
sheet.getRangeByIndexes(4, 0, categorySummary.length, 5).values = categorySummary;
sheet.getRange(`A5:E${4 + categorySummary.length}`).format.wrapText = true;
sheet.getRange(`A5:E${4 + categorySummary.length}`).format.rowHeightPx = 72;

const detailStartRow = 12;
const headersRow = [
  "类别",
  "项目",
  "URL",
  "功能定位",
  "核心功能总结",
  "输入/数据来源",
  "技术形态",
  "适用场景",
  "亮点",
  "局限/注意事项",
  "Stars",
  "更新时间",
  "分析依据",
];
sheet.getRangeByIndexes(detailStartRow - 1, 0, 1, headersRow.length).values = [headersRow];
styleHeader(sheet.getRangeByIndexes(detailStartRow - 1, 0, 1, headersRow.length));

const detailRows = selectedProjects.map((project) => {
  const repo = metadataMap.get(project.repo) ?? {};
  const source = sourceMap.get(project.repo);
  const evidence = source?.status === 200 ? source.readmeUrl : source?.evidence || "统计表元数据";
  return [
    project.category,
    project.repo,
    `https://github.com/${project.repo}`,
    project.position,
    project.core,
    project.inputs,
    project.tech,
    project.scenario,
    project.highlights,
    project.caveats,
    repo.stargazers_count ?? "",
    formatDate(repo.updated_at),
    truncate(evidence, 300),
  ];
});

sheet.getRangeByIndexes(detailStartRow, 0, detailRows.length, headersRow.length).values = detailRows;
sheet.getRange(`K${detailStartRow + 1}:K${detailStartRow + detailRows.length}`).setNumberFormat("#,##0");
sheet.getRange(`L${detailStartRow + 1}:L${detailStartRow + detailRows.length}`).setNumberFormat("yyyy-mm-dd");

sheet.freezePanes.freezeRows(detailStartRow);
sheet.getRange(`A1:M${detailStartRow + detailRows.length}`).format.wrapText = true;
sheet.getRangeByIndexes(detailStartRow, 0, detailRows.length, headersRow.length).format.rowHeightPx = 96;
sheet.getRangeByIndexes(detailStartRow - 1, 0, 1, headersRow.length).format.rowHeightPx = 36;

const widths = [115, 280, 310, 230, 420, 280, 300, 290, 290, 320, 70, 105, 310];
widths.forEach((width, col) => {
  sheet.getRangeByIndexes(0, col, detailStartRow + detailRows.length + 2, 1).format.columnWidthPx = width;
});

const expected = new Set(selectedProjects.map((project) => project.repo));
const actual = new Set(detailRows.map((row) => row[1]));
const missing = [...expected].filter((repo) => !actual.has(repo));
if (missing.length > 0) {
  throw new Error(`Missing projects in function analysis sheet: ${missing.join(", ")}`);
}
const missingEvidence = detailRows.filter((row) => !row[12]);
if (missingEvidence.length > 0) {
  throw new Error(`Missing evidence for projects: ${missingEvidence.map((row) => row[1]).join(", ")}`);
}

const formulaErrors = await workbook.inspect({
  kind: "match",
  searchTerm: "#REF!|#DIV/0!|#VALUE!|#NAME\\?|#N/A",
  options: { useRegex: true, maxResults: 300 },
  summary: "formula error scan after function analysis",
  maxChars: 1000,
});
console.log("Formula error scan:");
console.log(formulaErrors.ndjson);

const analysisInspect = await workbook.inspect({
  kind: "table",
  range: "功能分析!A1:M28",
  include: "values,formulas",
  tableMaxRows: 28,
  tableMaxCols: 13,
  maxChars: 10000,
});
console.log("Function analysis inspect:");
console.log(analysisInspect.ndjson);

const preview = await workbook.render({
  sheetName: "功能分析",
  range: "A1:M28",
  scale: 1,
  format: "png",
});
await fs.writeFile(previewPath, new Uint8Array(await preview.arrayBuffer()));

const output = await SpreadsheetFile.exportXlsx(workbook);
await output.save(workbookPath);

const finalSheets = await workbook.inspect({
  kind: "sheet",
  include: "id,name",
  maxChars: 4000,
});
console.log("Sheets after export:");
console.log(finalSheets.ndjson);
console.log(JSON.stringify({ saved: workbookPath, analyzed_projects: selectedProjects.length }, null, 2));

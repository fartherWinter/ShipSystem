import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import ts from 'typescript';

export const ROOT = process.cwd();
export const APP_FILE = join(ROOT, 'src', 'App.tsx');
export const PAGES_DIR = join(ROOT, 'src', 'pages');
export const EXPECTED_ROUTES = new Map([
  ['/dashboard', { component: 'DashboardPage', roles: ['super_admin', 'admin', 'dispatcher', 'viewer'] }],
  ['/ships', { component: 'ShipsPage', roles: ['super_admin', 'admin', 'dispatcher', 'viewer'] }],
  ['/monitor', { component: 'MonitorPage', roles: ['super_admin', 'admin', 'dispatcher', 'viewer'] }],
  ['/battle', { component: 'BattlePage', roles: ['super_admin', 'admin', 'dispatcher', 'viewer'] }],
  ['/tracks', { component: 'TracksPage', roles: ['super_admin', 'admin', 'dispatcher', 'viewer'] }],
  ['/alarms', { component: 'AlarmsPage', roles: ['super_admin', 'admin', 'dispatcher', 'viewer'] }],
  ['/dispatch', { component: 'DispatchPage', roles: ['super_admin', 'admin', 'dispatcher'] }],
  ['/rbac', { component: 'RbacPage', roles: ['super_admin'] }],
]);

export function checkFrontendRoutes({
  appFile = APP_FILE,
  pagesDir = PAGES_DIR,
  expectedRoutes = EXPECTED_ROUTES,
  readFile = (path) => readFileSync(path, 'utf8'),
  exists = existsSync,
} = {}) {
  const sourceText = readFile(appFile);
  const sourceFile = ts.createSourceFile(appFile, sourceText, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const lazyPages = collectLazyPages(sourceFile);
  const routeConfig = collectRouteConfig(sourceFile);
  const renderedRoutes = collectRenderedRoutes(sourceFile);
  const failures = [];

  for (const [path, expected] of expectedRoutes) {
    const config = routeConfig.get(path);
    if (!config) {
      failures.push(`missing route config for ${path}`);
      continue;
    }
    if (config.label === '') {
      failures.push(`route ${path} must define a non-empty label`);
    }
    if (!sameItems(config.roles, expected.roles)) {
      failures.push(`route ${path} roles are ${config.roles.join(',')}, expected ${expected.roles.join(',')}`);
    }

    const renderedComponent = renderedRoutes.get(path);
    if (renderedComponent !== expected.component) {
      failures.push(`route ${path} renders ${renderedComponent || '<missing>'}, expected ${expected.component}`);
    }

    const lazyImport = lazyPages.get(expected.component);
    if (lazyImport !== `./pages/${expected.component}`) {
      failures.push(`component ${expected.component} must be lazy-loaded from ./pages/${expected.component}`);
    }

    const pageFile = join(pagesDir, `${expected.component}.tsx`);
    if (!exists(pageFile)) {
      failures.push(`missing page file ${pageFile}`);
    }
  }

  for (const path of routeConfig.keys()) {
    if (!expectedRoutes.has(path)) {
      failures.push(`unexpected route config path ${path}`);
    }
  }

  for (const path of renderedRoutes.keys()) {
    if (path !== '*' && !expectedRoutes.has(path)) {
      failures.push(`unexpected rendered route path ${path}`);
    }
  }

  if (renderedRoutes.get('*') !== 'Navigate') {
    failures.push('wildcard route must redirect with Navigate');
  }

  return { failures, lazyPages, routeConfig, renderedRoutes };
}

export function main({
  log = console.log,
  error = console.error,
  expectedRoutes = EXPECTED_ROUTES,
  ...options
} = {}) {
  const { failures, lazyPages } = checkFrontendRoutes({ expectedRoutes, ...options });
  if (failures.length > 0) {
    for (const failure of failures) {
      error(`[FAIL] ${failure}`);
    }
    return 1;
  }
  log(`Frontend route smoke: routes=${expectedRoutes.size}, lazy_pages=${lazyPages.size}`);
  return 0;
}

function collectLazyPages(file) {
  const pages = new Map();
  visit(file, (node) => {
    if (!ts.isVariableDeclaration(node) || !ts.isIdentifier(node.name)) return;
    const initializer = node.initializer;
    if (!initializer || !ts.isCallExpression(initializer)) return;
    if (!ts.isIdentifier(initializer.expression) || initializer.expression.text !== 'lazy') return;
    const [callback] = initializer.arguments;
    if (!callback || (!ts.isArrowFunction(callback) && !ts.isFunctionExpression(callback))) return;
    const importPath = findDynamicImportPath(callback.body);
    if (importPath) {
      pages.set(node.name.text, importPath);
    }
  });
  return pages;
}

function collectRouteConfig(file) {
  const configs = new Map();
  visit(file, (node) => {
    if (!ts.isVariableDeclaration(node) || !ts.isIdentifier(node.name) || node.name.text !== 'routes') return;
    if (!node.initializer || !ts.isArrayLiteralExpression(node.initializer)) return;
    for (const element of node.initializer.elements) {
      if (!ts.isObjectLiteralExpression(element)) continue;
      const path = stringProperty(element, 'path');
      if (!path) continue;
      configs.set(path, {
        label: stringProperty(element, 'label'),
        roles: rolesProperty(element),
      });
    }
  });
  return configs;
}

function collectRenderedRoutes(file) {
  const routes = new Map();
  visit(file, (node) => {
    if (!ts.isJsxSelfClosingElement(node)) return;
    if (!ts.isIdentifier(node.tagName) || node.tagName.text !== 'Route') return;
    const path = jsxStringAttribute(node, 'path');
    const component = routeElementComponent(node);
    if (path) {
      routes.set(path, component);
    }
  });
  return routes;
}

function routeElementComponent(node) {
  const attr = node.attributes.properties.find(
    (item) => ts.isJsxAttribute(item) && item.name.text === 'element',
  );
  if (!attr || !ts.isJsxAttribute(attr) || !attr.initializer || !ts.isJsxExpression(attr.initializer)) {
    return '';
  }
  const expression = attr.initializer.expression;
  if (!expression) return '';
  const tag = jsxTagName(expression);
  return ts.isIdentifier(tag) ? tag.text : '';
}

function jsxTagName(expression) {
  if (ts.isJsxElement(expression)) return expression.openingElement.tagName;
  if (ts.isJsxSelfClosingElement(expression)) return expression.tagName;
  return undefined;
}

function jsxStringAttribute(node, name) {
  const attr = node.attributes.properties.find(
    (item) => ts.isJsxAttribute(item) && item.name.text === name,
  );
  if (!attr || !ts.isJsxAttribute(attr) || !attr.initializer || !ts.isStringLiteral(attr.initializer)) {
    return '';
  }
  return attr.initializer.text;
}

function stringProperty(object, name) {
  const prop = object.properties.find(
    (item) => ts.isPropertyAssignment(item) && propertyName(item.name) === name,
  );
  if (!prop || !ts.isPropertyAssignment(prop) || !ts.isStringLiteral(prop.initializer)) {
    return '';
  }
  return prop.initializer.text;
}

function rolesProperty(object) {
  const prop = object.properties.find(
    (item) => ts.isPropertyAssignment(item) && propertyName(item.name) === 'roles',
  );
  if (!prop || !ts.isPropertyAssignment(prop)) return [];
  const expression = ts.isAsExpression(prop.initializer) ? prop.initializer.expression : prop.initializer;
  if (!ts.isArrayLiteralExpression(expression)) return [];
  return expression.elements.filter(ts.isStringLiteral).map((item) => item.text);
}

function propertyName(name) {
  if (ts.isIdentifier(name) || ts.isStringLiteral(name)) return name.text;
  return '';
}

function findDynamicImportPath(node) {
  if (ts.isCallExpression(node) && node.expression.kind === ts.SyntaxKind.ImportKeyword) {
    const [specifier] = node.arguments;
    return ts.isStringLiteral(specifier) ? specifier.text : '';
  }
  let result = '';
  visit(node, (child) => {
    if (result || !ts.isCallExpression(child) || child.expression.kind !== ts.SyntaxKind.ImportKeyword) return;
    const [specifier] = child.arguments;
    if (specifier && ts.isStringLiteral(specifier)) {
      result = specifier.text;
    }
  });
  return result;
}

function sameItems(left, right) {
  return left.length === right.length && left.every((item, index) => item === right[index]);
}

function visit(node, callback) {
  callback(node);
  ts.forEachChild(node, (child) => visit(child, callback));
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exit(main());
}

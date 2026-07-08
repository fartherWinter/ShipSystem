INSERT INTO menus (name, path, icon, sort, created_at, updated_at)
VALUES ('雷达对战', '/battle', 'radar', 35, now(), now())
ON CONFLICT (path) DO UPDATE
SET
  name = EXCLUDED.name,
  icon = EXCLUDED.icon,
  sort = EXCLUDED.sort,
  updated_at = now();

CREATE EXTENSION IF NOT EXISTS postgis;

-- Docker Compose 首次初始化数据库时确保 PostGIS 可用。
-- 正式表结构迁移由 Go 应用内嵌的 backend/internal/database/migrations/*.sql 执行。

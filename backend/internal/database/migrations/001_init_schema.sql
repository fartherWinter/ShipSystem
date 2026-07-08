CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS roles (
  id bigserial PRIMARY KEY,
  name varchar(64) NOT NULL,
  code varchar(64) NOT NULL,
  description varchar(255),
  created_at timestamptz,
  updated_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_roles_code ON roles (code);

CREATE TABLE IF NOT EXISTS menus (
  id bigserial PRIMARY KEY,
  name varchar(64) NOT NULL,
  path varchar(128) NOT NULL,
  icon varchar(64),
  parent_id bigint,
  sort bigint,
  created_at timestamptz,
  updated_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_menus_path ON menus (path);

CREATE TABLE IF NOT EXISTS users (
  id bigserial PRIMARY KEY,
  username varchar(64) NOT NULL,
  password_hash varchar(255) NOT NULL,
  display_name varchar(64) NOT NULL,
  role_id bigint,
  status varchar(32) DEFAULT 'enabled',
  created_at timestamptz,
  updated_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users (username);

CREATE TABLE IF NOT EXISTS ships (
  id bigserial PRIMARY KEY,
  name varchar(128) NOT NULL,
  mmsi varchar(32) NOT NULL,
  ship_type varchar(64),
  flag varchar(32),
  length_m double precision,
  width_m double precision,
  status varchar(32) DEFAULT 'active',
  created_at timestamptz,
  updated_at timestamptz,
  deleted_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ships_mmsi ON ships (mmsi);
CREATE INDEX IF NOT EXISTS idx_ships_deleted_at ON ships (deleted_at);

CREATE TABLE IF NOT EXISTS ship_locations (
  id bigserial PRIMARY KEY,
  ship_id bigint NOT NULL,
  longitude double precision NOT NULL,
  latitude double precision NOT NULL,
  speed_knots double precision,
  course double precision,
  reported_at timestamptz NOT NULL,
  created_at timestamptz,
  geom geography(Point, 4326)
);
CREATE INDEX IF NOT EXISTS idx_ship_locations_ship_id ON ship_locations (ship_id);
CREATE INDEX IF NOT EXISTS idx_ship_locations_reported_at ON ship_locations (reported_at);
CREATE INDEX IF NOT EXISTS idx_ship_locations_geom ON ship_locations USING GIST (geom);
CREATE INDEX IF NOT EXISTS idx_ship_locations_ship_reported ON ship_locations (ship_id, reported_at DESC);

CREATE TABLE IF NOT EXISTS alarms (
  id bigserial PRIMARY KEY,
  ship_id bigint NOT NULL,
  type varchar(64) NOT NULL,
  level varchar(32) NOT NULL,
  title varchar(128) NOT NULL,
  message varchar(512),
  longitude double precision,
  latitude double precision,
  status varchar(32) DEFAULT 'OPEN',
  ack_by bigint,
  ack_at timestamptz,
  created_at timestamptz,
  updated_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_alarms_ship_id ON alarms (ship_id);
CREATE INDEX IF NOT EXISTS idx_alarms_status ON alarms (status);

CREATE TABLE IF NOT EXISTS dispatch_events (
  id bigserial PRIMARY KEY,
  ship_id bigint,
  title varchar(128) NOT NULL,
  description varchar(1024),
  status varchar(32) DEFAULT 'NEW',
  priority varchar(32) DEFAULT 'normal',
  handler_user_id bigint,
  created_by_id bigint,
  created_at timestamptz,
  updated_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_dispatch_events_status ON dispatch_events (status);

CREATE TABLE IF NOT EXISTS dispatch_event_logs (
  id bigserial PRIMARY KEY,
  event_id bigint NOT NULL,
  from_status varchar(32),
  to_status varchar(32) NOT NULL,
  remark varchar(512),
  operator_id bigint,
  created_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_dispatch_event_logs_event_id ON dispatch_event_logs (event_id);

CREATE TABLE IF NOT EXISTS battle_sessions (
  id bigserial PRIMARY KEY,
  session_id varchar(64) NOT NULL,
  name varchar(128) NOT NULL,
  scenario_code varchar(64) NOT NULL,
  status varchar(32) NOT NULL DEFAULT 'running',
  started_at timestamptz NOT NULL,
  stopped_at timestamptz,
  last_scan_at timestamptz,
  created_at timestamptz,
  updated_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_battle_sessions_session_id ON battle_sessions (session_id);
CREATE INDEX IF NOT EXISTS idx_battle_sessions_scenario_code ON battle_sessions (scenario_code);
CREATE INDEX IF NOT EXISTS idx_battle_sessions_status ON battle_sessions (status);
CREATE INDEX IF NOT EXISTS idx_battle_sessions_started_at ON battle_sessions (started_at);

CREATE TABLE IF NOT EXISTS radar_targets (
  id bigserial PRIMARY KEY,
  session_id varchar(64) NOT NULL,
  radar_id varchar(64) NOT NULL,
  target_id varchar(64) NOT NULL,
  side varchar(16) NOT NULL,
  longitude double precision NOT NULL,
  latitude double precision NOT NULL,
  course double precision,
  speed_knots double precision,
  confidence double precision,
  detected boolean,
  scan_time timestamptz NOT NULL,
  created_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_radar_target_current ON radar_targets (session_id, radar_id, target_id);
CREATE INDEX IF NOT EXISTS idx_radar_targets_session_id ON radar_targets (session_id);
CREATE INDEX IF NOT EXISTS idx_radar_targets_side ON radar_targets (side);
CREATE INDEX IF NOT EXISTS idx_radar_targets_detected ON radar_targets (detected);
CREATE INDEX IF NOT EXISTS idx_radar_targets_scan_time ON radar_targets (scan_time);
CREATE INDEX IF NOT EXISTS idx_radar_targets_session_scan ON radar_targets (session_id, scan_time DESC);

CREATE TABLE IF NOT EXISTS battle_units (
  id bigserial PRIMARY KEY,
  session_id varchar(64) NOT NULL,
  unit_id varchar(64) NOT NULL,
  ship_id bigint,
  name varchar(128) NOT NULL,
  side varchar(16) NOT NULL,
  hp double precision,
  max_hp double precision,
  radar_range_km double precision,
  weapon_range_km double precision,
  cooldown_seconds double precision,
  longitude double precision NOT NULL,
  latitude double precision NOT NULL,
  course double precision,
  speed_knots double precision,
  status varchar(32) NOT NULL DEFAULT 'active',
  created_at timestamptz,
  updated_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_battle_unit_current ON battle_units (session_id, unit_id);
CREATE INDEX IF NOT EXISTS idx_battle_units_session_id ON battle_units (session_id);
CREATE INDEX IF NOT EXISTS idx_battle_units_side ON battle_units (side);
CREATE INDEX IF NOT EXISTS idx_battle_units_status ON battle_units (status);

CREATE TABLE IF NOT EXISTS battle_projectiles (
  id bigserial PRIMARY KEY,
  session_id varchar(64) NOT NULL,
  projectile_id varchar(64) NOT NULL,
  source_unit_id varchar(64) NOT NULL,
  target_unit_id varchar(64) NOT NULL,
  side varchar(16) NOT NULL,
  longitude double precision NOT NULL,
  latitude double precision NOT NULL,
  speed_km_h double precision,
  status varchar(32) NOT NULL,
  created_at timestamptz,
  updated_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_battle_projectile_current ON battle_projectiles (session_id, projectile_id);
CREATE INDEX IF NOT EXISTS idx_battle_projectiles_session_id ON battle_projectiles (session_id);
CREATE INDEX IF NOT EXISTS idx_battle_projectiles_source_unit_id ON battle_projectiles (source_unit_id);
CREATE INDEX IF NOT EXISTS idx_battle_projectiles_target_unit_id ON battle_projectiles (target_unit_id);
CREATE INDEX IF NOT EXISTS idx_battle_projectiles_side ON battle_projectiles (side);
CREATE INDEX IF NOT EXISTS idx_battle_projectiles_status ON battle_projectiles (status);

CREATE TABLE IF NOT EXISTS battle_events (
  id bigserial PRIMARY KEY,
  session_id varchar(64) NOT NULL,
  event_id varchar(96) NOT NULL,
  type varchar(64) NOT NULL,
  severity varchar(32) NOT NULL,
  message varchar(512) NOT NULL,
  source_unit_id varchar(64),
  target_unit_id varchar(64),
  longitude double precision,
  latitude double precision,
  occurred_at timestamptz NOT NULL,
  created_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_battle_event_once ON battle_events (session_id, event_id);
CREATE INDEX IF NOT EXISTS idx_battle_events_session_id ON battle_events (session_id);
CREATE INDEX IF NOT EXISTS idx_battle_events_type ON battle_events (type);
CREATE INDEX IF NOT EXISTS idx_battle_events_severity ON battle_events (severity);
CREATE INDEX IF NOT EXISTS idx_battle_events_occurred_at ON battle_events (occurred_at);
CREATE INDEX IF NOT EXISTS idx_battle_events_session_time ON battle_events (session_id, occurred_at DESC);

CREATE TABLE IF NOT EXISTS battle_snapshots (
  id bigserial PRIMARY KEY,
  session_id varchar(64) NOT NULL,
  tick bigint NOT NULL,
  snapshot_time timestamptz NOT NULL,
  units_json jsonb NOT NULL,
  projectiles_json jsonb NOT NULL,
  radar_targets_json jsonb NOT NULL,
  events_json jsonb NOT NULL,
  created_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_battle_snapshot_tick ON battle_snapshots (session_id, tick);
CREATE INDEX IF NOT EXISTS idx_battle_snapshots_session_id ON battle_snapshots (session_id);
CREATE INDEX IF NOT EXISTS idx_battle_snapshots_snapshot_time ON battle_snapshots (snapshot_time);
CREATE INDEX IF NOT EXISTS idx_battle_snapshots_session_time ON battle_snapshots (session_id, snapshot_time);

export type {
  Alarm,
  AlarmPageResult,
  AnalyticsErrorResponse,
  AnalyticsProxyResponse,
  BattleDamageStat,
  BattleEvent,
  BattleProjectile,
  BattleReport,
  BattleScenario,
  BattleScenarioListResponse,
  BattleSession,
  BattleSessionCreateRequest,
  BattleSessionCreateResponse,
  BattleSessionPageResult,
  BattleSimulationStartRequest,
  BattleSimulationStopRequest,
  BattleSnapshot,
  BattleSnapshotListResponse,
  BattleState,
  BattleStatePayload,
  BattleTimelineEventSummary,
  BattleTimelineItem,
  BattleTimelineResponse,
  BattleUnit,
  BattleReport as BattleReportType,
  DispatchEvent,
  DispatchEventCreateRequest,
  DispatchEventPageResult,
  DispatchStatusUpdateRequest,
  LocationReportResponse,
  LoginRequest,
  LoginResponse,
  Menu,
  MenuListResponse,
  RadarReportPayload,
  RadarTarget,
  Role,
  RoleListResponse,
  Ship,
  ShipLocation,
  ShipLocationReportRequest,
  ShipPageResult,
  ShipUpsertRequest,
  SimulationStartRequest,
  TrackListResponse,
  User,
  UserListResponse,
} from './generated/api-types';

import type {
  Alarm,
  BattleEvent,
  BattleProjectile,
  BattleState,
  DispatchEvent,
  RadarTarget,
  ShipLocation,
} from './generated/api-types';

export type RadarScan = {
  sessionId: string;
  radarId: string;
  scanTime: string;
  targets: RadarTarget[];
};

export type WsMessage =
  | { type: 'ship_location_updated'; data: ShipLocation }
  | { type: 'alarm_created'; data: Alarm }
  | { type: 'dispatch_event_updated'; data: DispatchEvent }
  | { type: 'radar_scan_updated'; data: RadarScan }
  | { type: 'battle_state_updated'; data: BattleState }
  | { type: 'battle_event_created'; data: BattleEvent }
  | { type: 'projectile_updated'; data: BattleProjectile }
  | { type: 'heartbeat'; data: { time: string } };

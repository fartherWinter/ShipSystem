import type { Sensor, Track, TrackPoint, Vec3, Zone } from "./types";

type PointFeature = {
  type: "Feature";
  geometry: { type: "Point"; coordinates: [number, number] };
  properties: Record<string, string | number>;
};

type PolygonFeature = {
  type: "Feature";
  geometry: { type: "Polygon"; coordinates: Array<Array<[number, number]>> };
  properties: Record<string, string | number>;
};

type LineFeature = {
  type: "Feature";
  geometry: { type: "LineString"; coordinates: Array<[number, number]> };
  properties: Record<string, string | number>;
};

export type FeatureCollection<TFeature> = {
  type: "FeatureCollection";
  features: TFeature[];
};

export function tracksToFeatureCollection(tracks: Track[]): FeatureCollection<PointFeature> {
  return {
    type: "FeatureCollection",
    features: tracks.map((track) => ({
      type: "Feature",
      geometry: { type: "Point", coordinates: [track.position.lon, track.position.lat] },
      properties: {
        id: track.id,
        track_no: track.track_no,
        threat: track.threat_level,
        kind: track.kind,
        status: track.status,
        confidence: Math.round(track.confidence * 100)
      }
    }))
  };
}

export function sensorsToFeatureCollection(sensors: Sensor[]): FeatureCollection<PointFeature> {
  return {
    type: "FeatureCollection",
    features: sensors
      .filter((sensor) => Number.isFinite(sensor.position?.lon) && Number.isFinite(sensor.position?.lat))
      .map((sensor) => ({
        type: "Feature",
        geometry: { type: "Point", coordinates: [sensor.position.lon, sensor.position.lat] },
        properties: {
          id: sensor.id,
          name: sensor.name,
          kind: sensor.kind
        }
      }))
  };
}

export function zonesToFeatureCollection(zones: Zone[]): FeatureCollection<PolygonFeature> {
  return {
    type: "FeatureCollection",
    features: zones
      .filter((zone) => zone.polygon.length >= 3)
      .map((zone) => {
        const ring = zone.polygon.map((point) => [point.lon, point.lat] as [number, number]);
        const first = ring[0];
        const last = ring[ring.length - 1];
        const closed = first[0] === last[0] && first[1] === last[1] ? ring : [...ring, first];
        return {
          type: "Feature",
          geometry: { type: "Polygon", coordinates: [closed] },
          properties: {
            id: zone.id,
            name: zone.name,
            kind: zone.kind
          }
        };
      })
  };
}

export function zoneVerticesToFeatureCollection(zones: Zone[], selectedZoneID = "", selectedVertexIndex = -1): FeatureCollection<PointFeature> {
  return {
    type: "FeatureCollection",
    features: zones.flatMap((zone) =>
      (zone.polygon ?? [])
        .filter((point) => Number.isFinite(point.lon) && Number.isFinite(point.lat))
        .map((point, index) => ({
          type: "Feature" as const,
          geometry: { type: "Point" as const, coordinates: [point.lon, point.lat] as [number, number] },
          properties: {
            id: `${zone.id}:${index}`,
            zone_id: zone.id,
            zone_name: zone.name,
            vertex_index: index,
            order: index + 1,
            selected: zone.id === selectedZoneID && index === selectedVertexIndex ? 1 : 0
          }
        }))
    )
  };
}

export function zoneDraftToFeatureCollection(points: Vec3[]): FeatureCollection<LineFeature> {
  const coordinates = points.map((point) => [point.lon, point.lat] as [number, number]);
  return {
    type: "FeatureCollection",
    features:
      coordinates.length >= 2
        ? [
            {
              type: "Feature",
              geometry: { type: "LineString", coordinates },
              properties: { id: "scenario-zone-draft" }
            }
          ]
        : []
  };
}

export function zoneDraftPointsToFeatureCollection(points: Vec3[]): FeatureCollection<PointFeature> {
  return {
    type: "FeatureCollection",
    features: points.map((point, index) => ({
      type: "Feature",
      geometry: { type: "Point", coordinates: [point.lon, point.lat] },
      properties: {
        id: `draft-${index + 1}`,
        order: index + 1
      }
    }))
  };
}

export function trackHistoryToFeatureCollection(points: TrackPoint[]): FeatureCollection<LineFeature> {
  const coordinates = points.map((point) => [point.position.lon, point.position.lat] as [number, number]);
  return {
    type: "FeatureCollection",
    features:
      coordinates.length >= 2
        ? [
            {
              type: "Feature",
              geometry: { type: "LineString", coordinates },
              properties: { id: points[0]?.track_id ?? "history" }
            }
          ]
        : []
  };
}

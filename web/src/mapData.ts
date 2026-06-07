import type { Track, TrackPoint, Zone } from "./types";

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

import { describe, expect, it } from "vitest";
import { trackHistoryToFeatureCollection, tracksToFeatureCollection, zonesToFeatureCollection } from "./mapData";
import type { Track, Zone } from "./types";

describe("map data transforms", () => {
  it("converts tracks into point features with display properties", () => {
    const tracks: Track[] = [
      {
        id: "track-1",
        track_no: "T-001",
        kind: "surface_contact",
        threat_level: "medium",
        position: { lon: 121.5, lat: 31.2, alt_m: 0 },
        velocity: { lon: 0, lat: 0, alt_m: 0 },
        confidence: 0.82,
        updated_at: "2026-06-07T00:00:00Z",
        status: "active"
      }
    ];

    const collection = tracksToFeatureCollection(tracks);

    expect(collection.features).toHaveLength(1);
    expect(collection.features[0].geometry.coordinates).toEqual([121.5, 31.2]);
    expect(collection.features[0].properties.confidence).toBe(82);
  });

  it("closes zone polygon rings for MapLibre", () => {
    const zones: Zone[] = [
      {
        id: "area",
        name: "Training Area",
        kind: "exercise_boundary",
        polygon: [
          { lon: 1, lat: 1, alt_m: 0 },
          { lon: 2, lat: 1, alt_m: 0 },
          { lon: 2, lat: 2, alt_m: 0 }
        ]
      }
    ];

    const collection = zonesToFeatureCollection(zones);
    const ring = collection.features[0].geometry.coordinates[0];

    expect(ring[0]).toEqual(ring[ring.length - 1]);
  });

  it("builds a selected track history line when enough points exist", () => {
    const collection = trackHistoryToFeatureCollection([
      { track_id: "t1", sampled_at: "2026-06-07T00:00:00Z", position: { lon: 1, lat: 2, alt_m: 0 }, speed: 1, heading: 0, confidence: 0.9 },
      { track_id: "t1", sampled_at: "2026-06-07T00:00:01Z", position: { lon: 3, lat: 4, alt_m: 0 }, speed: 1, heading: 0, confidence: 0.9 }
    ]);

    expect(collection.features).toHaveLength(1);
    expect(collection.features[0].geometry.coordinates).toEqual([
      [1, 2],
      [3, 4]
    ]);
  });
});

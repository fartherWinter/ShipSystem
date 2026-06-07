import { useEffect, useRef, useState } from "react";
import maplibregl, { GeoJSONSource, Map } from "maplibre-gl";
import { mapTileAttribution, mapTileUrl } from "./config";
import { trackHistoryToFeatureCollection, tracksToFeatureCollection, zonesToFeatureCollection } from "./mapData";
import type { Track, TrackPoint, Vec3, Zone } from "./types";

type SimulationMapProps = {
  center: Vec3;
  tracks: Track[];
  trackPoints: TrackPoint[];
  zones: Zone[];
  selectedTrackID?: string;
  onSelectTrack: (track: Track | null) => void;
};

export function SimulationMap({ center, tracks, trackPoints, zones, selectedTrackID, onSelectTrack }: SimulationMapProps) {
  const mapEl = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<Map | null>(null);
  const tracksRef = useRef<Track[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [mapError, setMapError] = useState("");

  tracksRef.current = tracks;

  useEffect(() => {
    if (!mapEl.current || mapRef.current) return;
    const map = new maplibregl.Map({
      container: mapEl.current,
      style: {
        version: 8,
        sources: {
          osm: {
            type: "raster",
            tiles: [mapTileUrl],
            tileSize: 256,
            attribution: mapTileAttribution
          }
        },
        layers: [{ id: "osm", type: "raster", source: "osm" }]
      },
      center: [center.lon, center.lat],
      zoom: 8
    });

    map.addControl(new maplibregl.NavigationControl(), "top-right");
    map.on("error", () => {
      setMapError("Map tiles are unavailable.");
    });
    map.on("load", () => {
      map.addSource("zones", {
        type: "geojson",
        data: zonesToFeatureCollection([])
      });
      map.addLayer({
        id: "zones-fill",
        type: "fill",
        source: "zones",
        paint: {
          "fill-color": "#2d6a4f",
          "fill-opacity": 0.1
        }
      });
      map.addLayer({
        id: "zones-line",
        type: "line",
        source: "zones",
        paint: {
          "line-color": "#2d6a4f",
          "line-width": 2,
          "line-dasharray": [2, 1]
        }
      });
      map.addSource("tracks", {
        type: "geojson",
        data: tracksToFeatureCollection([])
      });
      map.addSource("track-history", {
        type: "geojson",
        data: trackHistoryToFeatureCollection([])
      });
      map.addLayer({
        id: "track-history",
        type: "line",
        source: "track-history",
        paint: {
          "line-color": "#0f766e",
          "line-width": 3,
          "line-opacity": 0.72
        }
      });
      map.addLayer({
        id: "tracks",
        type: "circle",
        source: "tracks",
        paint: {
          "circle-radius": [
            "case",
            ["==", ["get", "id"], selectedTrackID ?? ""],
            11,
            ["==", ["get", "threat"], "high"],
            9,
            6
          ],
          "circle-color": [
            "case",
            ["==", ["get", "threat"], "high"],
            "#d92d20",
            ["==", ["get", "threat"], "medium"],
            "#f79009",
            "#1570ef"
          ],
          "circle-stroke-width": ["case", ["==", ["get", "id"], selectedTrackID ?? ""], 4, 2],
          "circle-stroke-color": "#ffffff"
        }
      });
      map.on("click", "tracks", (event) => {
        const id = event.features?.[0]?.properties?.id;
        onSelectTrack(tracksRef.current.find((track) => track.id === id) ?? null);
      });
      map.on("mouseenter", "tracks", () => {
        map.getCanvas().style.cursor = "pointer";
      });
      map.on("mouseleave", "tracks", () => {
        map.getCanvas().style.cursor = "";
      });
      setLoaded(true);
      setMapError("");
    });
    mapRef.current = map;
    return () => {
      map.remove();
      mapRef.current = null;
    };
  }, []);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !loaded) return;
    const source = map.getSource("tracks") as GeoJSONSource | undefined;
    source?.setData(tracksToFeatureCollection(tracks) as never);
  }, [loaded, tracks]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !loaded) return;
    const source = map.getSource("zones") as GeoJSONSource | undefined;
    source?.setData(zonesToFeatureCollection(zones) as never);
  }, [loaded, zones]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !loaded) return;
    const source = map.getSource("track-history") as GeoJSONSource | undefined;
    source?.setData(trackHistoryToFeatureCollection(trackPoints) as never);
  }, [loaded, trackPoints]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !loaded || !map.getLayer("tracks")) return;
    map.setPaintProperty("tracks", "circle-radius", [
      "case",
      ["==", ["get", "id"], selectedTrackID ?? ""],
      11,
      ["==", ["get", "threat"], "high"],
      9,
      6
    ]);
    map.setPaintProperty("tracks", "circle-stroke-width", ["case", ["==", ["get", "id"], selectedTrackID ?? ""], 4, 2]);
  }, [loaded, selectedTrackID]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !loaded) return;
    map.easeTo({ center: [center.lon, center.lat], duration: 600 });
  }, [center.lat, center.lon, loaded]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !loaded || !selectedTrackID) return;
    const selected = tracks.find((track) => track.id === selectedTrackID);
    if (selected) {
      map.easeTo({ center: [selected.position.lon, selected.position.lat], duration: 450 });
    }
  }, [loaded, selectedTrackID, tracks]);

  return (
    <div className="mapFrame">
      <div ref={mapEl} className="map" />
      {!loaded && !mapError ? (
        <div className="mapStatus" role="status">
          Loading map
        </div>
      ) : null}
      {mapError ? (
        <div className="mapStatus mapStatusError" role="alert">
          {mapError}
        </div>
      ) : null}
    </div>
  );
}

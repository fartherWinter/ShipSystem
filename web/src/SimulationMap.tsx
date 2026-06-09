import { useEffect, useRef, useState, type MouseEvent } from "react";
import maplibregl, { GeoJSONSource, Map, type MapLayerMouseEvent, type MapMouseEvent } from "maplibre-gl";
import { mapTileAttribution, mapTileUrl } from "./config";
import {
  sensorsToFeatureCollection,
  trackHistoryToFeatureCollection,
  tracksToFeatureCollection,
  zoneDraftPointsToFeatureCollection,
  zoneDraftToFeatureCollection,
  zoneVerticesToFeatureCollection,
  zonesToFeatureCollection
} from "./mapData";
import type { ScenarioEditorMapMode, Sensor, Track, TrackPoint, Vec3, Zone } from "./types";

type DraggingVertex = {
  zoneID: string;
  vertexIndex: number;
  point: Vec3;
  moved: boolean;
};

type SimulationMapProps = {
  center: Vec3;
  tracks: Track[];
  trackPoints: TrackPoint[];
  zones: Zone[];
  scenarioSensors?: Sensor[];
  scenarioZones?: Zone[];
  scenarioZoneDraft?: Vec3[];
  scenarioEditorMapMode?: ScenarioEditorMapMode;
  selectedScenarioZoneID?: string;
  selectedScenarioVertexIndex?: number;
  selectedTrackID?: string;
  onSelectTrack: (track: Track | null) => void;
  onScenarioMapClick?: (point: Vec3) => void;
  onScenarioVertexSelect?: (zoneID: string, vertexIndex: number) => void;
  onScenarioVertexMove?: (zoneID: string, vertexIndex: number, point: Vec3) => void;
};

export function SimulationMap({
  center,
  tracks,
  trackPoints,
  zones,
  scenarioSensors = [],
  scenarioZones = [],
  scenarioZoneDraft = [],
  scenarioEditorMapMode = "off",
  selectedScenarioZoneID = "",
  selectedScenarioVertexIndex = -1,
  selectedTrackID,
  onSelectTrack,
  onScenarioMapClick,
  onScenarioVertexSelect,
  onScenarioVertexMove
}: SimulationMapProps) {
  const mapEl = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<Map | null>(null);
  const tracksRef = useRef<Track[]>([]);
  const scenarioZonesRef = useRef<Zone[]>([]);
  const editorModeRef = useRef<ScenarioEditorMapMode>("off");
  const selectedScenarioZoneIDRef = useRef("");
  const selectedScenarioVertexIndexRef = useRef(-1);
  const onScenarioMapClickRef = useRef<SimulationMapProps["onScenarioMapClick"]>(undefined);
  const onScenarioVertexSelectRef = useRef<SimulationMapProps["onScenarioVertexSelect"]>(undefined);
  const onScenarioVertexMoveRef = useRef<SimulationMapProps["onScenarioVertexMove"]>(undefined);
  const draggingVertexRef = useRef<DraggingVertex | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [mapError, setMapError] = useState("");

  tracksRef.current = tracks;
  scenarioZonesRef.current = scenarioZones;
  editorModeRef.current = scenarioEditorMapMode;
  selectedScenarioZoneIDRef.current = selectedScenarioZoneID;
  selectedScenarioVertexIndexRef.current = selectedScenarioVertexIndex;
  onScenarioMapClickRef.current = onScenarioMapClick;
  onScenarioVertexSelectRef.current = onScenarioVertexSelect;
  onScenarioVertexMoveRef.current = onScenarioVertexMove;

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
    let initialized = false;
    const initializeMapLayers = () => {
      if (initialized) return;
      initialized = true;
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
      map.addSource("scenario-zones", {
        type: "geojson",
        data: zonesToFeatureCollection([])
      });
      map.addLayer({
        id: "scenario-zones-fill",
        type: "fill",
        source: "scenario-zones",
        paint: {
          "fill-color": "#245a94",
          "fill-opacity": 0.09
        }
      });
      map.addLayer({
        id: "scenario-zones-line",
        type: "line",
        source: "scenario-zones",
        paint: {
          "line-color": "#245a94",
          "line-width": 2
        }
      });
      map.addSource("scenario-zone-vertices", {
        type: "geojson",
        data: zoneVerticesToFeatureCollection([])
      });
      map.addLayer({
        id: "scenario-zone-vertices",
        type: "circle",
        source: "scenario-zone-vertices",
        layout: { visibility: "none" },
        paint: {
          "circle-radius": ["case", ["==", ["get", "selected"], 1], 8, 5],
          "circle-color": ["case", ["==", ["get", "selected"], 1], "#b42318", "#245a94"],
          "circle-stroke-color": "#ffffff",
          "circle-stroke-width": ["case", ["==", ["get", "selected"], 1], 3, 2]
        }
      });
      map.addSource("scenario-zone-draft", {
        type: "geojson",
        data: zoneDraftToFeatureCollection([])
      });
      map.addLayer({
        id: "scenario-zone-draft",
        type: "line",
        source: "scenario-zone-draft",
        paint: {
          "line-color": "#b7791f",
          "line-width": 3,
          "line-dasharray": [1, 1]
        }
      });
      map.addSource("scenario-zone-draft-points", {
        type: "geojson",
        data: zoneDraftPointsToFeatureCollection([])
      });
      map.addLayer({
        id: "scenario-zone-draft-points",
        type: "circle",
        source: "scenario-zone-draft-points",
        paint: {
          "circle-radius": 5,
          "circle-color": "#b7791f",
          "circle-stroke-color": "#ffffff",
          "circle-stroke-width": 2
        }
      });
      map.addSource("scenario-sensors", {
        type: "geojson",
        data: sensorsToFeatureCollection([])
      });
      map.addLayer({
        id: "scenario-sensors",
        type: "circle",
        source: "scenario-sensors",
        paint: {
          "circle-radius": 8,
          "circle-color": "#245a94",
          "circle-stroke-color": "#ffffff",
          "circle-stroke-width": 2
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
        if (editorModeRef.current !== "off") return;
        const id = event.features?.[0]?.properties?.id;
        onSelectTrack(tracksRef.current.find((track) => track.id === id) ?? null);
      });
      const previewDraggedVertex = (event: MapMouseEvent) => {
        const dragging = draggingVertexRef.current;
        if (!dragging) return;
        event.preventDefault();
        dragging.point = lngLatPoint(event);
        dragging.moved = true;
        previewScenarioVertex(map, scenarioZonesRef.current, dragging);
      };
      const finishDraggedVertex = (event: MapMouseEvent) => {
        const dragging = draggingVertexRef.current;
        if (!dragging) return;
        event.preventDefault();
        map.off("mousemove", previewDraggedVertex);
        map.off("mouseup", finishDraggedVertex);
        map.dragPan.enable();
        map.getCanvas().style.cursor = editorModeRef.current === "vertex" ? "grab" : "";
        draggingVertexRef.current = null;
        if (dragging.moved) {
          onScenarioVertexMoveRef.current?.(dragging.zoneID, dragging.vertexIndex, dragging.point);
        }
      };
      map.on("mousedown", "scenario-zone-vertices", (event: MapLayerMouseEvent) => {
        if (editorModeRef.current !== "vertex") return;
        const target = vertexTargetFromEvent(event);
        if (!target) return;
        event.preventDefault();
        event.originalEvent.stopPropagation();
        draggingVertexRef.current = {
          ...target,
          point: lngLatPoint(event),
          moved: false
        };
        onScenarioVertexSelectRef.current?.(target.zoneID, target.vertexIndex);
        previewScenarioVertex(map, scenarioZonesRef.current, draggingVertexRef.current);
        map.dragPan.disable();
        map.getCanvas().style.cursor = "grabbing";
        map.on("mousemove", previewDraggedVertex);
        map.on("mouseup", finishDraggedVertex);
      });
      map.on("click", (event) => {
        if (editorModeRef.current === "off") return;
        if (draggingVertexRef.current) return;
        onScenarioMapClickRef.current?.({ lon: event.lngLat.lng, lat: event.lngLat.lat, alt_m: 0 });
      });
      map.on("mouseenter", "tracks", () => {
        map.getCanvas().style.cursor = "pointer";
      });
      map.on("mouseleave", "tracks", () => {
        map.getCanvas().style.cursor = "";
      });
      map.on("mouseenter", "scenario-zone-vertices", () => {
        if (editorModeRef.current === "vertex") {
          map.getCanvas().style.cursor = "grab";
        }
      });
      map.on("mouseleave", "scenario-zone-vertices", () => {
        if (editorModeRef.current === "vertex" && !draggingVertexRef.current) {
          map.getCanvas().style.cursor = "crosshair";
        }
      });
      setLoaded(true);
      setMapError("");
    };
    map.on("style.load", initializeMapLayers);
    map.on("load", initializeMapLayers);
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
    const zoneSource = map.getSource("scenario-zones") as GeoJSONSource | undefined;
    const vertexSource = map.getSource("scenario-zone-vertices") as GeoJSONSource | undefined;
    zoneSource?.setData(zonesToFeatureCollection(scenarioZones) as never);
    vertexSource?.setData(zoneVerticesToFeatureCollection(scenarioZones, selectedScenarioZoneID, selectedScenarioVertexIndex) as never);
  }, [loaded, scenarioZones, selectedScenarioVertexIndex, selectedScenarioZoneID]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !loaded) return;
    const source = map.getSource("scenario-sensors") as GeoJSONSource | undefined;
    source?.setData(sensorsToFeatureCollection(scenarioSensors) as never);
  }, [loaded, scenarioSensors]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !loaded) return;
    const lineSource = map.getSource("scenario-zone-draft") as GeoJSONSource | undefined;
    const pointSource = map.getSource("scenario-zone-draft-points") as GeoJSONSource | undefined;
    lineSource?.setData(zoneDraftToFeatureCollection(scenarioZoneDraft) as never);
    pointSource?.setData(zoneDraftPointsToFeatureCollection(scenarioZoneDraft) as never);
  }, [loaded, scenarioZoneDraft]);

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

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !loaded) return;
    map.getCanvas().style.cursor = scenarioEditorMapMode === "off" ? "" : "crosshair";
    if (map.getLayer("scenario-zone-vertices")) {
      map.setLayoutProperty("scenario-zone-vertices", "visibility", scenarioEditorMapMode === "vertex" ? "visible" : "none");
    }
  }, [loaded, scenarioEditorMapMode]);

  function handleFallbackMapClick(event: MouseEvent<HTMLDivElement>) {
    if (loaded || scenarioEditorMapMode === "off") return;
    const rect = event.currentTarget.getBoundingClientRect();
    if (rect.width <= 0 || rect.height <= 0) return;
    const dx = event.clientX - (rect.left + rect.width / 2);
    const dy = event.clientY - (rect.top + rect.height / 2);
    const degreesPerPixel = 360 / (256 * 2 ** 8);
    const lon = center.lon + dx * degreesPerPixel;
    const lat = center.lat - dy * degreesPerPixel;
    onScenarioMapClick?.({ lon, lat, alt_m: 0 });
  }

  return (
    <div className="mapFrame" onClick={handleFallbackMapClick}>
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

function lngLatPoint(event: MapMouseEvent): Vec3 {
  return { lon: event.lngLat.lng, lat: event.lngLat.lat, alt_m: 0 };
}

function vertexTargetFromEvent(event: MapLayerMouseEvent) {
  const properties = event.features?.[0]?.properties as Record<string, unknown> | undefined;
  const zoneID = typeof properties?.zone_id === "string" ? properties.zone_id : "";
  const vertexIndex = Number(properties?.vertex_index);
  if (!zoneID || !Number.isInteger(vertexIndex) || vertexIndex < 0) return null;
  return { zoneID, vertexIndex };
}

function previewScenarioVertex(map: Map, zones: Zone[], dragging: DraggingVertex) {
  const previewZones = moveZoneVertex(zones, dragging.zoneID, dragging.vertexIndex, dragging.point);
  const zoneSource = map.getSource("scenario-zones") as GeoJSONSource | undefined;
  const vertexSource = map.getSource("scenario-zone-vertices") as GeoJSONSource | undefined;
  zoneSource?.setData(zonesToFeatureCollection(previewZones) as never);
  vertexSource?.setData(zoneVerticesToFeatureCollection(previewZones, dragging.zoneID, dragging.vertexIndex) as never);
}

function moveZoneVertex(zones: Zone[], zoneID: string, vertexIndex: number, point: Vec3): Zone[] {
  return zones.map((zone) => {
    if (zone.id !== zoneID || vertexIndex < 0 || vertexIndex >= (zone.polygon ?? []).length) return zone;
    const polygon = [...zone.polygon];
    polygon[vertexIndex] = point;
    return { ...zone, polygon };
  });
}

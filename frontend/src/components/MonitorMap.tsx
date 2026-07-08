import { useEffect, useRef } from 'react';
import Feature from 'ol/Feature';
import OlMap from 'ol/Map';
import View from 'ol/View';
import CircleGeom from 'ol/geom/Circle';
import LineString from 'ol/geom/LineString';
import Point from 'ol/geom/Point';
import TileLayer from 'ol/layer/Tile';
import VectorLayer from 'ol/layer/Vector';
import { isEmpty as isEmptyExtent } from 'ol/extent';
import { fromLonLat } from 'ol/proj';
import OSM from 'ol/source/OSM';
import VectorSource from 'ol/source/Vector';
import { Circle as CircleStyle, Fill, Stroke, Style, Text } from 'ol/style';
import type { BattleProjectile, BattleUnit, RadarTarget, ShipLocation } from '../types';

type Props = {
  locations?: ShipLocation[];
  track?: ShipLocation[];
  battleUnits?: BattleUnit[];
  radarTargets?: RadarTarget[];
  projectiles?: BattleProjectile[];
  fitMode?: 'center-first' | 'fit-data';
};

export default function MonitorMap({
  locations = [],
  track = [],
  battleUnits = [],
  radarTargets = [],
  projectiles = [],
  fitMode = 'center-first',
}: Props) {
  const nodeRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<OlMap | null>(null);
  const sourceRef = useRef(new VectorSource());

  useEffect(() => {
    if (!nodeRef.current || mapRef.current) return;
    mapRef.current = new OlMap({
      target: nodeRef.current,
      layers: [
        new TileLayer({ source: new OSM() }),
        new VectorLayer({
          source: sourceRef.current,
          style: (feature) => {
            const kind = feature.get('kind');
            const side = feature.get('side');
            const color = side === 'red' ? '#dc2626' : side === 'blue' ? '#2563eb' : '#0f766e';
            if (kind === 'track') {
              return new Style({ stroke: new Stroke({ color: '#2563eb', width: 3 }) });
            }
            if (kind === 'radarRange') {
              return new Style({
                stroke: new Stroke({ color, width: 1.5, lineDash: [8, 8] }),
                fill: new Fill({ color: side === 'red' ? 'rgba(220,38,38,0.06)' : 'rgba(37,99,235,0.06)' }),
              });
            }
            if (kind === 'projectileTrack') {
              return new Style({ stroke: new Stroke({ color, width: 2, lineDash: [4, 8] }) });
            }
            if (kind === 'projectile') {
              return new Style({
                image: new CircleStyle({
                  radius: feature.get('status') === 'hit' ? 5 : 4,
                  fill: new Fill({ color: '#f97316' }),
                  stroke: new Stroke({ color: '#ffffff', width: 1.5 }),
                }),
              });
            }
            if (kind === 'radarTarget') {
              return new Style({
                image: new CircleStyle({
                  radius: 5,
                  fill: new Fill({ color: feature.get('detected') ? color : '#94a3b8' }),
                  stroke: new Stroke({ color: '#ffffff', width: 1.5 }),
                }),
              });
            }
            if (kind === 'battleUnit') {
              const status = feature.get('status');
              return new Style({
                image: new CircleStyle({
                  radius: status === 'destroyed' ? 8 : 10,
                  fill: new Fill({ color: status === 'destroyed' ? '#64748b' : color }),
                  stroke: new Stroke({ color: '#ffffff', width: 2 }),
                }),
                text: new Text({
                  text: feature.get('label') ?? '',
                  offsetY: -18,
                  font: '12px "Segoe UI", sans-serif',
                  fill: new Fill({ color: '#172033' }),
                  stroke: new Stroke({ color: '#ffffff', width: 3 }),
                }),
              });
            }
            return new Style({
              image: new CircleStyle({
                radius: 7,
                fill: new Fill({ color: '#0f766e' }),
                stroke: new Stroke({ color: '#ffffff', width: 2 }),
              }),
            });
          },
        }),
      ],
      view: new View({
        center: fromLonLat([121.49, 31.23]),
        zoom: 10,
      }),
    });
    return () => {
      mapRef.current?.setTarget(undefined);
      mapRef.current = null;
    };
  }, []);

  useEffect(() => {
    sourceRef.current.clear();
    const unitById = new Map(battleUnits.map((unit) => [unit.unitId, unit]));
    if (track.length > 1) {
      const line = new Feature({
        geometry: new LineString(track.map((item) => fromLonLat([item.longitude, item.latitude]))),
        kind: 'track',
      });
      sourceRef.current.addFeature(line);
    }
    locations.forEach((loc) => {
      sourceRef.current.addFeature(
        new Feature({
          geometry: new Point(fromLonLat([loc.longitude, loc.latitude])),
          kind: 'ship',
          shipId: loc.shipId,
        }),
      );
    });
    battleUnits.forEach((unit) => {
      const center = fromLonLat([unit.longitude, unit.latitude]);
      sourceRef.current.addFeature(
        new Feature({
          geometry: new CircleGeom(center, Math.max(unit.radarRangeKm, 1) * 1000),
          kind: 'radarRange',
          side: unit.side,
        }),
      );
      sourceRef.current.addFeature(
        new Feature({
          geometry: new Point(center),
          kind: 'battleUnit',
          side: unit.side,
          status: unit.status,
          label: `${unit.side.toUpperCase()} ${Math.max(0, Math.round(unit.hp))}`,
        }),
      );
    });
    radarTargets.forEach((target) => {
      sourceRef.current.addFeature(
        new Feature({
          geometry: new Point(fromLonLat([target.longitude, target.latitude])),
          kind: 'radarTarget',
          side: target.side,
          detected: target.detected,
        }),
      );
    });
    projectiles.forEach((projectile) => {
      const target = unitById.get(projectile.targetUnitId);
      if (target) {
        sourceRef.current.addFeature(
          new Feature({
            geometry: new LineString([fromLonLat([projectile.longitude, projectile.latitude]), fromLonLat([target.longitude, target.latitude])]),
            kind: 'projectileTrack',
            side: projectile.side,
          }),
        );
      }
      sourceRef.current.addFeature(
        new Feature({
          geometry: new Point(fromLonLat([projectile.longitude, projectile.latitude])),
          kind: 'projectile',
          side: projectile.side,
          status: projectile.status,
        }),
      );
    });
    const firstProjectile = projectiles[0];
    const firstUnit = battleUnits[0];
    const firstTarget = radarTargets[0];
    const first = locations[0] ?? track[0] ?? firstUnit ?? firstTarget ?? firstProjectile;
    if (!first || !mapRef.current) {
      return;
    }
    if (fitMode === 'fit-data') {
      const extent = sourceRef.current.getExtent();
      if (extent && !isEmptyExtent(extent)) {
        mapRef.current.updateSize();
        mapRef.current.getView().fit(extent, {
          duration: 300,
          padding: [48, 48, 48, 48],
          maxZoom: 13,
        });
        return;
      }
    }
    mapRef.current.getView().animate({ center: fromLonLat([first.longitude, first.latitude]), duration: 300 });
  }, [locations, track, battleUnits, radarTargets, projectiles, fitMode]);

  return <div ref={nodeRef} className="monitor-map" />;
}

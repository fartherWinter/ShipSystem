export type FrontendAuthMode = "off" | "token" | "proxy";

export const apiBase = normalizeEnvString(import.meta.env.VITE_API_BASE, "http://localhost:8080").replace(/\/+$/, "");

export const authMode: FrontendAuthMode = normalizeAuthMode(import.meta.env.VITE_AUTH_MODE);

export const mapTileUrl = normalizeEnvString(import.meta.env.VITE_MAP_TILE_URL, "https://tile.openstreetmap.org/{z}/{x}/{y}.png");

export const mapTileAttribution = normalizeEnvString(import.meta.env.VITE_MAP_TILE_ATTRIBUTION, "OpenStreetMap");

function normalizeAuthMode(value: unknown): FrontendAuthMode {
  if (value === "token" || value === "proxy" || value === "off") {
    return value;
  }
  return "off";
}

function normalizeEnvString(value: unknown, fallback: string) {
  return typeof value === "string" && value.trim() !== "" ? value.trim() : fallback;
}

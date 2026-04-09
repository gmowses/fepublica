// Typed client for the Fé Pública HTTP API.
// All endpoints return JSON. All methods throw on non-2xx.

const BASE = import.meta.env.DEV ? "" : "";

export interface Source {
  id: string;
  name: string;
  base_url: string;
  description?: string;
  created_at: string;
}

export interface Snapshot {
  id: number;
  source_id: string;
  collected_at: string;
  api_version?: string;
  record_count: number;
  bytes_size: number;
  merkle_root?: string;
  merkle_computed_at?: string;
  collector_version: string;
  notes?: string;
}

export interface Anchor {
  id: number;
  snapshot_id: number;
  calendar_url: string;
  submitted_at: string;
  upgraded: boolean;
  upgraded_at?: string;
  block_height?: number;
  receipt_bytes: number;
}

export interface EventMeta {
  id: number;
  external_id: string;
  content_hash: string;
}

export interface EventFull {
  id: number;
  snapshot_id: number;
  source_id: string;
  external_id: string;
  content_hash: string;
  canonical_json: unknown;
}

export interface DiffItem {
  external_id: string;
  hash_a?: string;
  hash_b?: string;
}

export interface DiffResult {
  source_id: string;
  snapshot_a: { id: number; collected_at: string; record_count: number };
  snapshot_b: { id: number; collected_at: string; record_count: number };
  summary: { added: number; removed: number; changed: number };
  added: DiffItem[] | null;
  removed: DiffItem[] | null;
  changed: DiffItem[] | null;
}

export interface HealthInfo {
  status: string;
  version: string;
  uptime: string;
  now: string;
}

async function get<T>(path: string): Promise<T> {
  const res = await fetch(BASE + path, {
    headers: { Accept: "application/json" },
  });
  if (!res.ok) {
    const body = await res.text().catch(() => "");
    throw new Error(`HTTP ${res.status} on ${path}: ${body.slice(0, 200)}`);
  }
  return res.json();
}

export const api = {
  health: () => get<HealthInfo>("/api/health"),
  sources: () => get<{ sources: Source[] }>("/api/sources"),
  snapshots: (params: { source?: string; limit?: number } = {}) => {
    const q = new URLSearchParams();
    if (params.source) q.set("source", params.source);
    if (params.limit) q.set("limit", String(params.limit));
    return get<{ snapshots: Snapshot[] }>(
      "/api/snapshots" + (q.toString() ? "?" + q : "")
    );
  },
  snapshot: (id: number) => get<Snapshot>(`/api/snapshots/${id}`),
  snapshotAnchors: (id: number) =>
    get<{ anchors: Anchor[] }>(`/api/snapshots/${id}/anchors`),
  snapshotEvents: (
    id: number,
    params: { limit?: number; offset?: number; search?: string } = {}
  ) => {
    const q = new URLSearchParams();
    if (params.limit) q.set("limit", String(params.limit));
    if (params.offset) q.set("offset", String(params.offset));
    if (params.search) q.set("search", params.search);
    return get<{
      snapshot_id: number;
      total: number;
      limit: number;
      offset: number;
      search: string;
      events: EventMeta[];
    }>(`/api/snapshots/${id}/events?${q.toString()}`);
  },
  event: (snapshotId: number, externalId: string) =>
    get<EventFull>(
      `/api/snapshots/${snapshotId}/events/${encodeURIComponent(externalId)}`
    ),
  proof: (snapshotId: number, externalId: string) =>
    get<unknown>(
      `/api/snapshots/${snapshotId}/events/${encodeURIComponent(externalId)}/proof`
    ),
  diff: (a: number, b: number) =>
    get<DiffResult>(`/api/snapshots/${a}/diff/${b}`),
};

export function shortHash(h?: string, n = 10): string {
  if (!h) return "—";
  return h.length > n + 2 ? h.slice(0, n) + "…" : h;
}

export function formatBytes(b: number): string {
  if (b < 1024) return `${b} B`;
  if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`;
  if (b < 1024 * 1024 * 1024) return `${(b / (1024 * 1024)).toFixed(2)} MB`;
  return `${(b / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

export function formatDate(s?: string): string {
  if (!s) return "—";
  try {
    return new Date(s).toLocaleString("pt-BR", {
      dateStyle: "short",
      timeStyle: "short",
    });
  } catch {
    return s;
  }
}

export function formatNumber(n?: number): string {
  if (n == null) return "—";
  return n.toLocaleString("pt-BR");
}

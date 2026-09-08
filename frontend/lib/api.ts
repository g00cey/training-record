// サーバ専用: backend への fetch ラッパ。
// Route Handler（app/bff/[...path]）や サーバコンポーネントから使う。
// ブラウザからは絶対に呼ばない（API キーがサーバ env にしかない）。
import 'server-only';

const INTERNAL_API_BASE = (
  process.env.INTERNAL_API_BASE ?? 'http://backend:8080/api'
).replace(/\/+$/, '');

const API_KEY = process.env.API_KEY ?? '';

export type ProxyResult = {
  status: number;
  body: ArrayBuffer;
  contentType: string | null;
};

/**
 * backend へリクエストを丸ごと転送する（Route Handler 用）。
 * backend 未到達なら 502 相当の JSON を返す（例外を投げない）。
 */
export async function proxyToBackend(
  method: string,
  path: string,
  search: string,
  contentType: string | null,
  body: ArrayBuffer | undefined,
): Promise<ProxyResult> {
  const url = `${INTERNAL_API_BASE}/${path.replace(/^\/+/, '')}${search}`;
  const headers = new Headers();
  headers.set('accept', 'application/json');
  if (contentType) headers.set('content-type', contentType);
  if (API_KEY) headers.set('authorization', `Bearer ${API_KEY}`);

  try {
    const res = await fetch(url, {
      method,
      headers,
      body,
      cache: 'no-store',
    });
    return {
      status: res.status,
      body: await res.arrayBuffer(),
      contentType: res.headers.get('content-type'),
    };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    const payload = JSON.stringify({
      error: {
        code: 'backend_unreachable',
        message: `backend に接続できません (${INTERNAL_API_BASE}): ${message}`,
      },
    });
    return {
      status: 502,
      body: new TextEncoder().encode(payload).buffer as ArrayBuffer,
      contentType: 'application/json; charset=utf-8',
    };
  }
}

/** サーバコンポーネントから型付きで取得したい場合のヘルパ（任意）。 */
export async function serverFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const url = `${INTERNAL_API_BASE}/${path.replace(/^\/+/, '')}`;
  const headers = new Headers(init?.headers);
  headers.set('accept', 'application/json');
  if (API_KEY) headers.set('authorization', `Bearer ${API_KEY}`);
  const res = await fetch(url, { ...init, headers, cache: 'no-store' });
  const text = await res.text();
  const data = text ? JSON.parse(text) : null;
  if (!res.ok) {
    const msg = data?.error?.message ?? `HTTP ${res.status}`;
    throw new Error(msg);
  }
  return data as T;
}

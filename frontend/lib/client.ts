'use client';

// ブラウザ側のデータ取得。常に同一オリジンの /bff プロキシ（app/bff/[...path]）を叩く。
// backend の URL も API キーもここには出てこない。
// nginx 配下では /api/* は backend 直送（Hermes 専用）なので、ブラウザ経路は /bff に分ける。

export const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? '/bff';

export class ApiError extends Error {
  status: number;
  code: string;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

async function parse(res: Response) {
  const text = await res.text();
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch {
    return { raw: text };
  }
}

/** SWR 用 fetcher。path は "/strength-sessions" のように先頭スラッシュ付き。 */
export async function fetcher<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { accept: 'application/json' },
  });
  const data = await parse(res);
  if (!res.ok) {
    const code = data?.error?.code ?? 'error';
    const message = data?.error?.message ?? `リクエストに失敗しました (HTTP ${res.status})`;
    throw new ApiError(res.status, code, message);
  }
  return data as T;
}

export type MutationMethod = 'POST' | 'PUT' | 'PATCH' | 'DELETE';

/** 書き込み系。204 は null を返す。 */
export async function apiMutate<T = unknown>(
  path: string,
  method: MutationMethod,
  body?: unknown,
): Promise<T | null> {
  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers: body === undefined
      ? { accept: 'application/json' }
      : { accept: 'application/json', 'content-type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await parse(res);
  if (!res.ok) {
    const code = data?.error?.code ?? 'error';
    const message = data?.error?.message ?? `更新に失敗しました (HTTP ${res.status})`;
    throw new ApiError(res.status, code, message);
  }
  return (data as T) ?? null;
}

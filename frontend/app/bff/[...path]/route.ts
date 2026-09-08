import { NextRequest, NextResponse } from 'next/server';
import { proxyToBackend } from '@/lib/api';

// ブラウザ → 同一オリジン /bff/** → ここ → backend(INTERNAL_API_BASE) に Bearer 付きで転送。
// API キーはサーバ env のみ。ブラウザには出さない。
// nginx では /api/* は backend 直送（Hermes 用）なので、ブラウザ経路は別パス /bff にする。

export const dynamic = 'force-dynamic';
export const runtime = 'nodejs';

async function handle(
  req: NextRequest,
  ctx: { params: Promise<{ path?: string[] }> },
): Promise<NextResponse> {
  const { path = [] } = await ctx.params;
  const method = req.method;
  const hasBody = method !== 'GET' && method !== 'HEAD' && method !== 'DELETE';
  const body = hasBody ? await req.arrayBuffer() : undefined;

  const result = await proxyToBackend(
    method,
    path.join('/'),
    req.nextUrl.search,
    req.headers.get('content-type'),
    body && body.byteLength > 0 ? body : undefined,
  );

  // 204 / 205 / 304 はボディ禁止（undici の Response コンストラクタが例外を投げる）。
  // ボディ空の場合も null を渡す。
  const noBody =
    result.status === 204 ||
    result.status === 205 ||
    result.status === 304 ||
    result.body.byteLength === 0;

  if (noBody) {
    return new NextResponse(null, { status: result.status });
  }

  const headers = new Headers();
  headers.set(
    'content-type',
    result.contentType ?? 'application/json; charset=utf-8',
  );
  return new NextResponse(result.body, { status: result.status, headers });
}

export const GET = handle;
export const POST = handle;
export const PUT = handle;
export const PATCH = handle;
export const DELETE = handle;

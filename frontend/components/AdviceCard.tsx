'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import useSWR from 'swr';
import { fetcher } from '@/lib/client';
import type { Advice } from '@/lib/types';
import {
  FREQ_STATUS,
  OVERLOAD_STATUS,
  ZONE_BADGE_CLASS,
  ZONE_LABEL,
  formatPct,
} from '@/lib/domain';
import { Card, ErrorBox, Spinner, cn } from './ui';

function excerpt(s: string, max = 44): string {
  const t = (s ?? '').replace(/\s+/g, ' ').trim();
  return t.length > max ? `${t.slice(0, max)}…` : t;
}

function wStr(v: number | null): string {
  return v === null || v === undefined ? '自重' : `${v}kg`;
}

export function AdviceCard({ compact = false }: { compact?: boolean }) {
  const { data, error, isLoading, mutate } = useSWR<Advice>('/advice', fetcher);

  if (compact) {
    if (isLoading || error || !data) return null; // 一覧トップでは静かに省略
    return <AdviceCompact advice={data} />;
  }

  return (
    <Card>
      <h3 className="mb-3 font-semibold text-gray-800">故障予防アドバイス</h3>
      {error && <ErrorBox error={error} onRetry={() => mutate()} />}
      {isLoading && !error && <Spinner />}
      {data && !error && <AdviceBody advice={data} />}
    </Card>
  );
}

function Chip({ className, children }: { className: string; children: ReactNode }) {
  return <span className={cn('badge', className)}>{children}</span>;
}

function AdviceCompact({ advice }: { advice: Advice }) {
  const { acwr, frequency, deload, warningSigns } = advice;
  const flagged = warningSigns.flaggedSessions.length;
  return (
    <Card>
      <div className="flex items-center justify-between gap-2">
        <h3 className="font-semibold text-gray-800">故障予防アドバイス</h3>
        <Link href="/volume" className="text-xs text-brand-700 hover:underline">
          詳細
        </Link>
      </div>
      <div className="mt-2 flex flex-wrap items-center gap-2 text-xs">
        <Chip className={ZONE_BADGE_CLASS[acwr.zone]}>
          ACWR {acwr.zone === 'no_data' ? '—' : acwr.value.toFixed(2)}・
          {ZONE_LABEL[acwr.zone]}
        </Chip>
        <Chip className={FREQ_STATUS[frequency.status].cls}>
          頻度 {FREQ_STATUS[frequency.status].label}（{frequency.total}回/
          {frequency.windowDays}日）
        </Chip>
        {deload.due && (
          <Chip className="bg-amber-100 text-amber-900 border-amber-300">
            デロード推奨
          </Chip>
        )}
        {flagged > 0 && (
          <Chip className="bg-red-100 text-red-800 border-red-200">
            警告サイン {flagged}件
          </Chip>
        )}
      </div>
    </Card>
  );
}

function AdviceBody({ advice }: { advice: Advice }) {
  const {
    asOf,
    acwr,
    frequency,
    progressiveOverload: po,
    deload,
    watchExercises,
    warningSigns,
  } = advice;

  return (
    <div className="space-y-4 text-sm">
      <p className="text-xs text-gray-400">{asOf} 時点</p>

      {/* ACWR */}
      <section>
        <div className="flex items-center gap-2">
          <span className="font-medium text-gray-700">急性:慢性負荷比 (ACWR)</span>
          <Chip className={ZONE_BADGE_CLASS[acwr.zone]}>
            {acwr.zone === 'no_data' ? '—' : acwr.value.toFixed(2)}・
            {ZONE_LABEL[acwr.zone]}
          </Chip>
        </div>
        <p className="mt-1 text-xs text-gray-500 tabular-nums">
          急性7日 {Math.round(acwr.acute7d).toLocaleString()} / 慢性(週){' '}
          {Math.round(acwr.chronicWeekly).toLocaleString()}
        </p>
      </section>

      {/* 頻度 */}
      <section>
        <div className="flex items-center gap-2">
          <span className="font-medium text-gray-700">トレーニング頻度</span>
          <Chip className={FREQ_STATUS[frequency.status].cls}>
            {FREQ_STATUS[frequency.status].label}
          </Chip>
        </div>
        <p className="mt-1 text-gray-600">{frequency.message}</p>
        <p className="mt-0.5 text-xs text-gray-500 tabular-nums">
          直近{frequency.windowDays}日: 計 {frequency.total} 回（筋トレ{' '}
          {frequency.strengthSessions} / スピン {frequency.spinSessions}）
          {frequency.maxConsecutiveWithin24h > 1 &&
            ` ・24h未満の連続 ${frequency.maxConsecutiveWithin24h} 回`}
        </p>
      </section>

      {/* プログレッシブオーバーロード */}
      <section>
        <div className="flex items-center gap-2">
          <span className="font-medium text-gray-700">
            プログレッシブオーバーロード
          </span>
          <Chip className={OVERLOAD_STATUS[po.status].cls}>
            {OVERLOAD_STATUS[po.status].label}
          </Chip>
          <span className="text-xs text-gray-500">
            週間ボリューム比 {formatPct(po.weeklyVolumeChangePct)}
          </span>
        </div>
        {po.exercises.length > 0 && (
          <ul className="mt-2 space-y-1">
            <li className="text-xs font-medium text-gray-500">増量した種目</li>
            {po.exercises.map((ex, i) => (
              <li
                key={`${ex.name}-${i}`}
                className={cn(
                  'rounded border px-2 py-1 text-xs tabular-nums',
                  ex.status === 'warning'
                    ? 'border-red-200 bg-red-50 text-red-800'
                    : 'border-gray-200 bg-gray-50 text-gray-700',
                )}
              >
                {ex.name}: {wStr(ex.prevWeight)} → {wStr(ex.latestWeight)}（
                {formatPct(ex.changePct)}）
                {ex.lastIncreasedOn && ` ・${ex.lastIncreasedOn}`}
              </li>
            ))}
          </ul>
        )}
      </section>

      {/* デロード */}
      <section>
        {deload.due ? (
          <div className="rounded-md border border-amber-300 bg-amber-50 p-3 text-amber-900">
            <p className="font-semibold">デロード週を推奨</p>
            <p className="mt-1 text-sm">{deload.message}</p>
          </div>
        ) : (
          <p className="text-xs text-gray-500">
            前回デロード:{' '}
            {deload.lastDeloadDate
              ? `${deload.lastDeloadDate}（${deload.weeksSince ?? '?'}週前）`
              : '記録なし'}
          </p>
        )}
      </section>

      {/* 要チェック種目 */}
      {watchExercises.length > 0 && (
        <section>
          <p className="font-medium text-gray-700">要チェック種目（フォーム注意）</p>
          <ul className="mt-1 space-y-1">
            {watchExercises.map((ex, i) => (
              <li
                key={`${ex.name}-${i}`}
                className="rounded border border-amber-200 bg-amber-50 px-2 py-1 text-xs text-amber-900 tabular-nums"
              >
                {ex.name}: {wStr(ex.from)} → {wStr(ex.to)}
                {ex.on && `（${ex.on}）`}
              </li>
            ))}
          </ul>
          <p className="mt-1 text-xs text-gray-500">
            重量を上げた高リスク種目です。詳しくはフォームガイドを参照してください。
          </p>
        </section>
      )}

      {/* 警告サイン */}
      <section>
        <p className="font-medium text-gray-700">警告サイン</p>
        {warningSigns.flaggedSessions.length > 0 ? (
          <ul className="mt-1 space-y-1">
            {warningSigns.flaggedSessions.map((s, i) => (
              <li
                key={`${s.date}-${i}`}
                className="rounded border border-red-200 bg-red-50 px-2 py-1 text-xs text-red-800"
              >
                <Link href={`/sessions/${s.date}`} className="font-medium underline">
                  {s.date}
                </Link>
                : {s.matched.join('、')} を記録（{excerpt(s.notes)}）
              </li>
            ))}
          </ul>
        ) : (
          <p className="mt-1 text-xs text-gray-500">
            直近28日の記録に痛み・違和感の記述はありません。
          </p>
        )}

        <details className="mt-2 text-xs">
          <summary className="cursor-pointer text-gray-500">
            チェックリスト（即中止 / 経過観察）
          </summary>
          <div className="mt-2 grid gap-3 sm:grid-cols-2">
            <div>
              <p className="font-medium text-red-700">即座に中止すべき</p>
              <ul className="mt-1 list-disc space-y-0.5 pl-4 text-gray-600">
                {warningSigns.stopNow.map((x, i) => (
                  <li key={i}>{x}</li>
                ))}
              </ul>
            </div>
            <div>
              <p className="font-medium text-amber-700">注意深く経過観察</p>
              <ul className="mt-1 list-disc space-y-0.5 pl-4 text-gray-600">
                {warningSigns.monitor.map((x, i) => (
                  <li key={i}>{x}</li>
                ))}
              </ul>
            </div>
          </div>
        </details>
      </section>
    </div>
  );
}

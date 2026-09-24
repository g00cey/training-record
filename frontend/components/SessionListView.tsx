'use client';

import { useMemo, useState } from 'react';
import useSWR from 'swr';
import Link from 'next/link';
import { fetcher } from '@/lib/client';
import type {
  SpinSession,
  SpinSessionList,
  StrengthSession,
  StrengthSessionList,
} from '@/lib/types';
import { guessKind, KIND_BADGE_CLASS, KIND_LABEL } from '@/lib/domain';
import { longLabel } from '@/lib/date';
import { AdviceCard } from './AdviceCard';
import { SpinNotes } from './SpinNotes';
import {
  Button,
  Card,
  EmptyState,
  ErrorBox,
  Field,
  LinkButton,
  SectionTitle,
  Spinner,
  cn,
} from './ui';

type Row = {
  date: string;
  strength?: StrengthSession;
  spin?: SpinSession;
};

function buildQuery(from: string, to: string): string {
  const p = new URLSearchParams();
  if (from) p.set('from', from);
  if (to) p.set('to', to);
  p.set('limit', '365');
  return `?${p.toString()}`;
}

export function SessionListView() {
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [applied, setApplied] = useState({ from: '', to: '' });

  const qs = buildQuery(applied.from, applied.to);

  const strength = useSWR<StrengthSessionList>(
    `/strength-sessions${qs}`,
    fetcher,
  );
  const spin = useSWR<SpinSessionList>(`/spin-sessions${qs}`, fetcher);

  const rows = useMemo<Row[]>(() => {
    const map = new Map<string, Row>();
    for (const s of strength.data?.items ?? []) {
      map.set(s.date, { ...(map.get(s.date) ?? { date: s.date }), strength: s });
    }
    for (const s of spin.data?.items ?? []) {
      map.set(s.date, { ...(map.get(s.date) ?? { date: s.date }), spin: s });
    }
    return [...map.values()].sort((a, b) => (a.date < b.date ? 1 : -1));
  }, [strength.data, spin.data]);

  const loading = strength.isLoading || spin.isLoading;
  const error = strength.error || spin.error;

  const retry = () => {
    strength.mutate();
    spin.mutate();
  };

  return (
    <div className="space-y-4">
      <SectionTitle
        action={
          <div className="flex gap-2">
            <LinkButton href="/sessions/new" variant="primary">
              筋トレを記録
            </LinkButton>
            <LinkButton href="/spin/new">スピンを記録</LinkButton>
          </div>
        }
      >
        セッション一覧
      </SectionTitle>

      <Card>
        <div className="flex flex-wrap items-end gap-3">
          <div className="w-40">
            <Field label="開始日">
              <input
                type="date"
                className="input"
                value={from}
                onChange={(e) => setFrom(e.target.value)}
              />
            </Field>
          </div>
          <div className="w-40">
            <Field label="終了日">
              <input
                type="date"
                className="input"
                value={to}
                onChange={(e) => setTo(e.target.value)}
              />
            </Field>
          </div>
          <Button
            variant="primary"
            onClick={() => setApplied({ from, to })}
          >
            絞り込む
          </Button>
          <Button
            variant="ghost"
            onClick={() => {
              setFrom('');
              setTo('');
              setApplied({ from: '', to: '' });
            }}
          >
            クリア
          </Button>
        </div>
      </Card>

      <AdviceCard compact />

      {error && <ErrorBox error={error} onRetry={retry} />}
      {loading && !error && <Spinner />}
      {!loading && !error && rows.length === 0 && (
        <EmptyState>この期間の記録はありません。</EmptyState>
      )}

      <div className="space-y-3">
        {rows.map((row) => (
          <SessionRow key={row.date} row={row} />
        ))}
      </div>
    </div>
  );
}

function SessionRow({ row }: { row: Row }) {
  const kind = row.strength ? guessKind(row.strength.notes) : null;
  return (
    <Card>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <Link
            href={`/sessions/${row.date}`}
            className="text-base font-semibold text-brand-700 hover:underline"
          >
            {longLabel(row.date)}
          </Link>
          {kind && (
            <span className={cn('badge', KIND_BADGE_CLASS[kind])}>
              {KIND_LABEL[kind]}
            </span>
          )}
          {row.spin && (
            <span className="badge border-sky-200 bg-sky-100 text-sky-800">
              スピン
            </span>
          )}
        </div>
        <div className="flex gap-2 text-sm">
          {row.strength ? (
            <Link
              href={`/sessions/${row.date}/edit`}
              className="text-gray-600 hover:underline"
            >
              筋トレを編集
            </Link>
          ) : (
            <Link
              href={`/sessions/new?date=${row.date}`}
              className="text-gray-600 hover:underline"
            >
              筋トレを追加
            </Link>
          )}
          {row.spin ? (
            <Link
              href={`/spin/${row.date}/edit`}
              className="text-gray-600 hover:underline"
            >
              スピンを編集
            </Link>
          ) : (
            <Link
              href={`/spin/new?date=${row.date}`}
              className="text-gray-600 hover:underline"
            >
              スピンを追加
            </Link>
          )}
        </div>
      </div>

      {row.strength && (
        <div className="mt-3 overflow-x-auto">
          <table className="w-full min-w-[480px] text-sm">
            <thead>
              <tr className="text-left text-xs text-gray-500">
                <th className="py-1 pr-3 font-medium">種目</th>
                <th className="py-1 pr-3 font-medium">重量</th>
                <th className="py-1 pr-3 font-medium">回数</th>
                <th className="py-1 pr-3 font-medium">セット</th>
                <th className="py-1 font-medium">メモ</th>
              </tr>
            </thead>
            <tbody>
              {row.strength.exercises.map((ex) => (
                <tr key={ex.id} className="border-t border-gray-100">
                  <td className="py-1 pr-3">{ex.name}</td>
                  <td className="py-1 pr-3 tabular-nums">
                    {ex.weight === null ? '自重' : `${ex.weight} kg`}
                  </td>
                  <td className="py-1 pr-3 tabular-nums">{ex.reps}</td>
                  <td className="py-1 pr-3 tabular-nums">{ex.sets}</td>
                  <td className="py-1 text-gray-500">{ex.notes}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {row.strength.notes && (
            <p className="mt-2 text-xs text-gray-500">notes: {row.strength.notes}</p>
          )}
        </div>
      )}

      {row.spin && (
        <div className="mt-3 rounded-md bg-sky-50 p-3 text-sm text-sky-900">
          <span className="font-medium">スピンバイク</span>{' '}
          {row.spin.durationMinutes} 分
          {row.spin.avgHeartRate != null && ` / 平均 ${row.spin.avgHeartRate} bpm`}
          {row.spin.maxHeartRate != null && ` / 最大 ${row.spin.maxHeartRate} bpm`}
          {row.spin.rpe != null && ` / RPE ${row.spin.rpe}`}
          {row.spin.distanceKm != null && ` / ${row.spin.distanceKm} km`}
          <SpinNotes
            notes={row.spin.notes}
            restClassName="mt-1 whitespace-pre-wrap text-xs text-sky-700"
          />
        </div>
      )}
    </Card>
  );
}

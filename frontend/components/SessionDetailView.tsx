'use client';

import useSWR from 'swr';
import { ApiError, fetcher } from '@/lib/client';
import type { SpinSession, StrengthSession } from '@/lib/types';
import { guessKind, KIND_BADGE_CLASS, KIND_LABEL } from '@/lib/domain';
import { longLabel } from '@/lib/date';
import { SpinNotes } from './SpinNotes';
import {
  Card,
  EmptyState,
  ErrorBox,
  LinkButton,
  SectionTitle,
  Spinner,
  cn,
} from './ui';

function useOptional<T>(key: string) {
  return useSWR<T | null>(key, async (k: string) => {
    try {
      return await fetcher<T>(k);
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) return null;
      throw e;
    }
  });
}

export function SessionDetailView({ date }: { date: string }) {
  const strength = useOptional<StrengthSession>(`/strength-sessions/${date}`);
  const spin = useOptional<SpinSession>(`/spin-sessions/${date}`);

  const loading = strength.isLoading || spin.isLoading;
  const error = strength.error || spin.error;

  if (loading) return <Spinner />;
  if (error) {
    return (
      <ErrorBox
        error={error}
        onRetry={() => {
          strength.mutate();
          spin.mutate();
        }}
      />
    );
  }

  const s = strength.data;
  const sp = spin.data;
  const kind = s ? guessKind(s.notes) : null;

  return (
    <div className="space-y-4">
      <SectionTitle
        action={
          <div className="flex gap-2">
            {s ? (
              <LinkButton href={`/sessions/${date}/edit`}>筋トレを編集</LinkButton>
            ) : (
              <LinkButton href={`/sessions/new?date=${date}`}>
                筋トレを追加
              </LinkButton>
            )}
            {sp ? (
              <LinkButton href={`/spin/${date}/edit`}>スピンを編集</LinkButton>
            ) : (
              <LinkButton href={`/spin/new?date=${date}`}>スピンを追加</LinkButton>
            )}
          </div>
        }
      >
        {longLabel(date)}
      </SectionTitle>

      {!s && !sp && <EmptyState>この日の記録はありません。</EmptyState>}

      {s && (
        <Card>
          <div className="mb-3 flex items-center gap-2">
            <h3 className="font-semibold text-gray-800">筋力トレーニング</h3>
            {kind && (
              <span className={cn('badge', KIND_BADGE_CLASS[kind])}>
                {KIND_LABEL[kind]}
              </span>
            )}
          </div>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[480px] text-sm">
              <thead>
                <tr className="text-left text-xs text-gray-500">
                  <th className="py-1 pr-3 font-medium">#</th>
                  <th className="py-1 pr-3 font-medium">種目</th>
                  <th className="py-1 pr-3 font-medium">重量</th>
                  <th className="py-1 pr-3 font-medium">回数</th>
                  <th className="py-1 pr-3 font-medium">セット</th>
                  <th className="py-1 font-medium">メモ</th>
                </tr>
              </thead>
              <tbody>
                {s.exercises.map((ex, i) => (
                  <tr key={ex.id} className="border-t border-gray-100">
                    <td className="py-1 pr-3 tabular-nums text-gray-400">
                      {i + 1}
                    </td>
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
          </div>
          {s.notes && (
            <p className="mt-3 text-sm text-gray-600">notes: {s.notes}</p>
          )}
        </Card>
      )}

      {sp && (
        <Card>
          <h3 className="mb-3 font-semibold text-gray-800">スピンバイク</h3>
          <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm sm:grid-cols-3">
            <Info label="時間" value={`${sp.durationMinutes} 分`} />
            <Info
              label="平均心拍"
              value={sp.avgHeartRate != null ? `${sp.avgHeartRate} bpm` : '—'}
            />
            <Info
              label="最大心拍"
              value={sp.maxHeartRate != null ? `${sp.maxHeartRate} bpm` : '—'}
            />
            <Info label="RPE" value={sp.rpe != null ? String(sp.rpe) : '—'} />
            <Info
              label="距離"
              value={sp.distanceKm != null ? `${sp.distanceKm} km` : '—'}
            />
          </dl>
          <SpinNotes
            notes={sp.notes}
            restClassName="mt-3 whitespace-pre-wrap text-sm text-gray-600"
          />
        </Card>
      )}
    </div>
  );
}

function Info({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-xs text-gray-500">{label}</dt>
      <dd className="font-medium tabular-nums">{value}</dd>
    </div>
  );
}

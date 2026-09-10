'use client';

import { useMemo, useState, type ReactNode } from 'react';
import dynamic from 'next/dynamic';
import useSWR from 'swr';
import { fetcher } from '@/lib/client';
import type {
  ExercisesList,
  LoadReport,
  Summary,
  VolumeExerciseResponse,
  VolumeSessionResponse,
  VolumeWeekResponse,
  WeeklySummary,
} from '@/lib/types';
import { AcwrGauge } from './AcwrGauge';
import { AdviceCard } from './AdviceCard';
import {
  Card,
  EmptyState,
  ErrorBox,
  SectionTitle,
  Spinner,
} from './ui';

const TrendChart = dynamic(() => import('./charts/TrendChart'), {
  ssr: false,
  loading: () => <Spinner label="グラフを描画中…" />,
});

type Granularity = 'session' | 'week';
type ExerciseMetric = 'volumeLoad' | 'weight' | 'reps';

const METRIC_LABEL: Record<ExerciseMetric, string> = {
  volumeLoad: 'Volume Load',
  weight: '重量(kg)',
  reps: '回数',
};

export function VolumeDashboard() {
  const [granularity, setGranularity] = useState<Granularity>('session');
  const [exercise, setExercise] = useState('');
  const [metric, setMetric] = useState<ExerciseMetric>('volumeLoad');

  const exercisesList = useSWR<ExercisesList>('/exercises', fetcher);

  const sessionVol = useSWR<VolumeSessionResponse>(
    granularity === 'session' ? '/volume?granularity=session' : null,
    fetcher,
  );
  const weekVol = useSWR<VolumeWeekResponse>(
    granularity === 'week' ? '/volume?granularity=week' : null,
    fetcher,
  );
  const exerciseVol = useSWR<VolumeExerciseResponse>(
    exercise ? `/volume?exercise=${encodeURIComponent(exercise)}` : null,
    fetcher,
  );

  const loadReport = useSWR<LoadReport>('/load-report', fetcher);
  const weekly = useSWR<WeeklySummary>('/summary/weekly', fetcher);
  const summary = useSWR<Summary>('/summary?days=30', fetcher);

  const volData = useMemo(() => {
    if (granularity === 'session') {
      return (sessionVol.data?.points ?? []).map((p) => ({
        x: p.date,
        volumeLoad: p.volumeLoad,
      }));
    }
    return (weekVol.data?.points ?? []).map((p) => ({
      x: p.weekStart,
      volumeLoad: p.volumeLoad,
    }));
  }, [granularity, sessionVol.data, weekVol.data]);

  const exData = useMemo(
    () =>
      (exerciseVol.data?.points ?? []).map((p) => ({
        x: p.date,
        volumeLoad: p.volumeLoad,
        weight: p.weight,
        reps: p.reps,
      })),
    [exerciseVol.data],
  );

  const volError =
    granularity === 'session' ? sessionVol.error : weekVol.error;
  const volLoading =
    granularity === 'session' ? sessionVol.isLoading : weekVol.isLoading;

  return (
    <div className="space-y-6">
      <SectionTitle>ボリューム & 負荷</SectionTitle>

      {summary.data && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <Stat
            label="直近30日 筋トレ"
            value={`${summary.data.strengthSessionsInPeriod} 回`}
          />
          <Stat
            label="直近30日 スピン"
            value={`${summary.data.spinSessionsInPeriod} 回`}
          />
          <Stat
            label="累計 筋トレ"
            value={`${summary.data.totalStrengthSessions} 回`}
          />
          <Stat
            label="最終 筋トレ日"
            value={summary.data.latestStrengthDate ?? '—'}
          />
        </div>
      )}

      <Card>
        <div className="mb-3 flex items-center justify-between gap-2">
          <h3 className="font-semibold text-gray-800">Volume Load 推移</h3>
          <div className="flex gap-1 text-sm">
            <ToggleButton
              active={granularity === 'session'}
              onClick={() => setGranularity('session')}
            >
              セッション
            </ToggleButton>
            <ToggleButton
              active={granularity === 'week'}
              onClick={() => setGranularity('week')}
            >
              週
            </ToggleButton>
          </div>
        </div>
        {volError && (
          <ErrorBox
            error={volError}
            onRetry={() =>
              granularity === 'session' ? sessionVol.mutate() : weekVol.mutate()
            }
          />
        )}
        {volLoading && !volError && <Spinner />}
        {!volLoading && !volError && (
          <TrendChart
            data={volData}
            xKey="x"
            series={[
              { key: 'volumeLoad', name: 'Volume Load', color: '#3a57e8' },
            ]}
            yTickFormatter={(v) => `${Math.round(v / 1000)}k`}
          />
        )}
      </Card>

      <Card>
        <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
          <h3 className="font-semibold text-gray-800">種目別推移</h3>
          <div className="flex flex-wrap gap-2">
            <select
              className="input w-52"
              value={exercise}
              onChange={(e) => setExercise(e.target.value)}
            >
              <option value="">種目を選択…</option>
              {(exercisesList.data?.exercises ?? []).map((n) => (
                <option key={n} value={n}>
                  {n}
                </option>
              ))}
            </select>
            <div className="flex gap-1 text-sm">
              {(Object.keys(METRIC_LABEL) as ExerciseMetric[]).map((mk) => (
                <ToggleButton
                  key={mk}
                  active={metric === mk}
                  onClick={() => setMetric(mk)}
                >
                  {METRIC_LABEL[mk]}
                </ToggleButton>
              ))}
            </div>
          </div>
        </div>
        {!exercise && <EmptyState>種目を選ぶと推移を表示します。</EmptyState>}
        {exercise && exerciseVol.error && (
          <ErrorBox
            error={exerciseVol.error}
            onRetry={() => exerciseVol.mutate()}
          />
        )}
        {exercise && exerciseVol.isLoading && !exerciseVol.error && <Spinner />}
        {exercise && !exerciseVol.isLoading && !exerciseVol.error && (
          <TrendChart
            data={exData}
            xKey="x"
            series={[
              {
                key: metric,
                name: METRIC_LABEL[metric],
                color: '#2c40c4',
              },
            ]}
          />
        )}
      </Card>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <h3 className="mb-3 font-semibold text-gray-800">ACWR（急性:慢性）</h3>
          {loadReport.error && (
            <ErrorBox
              error={loadReport.error}
              onRetry={() => loadReport.mutate()}
            />
          )}
          {loadReport.isLoading && !loadReport.error && <Spinner />}
          {loadReport.data && (
            <div className="space-y-3">
              <AcwrGauge
                acwr={loadReport.data.acwr}
                zone={loadReport.data.zone}
              />
              <dl className="grid grid-cols-2 gap-2 text-sm">
                <Mini
                  label="急性 7日 合計負荷"
                  value={Math.round(
                    loadReport.data.acute7d.total,
                  ).toLocaleString()}
                />
                <Mini
                  label="慢性 28日 週平均VL"
                  value={Math.round(
                    loadReport.data.chronic28d.weeklyVolumeLoad,
                  ).toLocaleString()}
                />
              </dl>
              {loadReport.data.recommendations?.length > 0 && (
                <ul className="list-disc space-y-1 pl-5 text-sm text-gray-600">
                  {loadReport.data.recommendations.map((r, i) => (
                    <li key={i}>{r}</li>
                  ))}
                </ul>
              )}
            </div>
          )}
        </Card>

        <Card>
          <h3 className="mb-3 font-semibold text-gray-800">週次サマリー</h3>
          {weekly.error && (
            <ErrorBox error={weekly.error} onRetry={() => weekly.mutate()} />
          )}
          {weekly.isLoading && !weekly.error && <Spinner />}
          {weekly.data && (
            <div className="space-y-3 text-sm">
              <p className="text-gray-500">{weekly.data.period}</p>
              <p>
                合計トレーニング:{' '}
                <span className="font-semibold">
                  {weekly.data.summary.totalTrainingSessions} 回
                </span>
              </p>
              {weekly.data.evaluation?.length > 0 && (
                <ul className="space-y-1">
                  {weekly.data.evaluation.map((e, i) => (
                    <li key={i}>{e}</li>
                  ))}
                </ul>
              )}
              {weekly.data.advice?.length > 0 && (
                <ul className="list-disc space-y-1 pl-5 text-gray-600">
                  {weekly.data.advice.map((a, i) => (
                    <li key={i}>{a}</li>
                  ))}
                </ul>
              )}
            </div>
          )}
        </Card>
      </div>

      <AdviceCard />
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="card p-3">
      <p className="text-xs text-gray-500">{label}</p>
      <p className="mt-1 text-lg font-semibold tabular-nums">{value}</p>
    </div>
  );
}

function Mini({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-xs text-gray-500">{label}</dt>
      <dd className="font-medium tabular-nums">{value}</dd>
    </div>
  );
}

function ToggleButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={
        active
          ? 'rounded-md bg-brand-600 px-3 py-1 text-white'
          : 'rounded-md border border-gray-300 bg-white px-3 py-1 text-gray-700 hover:bg-gray-50'
      }
    >
      {children}
    </button>
  );
}

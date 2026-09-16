'use client';

import useSWR from 'swr';
import { fetcher, ApiError } from '@/lib/client';
import type { TrainingEvaluation, TrainingEvaluationList } from '@/lib/types';
import { Card, ErrorBox, Spinner, EmptyState } from './ui';

function EvaluationBody({ evaluation }: { evaluation: TrainingEvaluation }) {
  return (
    <div className="space-y-3 text-sm">
      <p className="text-xs text-gray-400">
        {evaluation.periodFrom} 〜 {evaluation.periodTo} の評価
      </p>
      <p>{evaluation.summary}</p>
      {evaluation.strengths.length > 0 && (
        <div>
          <p className="font-medium text-emerald-700">良い点</p>
          <ul className="mt-1 list-disc space-y-0.5 pl-5 text-gray-700">
            {evaluation.strengths.map((s, i) => (
              <li key={i}>{s}</li>
            ))}
          </ul>
        </div>
      )}
      {evaluation.concerns.length > 0 && (
        <div>
          <p className="font-medium text-amber-700">懸念点</p>
          <ul className="mt-1 list-disc space-y-0.5 pl-5 text-gray-700">
            {evaluation.concerns.map((s, i) => (
              <li key={i}>{s}</li>
            ))}
          </ul>
        </div>
      )}
      {evaluation.suggestions.length > 0 && (
        <div>
          <p className="font-medium text-brand-700">提案</p>
          <ul className="mt-1 list-disc space-y-0.5 pl-5 text-gray-700">
            {evaluation.suggestions.map((s, i) => (
              <li key={i}>{s}</li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function BimonthlyEvaluationCard() {
  const { data, error, isLoading, mutate } = useSWR<TrainingEvaluation>(
    '/training-evaluations/bimonthly',
    fetcher,
  );
  const notYetRun = error instanceof ApiError && error.status === 404;

  return (
    <Card>
      <h3 className="mb-3 font-semibold text-gray-800">AIトレーニング評価（2ヶ月）</h3>
      {error && !notYetRun && <ErrorBox error={error} onRetry={() => mutate()} />}
      {notYetRun && <EmptyState>まだ評価がありません（毎日自動実行されます）。</EmptyState>}
      {isLoading && !error && <Spinner />}
      {data && !error && <EvaluationBody evaluation={data} />}
    </Card>
  );
}

function BiweeklyEvaluationHistoryCard() {
  const { data, error, isLoading, mutate } = useSWR<TrainingEvaluationList>(
    '/training-evaluations/biweekly',
    fetcher,
  );

  return (
    <Card>
      <h3 className="mb-3 font-semibold text-gray-800">AIトレーニング評価（2週間・履歴）</h3>
      {error && <ErrorBox error={error} onRetry={() => mutate()} />}
      {isLoading && !error && <Spinner />}
      {data && !error && data.evaluations.length === 0 && (
        <EmptyState>まだ評価がありません（毎日自動実行されます）。</EmptyState>
      )}
      {data && !error && data.evaluations.length > 0 && (
        <div className="max-h-96 space-y-2 overflow-y-auto">
          {data.evaluations.map((evaluation, i) => (
            <details key={`${evaluation.periodTo}-${i}`} className="rounded border border-gray-200 p-2" open={i === 0}>
              <summary className="cursor-pointer text-sm">
                <span className="text-gray-500">
                  {evaluation.periodFrom} 〜 {evaluation.periodTo}
                </span>
                {' — '}
                {evaluation.summary}
              </summary>
              <div className="mt-2">
                <EvaluationBody evaluation={evaluation} />
              </div>
            </details>
          ))}
        </div>
      )}
    </Card>
  );
}

export function TrainingEvaluationSection() {
  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <BimonthlyEvaluationCard />
      <BiweeklyEvaluationHistoryCard />
    </div>
  );
}

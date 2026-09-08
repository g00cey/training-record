'use client';

import { useMemo, useState } from 'react';
import useSWR from 'swr';
import { fetcher } from '@/lib/client';
import type { CalendarResponse } from '@/lib/types';
import {
  KIND_BADGE_CLASS,
  KIND_LABEL,
} from '@/lib/domain';
import {
  currentMonthJST,
  monthGrid,
  shiftMonth,
  todayJST,
  WEEKDAY_HEADERS,
} from '@/lib/date';
import { Button, Card, ErrorBox, LinkButton, Spinner, cn } from './ui';

export function CalendarView() {
  const [month, setMonth] = useState(currentMonthJST());
  const today = todayJST();

  const { data, error, isLoading, mutate } = useSWR<CalendarResponse>(
    `/calendar?month=${month}`,
    fetcher,
  );

  const byDate = useMemo(() => {
    const map = new Map<string, CalendarResponse['days'][number]>();
    for (const d of data?.days ?? []) map.set(d.date, d);
    return map;
  }, [data]);

  const weeks = useMemo(() => monthGrid(month), [month]);
  const [y, m] = month.split('-');

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">
          {y}年{parseInt(m, 10)}月
        </h1>
        <div className="flex gap-2">
          <Button onClick={() => setMonth(shiftMonth(month, -1))}>← 前月</Button>
          <Button onClick={() => setMonth(currentMonthJST())}>今月</Button>
          <Button onClick={() => setMonth(shiftMonth(month, 1))}>翌月 →</Button>
        </div>
      </div>

      {error && <ErrorBox error={error} onRetry={() => mutate()} />}
      {isLoading && !error && <Spinner />}

      {!error && (
        <Card className="p-2">
          <div className="grid grid-cols-7 gap-1 text-center text-xs font-medium text-gray-500">
            {WEEKDAY_HEADERS.map((w) => (
              <div key={w} className="py-1">
                {w}
              </div>
            ))}
          </div>
          <div className="mt-1 grid grid-cols-7 gap-1">
            {weeks.flat().map((cell) => {
              const info = byDate.get(cell.date);
              const isToday = cell.date === today;
              return (
                <a
                  key={cell.date}
                  href={`/sessions/${cell.date}`}
                  className={cn(
                    'flex min-h-[84px] flex-col rounded-md border p-1 text-left transition hover:border-brand-400',
                    cell.inMonth ? 'bg-white' : 'bg-gray-50 text-gray-400',
                    isToday ? 'border-brand-500 ring-1 ring-brand-300' : 'border-gray-200',
                  )}
                >
                  <span
                    className={cn(
                      'text-xs tabular-nums',
                      isToday && 'font-bold text-brand-700',
                    )}
                  >
                    {cell.day}
                  </span>
                  <span className="mt-1 flex flex-col gap-0.5">
                    {info?.strength && (
                      <span
                        className={cn(
                          'badge justify-center px-1 py-0 text-[10px]',
                          KIND_BADGE_CLASS[info.strength.kind],
                        )}
                        title={`${info.strength.exerciseCount}種目 / VL ${Math.round(
                          info.strength.volumeLoad,
                        ).toLocaleString()}`}
                      >
                        {KIND_LABEL[info.strength.kind]}
                      </span>
                    )}
                    {info?.spin && (
                      <span className="badge justify-center border-sky-200 bg-sky-100 px-1 py-0 text-[10px] text-sky-800">
                        スピン{info.spin.rpe != null ? ` R${info.spin.rpe}` : ''}
                      </span>
                    )}
                  </span>
                </a>
              );
            })}
          </div>
        </Card>
      )}

      <div className="flex flex-wrap gap-3 text-xs text-gray-500">
        <Legend className={KIND_BADGE_CLASS.bodyweight_only} label="自重のみ" />
        <Legend className={KIND_BADGE_CLASS.fw_only} label="FWのみ" />
        <Legend className={KIND_BADGE_CLASS.bodyweight_and_fw} label="自重＋FW" />
        <Legend
          className="border-sky-200 bg-sky-100 text-sky-800"
          label="スピン"
        />
      </div>

      <p className="text-sm">
        <LinkButton href="/sessions/new">その他の日を記録</LinkButton>
      </p>
    </div>
  );
}

function Legend({ className, label }: { className: string; label: string }) {
  return (
    <span className="flex items-center gap-1">
      <span className={cn('badge px-1 py-0', className)}>&nbsp;</span>
      {label}
    </span>
  );
}

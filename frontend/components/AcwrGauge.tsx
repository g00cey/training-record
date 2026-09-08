'use client';

import type { LoadReportZone } from '@/lib/types';
import { ZONE_COLOR, ZONE_LABEL } from '@/lib/domain';

// ACWR を 0〜2.0 のスケールでバー表示。ゾーン境界: 0.6 / 0.8 / 1.3 / 1.5
const MAX = 2.0;
const STOPS = [
  { to: 0.6, color: ZONE_COLOR.low },
  { to: 0.8, color: ZONE_COLOR.slightly_low },
  { to: 1.3, color: ZONE_COLOR.safe },
  { to: 1.5, color: ZONE_COLOR.caution },
  { to: MAX, color: ZONE_COLOR.warning },
];

export function AcwrGauge({
  acwr,
  zone,
}: {
  acwr: number;
  zone: LoadReportZone;
}) {
  const clamped = Math.max(0, Math.min(MAX, acwr || 0));
  const pct = (clamped / MAX) * 100;

  return (
    <div>
      <div className="flex items-baseline gap-2">
        <span className="text-3xl font-bold tabular-nums">
          {zone === 'no_data' ? '—' : acwr.toFixed(2)}
        </span>
        <span
          className="rounded-full px-2 py-0.5 text-xs font-medium text-white"
          style={{ backgroundColor: ZONE_COLOR[zone] }}
        >
          {ZONE_LABEL[zone]}
        </span>
      </div>
      <div className="relative mt-3 h-3 w-full overflow-hidden rounded-full">
        <div className="absolute inset-0 flex">
          {STOPS.map((s, i) => {
            const from = i === 0 ? 0 : STOPS[i - 1].to;
            const width = ((s.to - from) / MAX) * 100;
            return (
              <div
                key={s.to}
                style={{ width: `${width}%`, backgroundColor: s.color }}
                className="opacity-40"
              />
            );
          })}
        </div>
        {zone !== 'no_data' && (
          <div
            className="absolute top-1/2 h-5 w-1 -translate-y-1/2 rounded bg-gray-900"
            style={{ left: `calc(${pct}% - 2px)` }}
          />
        )}
      </div>
      <div className="mt-1 flex justify-between text-[10px] text-gray-400">
        <span>0</span>
        <span>0.8</span>
        <span>1.3</span>
        <span>1.5</span>
        <span>2.0+</span>
      </div>
    </div>
  );
}

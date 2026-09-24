'use client';

import {
  HR_ZONE_COLOR,
  HR_ZONES,
  mmSsToSeconds,
  parseHrZones,
} from '@/lib/domain';

/**
 * スピンセッションの notes を「心拍ゾーン内訳バー」と「自由記述」に分けて表示する。
 * ゾーン内訳部分は全体の運動時間を 100% としたバーグラフ + 凡例で表示し、
 * それ以外の自由記述はテキストのまま表示する。
 */
export function SpinNotes({
  notes,
  restClassName,
}: {
  notes: string | null | undefined;
  restClassName: string;
}) {
  const { zones, rest } = parseHrZones(notes);
  const seconds = HR_ZONES.map((z) => mmSsToSeconds(zones[z]));
  const total = seconds.reduce((a, b) => a + b, 0);

  if (total <= 0 && !rest) return null;

  return (
    <>
      {total > 0 && (
        <div className="mt-3">
          <div className="mb-1 text-xs text-gray-500">心拍ゾーン内訳</div>
          <div className="flex h-3 w-full overflow-hidden rounded-full bg-gray-100">
            {HR_ZONES.map((z, i) =>
              seconds[i] > 0 ? (
                <div
                  key={z}
                  style={{
                    width: `${(seconds[i] / total) * 100}%`,
                    backgroundColor: HR_ZONE_COLOR[z],
                  }}
                  title={`${z} ${zones[z]}`}
                />
              ) : null,
            )}
          </div>
          <div className="mt-1.5 flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-gray-500">
            {HR_ZONES.map((z, i) => (
              <span key={z} className="inline-flex items-center gap-1">
                <span
                  className="inline-block h-2 w-2 rounded-full"
                  style={{ backgroundColor: HR_ZONE_COLOR[z] }}
                />
                {z} {zones[z] || '0:00'}（{Math.round((seconds[i] / total) * 100)}%）
              </span>
            ))}
          </div>
        </div>
      )}
      {rest && <p className={restClassName}>{rest}</p>}
    </>
  );
}

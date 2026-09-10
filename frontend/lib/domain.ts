// ドメイン表現（docs/domain.md 準拠）。

import type {
  FrequencyStatus,
  LoadReportZone,
  OverloadStatus,
  SessionKind,
} from './types';

// --- 記録区分（notes プリセット） --------------------------------------------

export const NOTES_PRESETS = ['自重のみ', '自重＋フリーウェイト', 'FWのみ'] as const;

// --- kind ラベル ------------------------------------------------------------

export const KIND_LABEL: Record<SessionKind, string> = {
  bodyweight_only: '自重のみ',
  fw_only: 'FWのみ',
  bodyweight_and_fw: '自重＋FW',
  other: 'その他',
};

export const KIND_BADGE_CLASS: Record<SessionKind, string> = {
  bodyweight_only: 'bg-emerald-100 text-emerald-800 border-emerald-200',
  fw_only: 'bg-amber-100 text-amber-800 border-amber-200',
  bodyweight_and_fw: 'bg-brand-100 text-brand-800 border-brand-200',
  other: 'bg-gray-100 text-gray-700 border-gray-200',
};

/** notes 文字列から kind を推定（backend が付ける kind が無い一覧表示などで使う）。 */
export function guessKind(notes: string | null | undefined): SessionKind {
  const n = notes ?? '';
  const hasBw = n.includes('自重');
  const hasFw = n.includes('フリーウェイト') || n.includes('ＦＷ') || /(^|[^A-Za-z])FW/.test(n);
  if (hasBw && hasFw) return 'bodyweight_and_fw';
  if (hasFw) return 'fw_only';
  if (hasBw) return 'bodyweight_only';
  return 'other';
}

// --- ACWR ゾーン ----------------------------------------------------------

export const ZONE_LABEL: Record<LoadReportZone, string> = {
  safe: '適切',
  caution: 'やや高め',
  warning: '高リスク',
  low: '低い',
  slightly_low: 'やや低い',
  no_data: 'データ不足',
};

export const ZONE_COLOR: Record<LoadReportZone, string> = {
  safe: '#10b981',
  caution: '#f59e0b',
  warning: '#ef4444',
  low: '#6366f1',
  slightly_low: '#3b82f6',
  no_data: '#9ca3af',
};

/** ACWR ゾーンのバッジ用 Tailwind クラス（domain.md の色分け）。 */
export const ZONE_BADGE_CLASS: Record<LoadReportZone, string> = {
  safe: 'bg-emerald-100 text-emerald-800 border-emerald-200',
  caution: 'bg-amber-100 text-amber-800 border-amber-200',
  warning: 'bg-red-100 text-red-800 border-red-200',
  low: 'bg-gray-100 text-gray-700 border-gray-200',
  slightly_low: 'bg-gray-100 text-gray-700 border-gray-200',
  no_data: 'bg-gray-50 text-gray-500 border-gray-200',
};

// --- 故障予防アドバイス（Phase 6） ----------------------------------------

export const FREQ_STATUS: Record<
  FrequencyStatus,
  { label: string; cls: string }
> = {
  good: { label: '良好', cls: 'bg-emerald-100 text-emerald-800 border-emerald-200' },
  low: { label: '頻度不足', cls: 'bg-amber-100 text-amber-800 border-amber-200' },
  rest_needed: {
    label: '休息不足',
    cls: 'bg-red-100 text-red-800 border-red-200',
  },
  long_off: {
    label: '長期オフ',
    cls: 'bg-gray-100 text-gray-700 border-gray-200',
  },
};

export const OVERLOAD_STATUS: Record<
  OverloadStatus,
  { label: string; cls: string }
> = {
  ok: { label: '適正', cls: 'bg-emerald-100 text-emerald-800 border-emerald-200' },
  caution: {
    label: 'やや過剰',
    cls: 'bg-amber-100 text-amber-800 border-amber-200',
  },
  warning: { label: '過剰', cls: 'bg-red-100 text-red-800 border-red-200' },
};

/** ±つきパーセント表示。null は "—"。 */
export function formatPct(n: number | null | undefined): string {
  if (n === null || n === undefined || !Number.isFinite(n)) return '—';
  const sign = n > 0 ? '+' : '';
  return `${sign}${n.toFixed(1)}%`;
}

// --- 心拍ゾーン内訳（スピン notes の慣習フォーマット） ----------------------
// 例: 心拍ゾーン内訳: ウォームアップ7:25/インテンシブ9:04/有酸素6:48/無酸素12:05/最大酸素摂取量(高負荷)4:42

export const HR_ZONES = [
  'ウォームアップ',
  'インテンシブ',
  '有酸素',
  '無酸素',
  '最大酸素摂取量(高負荷)',
] as const;

export type HrZoneKey = (typeof HR_ZONES)[number];

export type HrZoneValues = Record<HrZoneKey, string>; // "mm:ss" or ""

export function emptyHrZones(): HrZoneValues {
  return HR_ZONES.reduce((acc, z) => {
    acc[z] = '';
    return acc;
  }, {} as HrZoneValues);
}

const HR_PREFIX = '心拍ゾーン内訳:';

/** notes から「心拍ゾーン内訳」部分をパースして {zones, rest} に分ける。 */
export function parseHrZones(notes: string | null | undefined): {
  zones: HrZoneValues;
  rest: string;
} {
  const zones = emptyHrZones();
  const text = notes ?? '';
  const idx = text.indexOf(HR_PREFIX);
  if (idx < 0) return { zones, rest: text.trim() };

  // 内訳部分は行末（改行）または文字列末まで
  const after = text.slice(idx + HR_PREFIX.length);
  const nlPos = after.indexOf('\n');
  const body = (nlPos >= 0 ? after.slice(0, nlPos) : after).trim();
  const rest = (
    text.slice(0, idx) + (nlPos >= 0 ? after.slice(nlPos + 1) : '')
  ).trim();

  for (const part of body.split('/')) {
    const seg = part.trim();
    const match = seg.match(/^(.*?)(\d{1,3}:\d{2})$/);
    if (!match) continue;
    const label = match[1].trim();
    const value = match[2];
    const zone = HR_ZONES.find((z) => z === label);
    if (zone) zones[zone] = value;
  }
  return { zones, rest };
}

/** ゾーン入力を慣習フォーマット文字列に組み立てる。全部空なら空文字。 */
export function buildHrZoneString(zones: HrZoneValues): string {
  const parts = HR_ZONES.filter((z) => zones[z] && zones[z].trim()).map(
    (z) => `${z}${zones[z].trim()}`,
  );
  if (parts.length === 0) return '';
  return `${HR_PREFIX} ${parts.join('/')}`;
}

/** 自由記述 + ゾーン文字列を 1 つの notes にまとめる。 */
export function composeSpinNotes(freeText: string, zones: HrZoneValues): string {
  const zoneStr = buildHrZoneString(zones);
  const free = freeText.trim();
  if (free && zoneStr) return `${free}\n${zoneStr}`;
  return free || zoneStr;
}

/** "mm:ss" の緩いバリデーション（空は許可）。 */
export function isValidMmSs(v: string): boolean {
  if (!v || !v.trim()) return true;
  return /^\d{1,3}:[0-5]\d$/.test(v.trim());
}

// --- RPE 自動算出（表示用の目安） -----------------------------------------

export function estimateRpe(
  avg: number | null | undefined,
  max: number | null | undefined,
): number | null {
  if (!avg || !max || max <= 0) return null;
  return Math.min(10, Math.max(1, Math.round((avg / max) * 10)));
}

// --- 自重種目リスト（Volume Load 計算の参考。domain.md） -----------------

export const BODYWEIGHT_EXERCISES = [
  '懸垂',
  '懸垂レッグレイズ',
  'ディップス',
  'バックエクステンション',
  'ベンチレッグレイズ',
];

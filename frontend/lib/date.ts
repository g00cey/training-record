// 日付ユーティリティ。
// 方針: date は "YYYY-MM-DD" のただの文字列。Date に変換して戻す加工はしない（TZ ずれ防止）。
// 曜日計算だけ Date.UTC を使う（表示専用、日付文字列は触らない）。

export const TZ = 'Asia/Tokyo';

const WEEKDAY_JA = ['日', '月', '火', '水', '木', '金', '土'];

/** JST の今日 "YYYY-MM-DD"。 */
export function todayJST(): string {
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(new Date());
}

/** JST の今月 "YYYY-MM"。 */
export function currentMonthJST(): string {
  return todayJST().slice(0, 7);
}

type YMD = { y: number; m: number; d: number };

export function parseDate(date: string): YMD {
  const [y, m, d] = date.split('-').map((x) => parseInt(x, 10));
  return { y, m, d };
}

export function parseMonth(month: string): { y: number; m: number } {
  const [y, m] = month.split('-').map((x) => parseInt(x, 10));
  return { y, m };
}

function pad(n: number): string {
  return String(n).padStart(2, '0');
}

export function formatMonth(y: number, m: number): string {
  return `${y}-${pad(m)}`;
}

export function formatDate(y: number, m: number, d: number): string {
  return `${y}-${pad(m)}-${pad(d)}`;
}

/** "YYYY-MM" を delta ヶ月ずらす。 */
export function shiftMonth(month: string, delta: number): string {
  const { y, m } = parseMonth(month);
  const total = (y * 12 + (m - 1)) + delta;
  const ny = Math.floor(total / 12);
  const nm = (total % 12) + 1;
  return formatMonth(ny, nm);
}

/** 曜日インデックス 0=日 .. 6=土（表示専用）。 */
export function weekdayIndex(date: string): number {
  const { y, m, d } = parseDate(date);
  return new Date(Date.UTC(y, m - 1, d)).getUTCDay();
}

export function daysInMonth(y: number, m: number): number {
  return new Date(Date.UTC(y, m, 0)).getUTCDate();
}

export type MonthCell = {
  date: string;
  day: number;
  inMonth: boolean;
};

/**
 * 月グリッド（週の始まり = 月曜）。前後の月のこぼれ日も埋めて 6 週ぶんの配列を返す。
 */
export function monthGrid(month: string): MonthCell[][] {
  const { y, m } = parseMonth(month);
  const total = daysInMonth(y, m);
  // 月初の曜日を月曜起点(0=月 .. 6=日)に変換
  const firstDow = (weekdayIndex(formatDate(y, m, 1)) + 6) % 7;

  const cells: MonthCell[] = [];

  // 前月のこぼれ
  const prevMonth = shiftMonth(month, -1);
  const { y: py, m: pm } = parseMonth(prevMonth);
  const prevTotal = daysInMonth(py, pm);
  for (let i = firstDow - 1; i >= 0; i--) {
    const d = prevTotal - i;
    cells.push({ date: formatDate(py, pm, d), day: d, inMonth: false });
  }

  // 当月
  for (let d = 1; d <= total; d++) {
    cells.push({ date: formatDate(y, m, d), day: d, inMonth: true });
  }

  // 翌月のこぼれ（6 週 = 42 セルまで）
  const nextMonth = shiftMonth(month, 1);
  const { y: ny, m: nm } = parseMonth(nextMonth);
  let nd = 1;
  while (cells.length % 7 !== 0 || cells.length < 42) {
    cells.push({ date: formatDate(ny, nm, nd), day: nd, inMonth: false });
    nd++;
    if (cells.length >= 42) break;
  }

  const weeks: MonthCell[][] = [];
  for (let i = 0; i < cells.length; i += 7) {
    weeks.push(cells.slice(i, i + 7));
  }
  return weeks;
}

/** "2026-09-07" -> "9/7(月)"。 */
export function shortLabel(date: string): string {
  const { m, d } = parseDate(date);
  return `${m}/${d}(${WEEKDAY_JA[weekdayIndex(date)]})`;
}

/** "2026-09-07" -> "2026年9月7日(月)"。 */
export function longLabel(date: string): string {
  const { y, m, d } = parseDate(date);
  return `${y}年${m}月${d}日(${WEEKDAY_JA[weekdayIndex(date)]})`;
}

export const WEEKDAY_HEADERS = ['月', '火', '水', '木', '金', '土', '日'];

'use client';

import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';

export type TrendSeries = {
  key: string;
  name: string;
  color: string;
};

export type TrendChartProps = {
  data: Array<Record<string, string | number | null>>;
  xKey: string;
  series: TrendSeries[];
  height?: number;
  yTickFormatter?: (v: number) => string;
};

export default function TrendChart({
  data,
  xKey,
  series,
  height = 280,
  yTickFormatter,
}: TrendChartProps) {
  if (!data || data.length === 0) {
    return (
      <div className="flex h-[200px] items-center justify-center text-sm text-gray-400">
        データがありません
      </div>
    );
  }
  return (
    <ResponsiveContainer width="100%" height={height}>
      <LineChart data={data} margin={{ top: 8, right: 16, bottom: 8, left: 8 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
        <XAxis
          dataKey={xKey}
          tick={{ fontSize: 11 }}
          minTickGap={20}
          stroke="#9ca3af"
        />
        <YAxis
          tick={{ fontSize: 11 }}
          stroke="#9ca3af"
          tickFormatter={
            yTickFormatter
              ? (v: number) => yTickFormatter(v)
              : (v: number) => `${v}`
          }
          width={56}
        />
        <Tooltip
          formatter={(value: number | string) =>
            typeof value === 'number' ? value.toLocaleString() : value
          }
        />
        {series.map((s) => (
          <Line
            key={s.key}
            type="monotone"
            dataKey={s.key}
            name={s.name}
            stroke={s.color}
            strokeWidth={2}
            dot={{ r: 2 }}
            activeDot={{ r: 4 }}
            connectNulls
          />
        ))}
      </LineChart>
    </ResponsiveContainer>
  );
}

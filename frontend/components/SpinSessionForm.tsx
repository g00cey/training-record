'use client';

import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { useForm, type Resolver } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import useSWR from 'swr';
import { apiMutate, ApiError, fetcher } from '@/lib/client';
import { numOrNull, spinFormSchema } from '@/lib/schemas';
import {
  composeSpinNotes,
  emptyHrZones,
  estimateRpe,
  HR_ZONES,
  parseHrZones,
  type HrZoneValues,
} from '@/lib/domain';
import type { SpinSession } from '@/lib/types';
import { longLabel, todayJST } from '@/lib/date';
import {
  Button,
  Card,
  ErrorBox,
  Field,
  SectionTitle,
  Spinner,
  Toast,
} from './ui';

type FormValues = {
  date: string;
  durationMinutes: string;
  avgHeartRate: string;
  maxHeartRate: string;
  rpe: string;
  distanceKm: string;
  freeNotes: string;
  hrZones: HrZoneValues;
};

const resolver = zodResolver(spinFormSchema) as unknown as Resolver<FormValues>;

function blankZones(): HrZoneValues {
  return emptyHrZones();
}

export function SpinSessionForm({
  mode,
  date: routeDate,
}: {
  mode: 'new' | 'edit';
  date?: string;
}) {
  const router = useRouter();
  const [initialDate, setInitialDate] = useState<string | null>(
    mode === 'edit' ? (routeDate ?? null) : null,
  );
  const [serverError, setServerError] = useState<string | null>(null);
  const [conflictDate, setConflictDate] = useState<string | null>(null);

  const existing = useSWR<SpinSession>(
    mode === 'edit' && routeDate ? `/spin-sessions/${routeDate}` : null,
    fetcher,
  );

  const defaultValues = useMemo<FormValues>(
    () => ({
      date: routeDate ?? todayJST(),
      durationMinutes: '',
      avgHeartRate: '',
      maxHeartRate: '',
      rpe: '',
      distanceKm: '',
      freeNotes: '',
      hrZones: blankZones(),
    }),
    [routeDate],
  );

  const form = useForm<FormValues>({ resolver, defaultValues, mode: 'onBlur' });
  const {
    register,
    handleSubmit,
    reset,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = form;

  useEffect(() => {
    if (mode === 'edit' && existing.data) {
      const s = existing.data;
      setInitialDate(s.date);
      const { zones, rest } = parseHrZones(s.notes);
      reset({
        date: s.date,
        durationMinutes: String(s.durationMinutes ?? ''),
        avgHeartRate: s.avgHeartRate != null ? String(s.avgHeartRate) : '',
        maxHeartRate: s.maxHeartRate != null ? String(s.maxHeartRate) : '',
        rpe: s.rpe != null ? String(s.rpe) : '',
        distanceKm: s.distanceKm != null ? String(s.distanceKm) : '',
        freeNotes: rest,
        hrZones: zones,
      });
    }
  }, [mode, existing.data, reset]);

  const avg = numOrNull(watch('avgHeartRate'));
  const max = numOrNull(watch('maxHeartRate'));
  const rpeGuess = estimateRpe(avg, max);

  const onSubmit = handleSubmit(async (values) => {
    setServerError(null);
    setConflictDate(null);

    const notes = composeSpinNotes(values.freeNotes ?? '', values.hrZones);
    const payload = {
      date: values.date,
      durationMinutes: numOrNull(values.durationMinutes) ?? 0,
      avgHeartRate: numOrNull(values.avgHeartRate),
      maxHeartRate: numOrNull(values.maxHeartRate),
      rpe: numOrNull(values.rpe),
      distanceKm: numOrNull(values.distanceKm),
      notes,
      // docs/api.md の body 例は snake_case のため互換用に併記する
      duration_minutes: numOrNull(values.durationMinutes) ?? 0,
      avg_heart_rate: numOrNull(values.avgHeartRate),
      max_heart_rate: numOrNull(values.maxHeartRate),
      distance_km: numOrNull(values.distanceKm),
    };

    try {
      if (mode === 'new') {
        const created = await apiMutate<SpinSession>('/spin-sessions', 'POST', payload);
        router.push(`/sessions/${created?.date ?? values.date}`);
        return;
      }
      const target = initialDate ?? routeDate ?? values.date;
      await apiMutate(`/spin-sessions/${target}`, 'PUT', payload);
      router.push(`/sessions/${target}`);
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setConflictDate(values.date);
        return;
      }
      setServerError(e instanceof Error ? e.message : String(e));
    }
  });

  async function onDelete() {
    if (!initialDate) return;
    if (
      !window.confirm(
        `${longLabel(initialDate)} のスピン記録を削除します。よろしいですか？`,
      )
    ) {
      return;
    }
    try {
      await apiMutate(`/spin-sessions/${initialDate}`, 'DELETE');
      router.push('/');
    } catch (e) {
      setServerError(e instanceof Error ? e.message : String(e));
    }
  }

  if (mode === 'edit' && existing.isLoading) return <Spinner />;
  if (mode === 'edit' && existing.error) {
    return <ErrorBox error={existing.error} onRetry={() => existing.mutate()} />;
  }

  return (
    <form onSubmit={onSubmit} className="space-y-4">
      <SectionTitle
        action={
          mode === 'edit' && initialDate ? (
            <Button type="button" variant="danger" onClick={onDelete}>
              この日を削除
            </Button>
          ) : null
        }
      >
        {mode === 'new' ? 'スピンを記録' : `スピンを編集（${routeDate}）`}
      </SectionTitle>

      {conflictDate && (
        <Toast kind="error">
          {conflictDate} は既にスピン記録があります。
          <Link href={`/spin/${conflictDate}/edit`} className="ml-1 underline">
            その日を編集
          </Link>
          してください。
        </Toast>
      )}
      {serverError && <Toast kind="error">{serverError}</Toast>}

      <Card className="grid gap-4 sm:grid-cols-3">
        <Field label="日付" error={errors.date?.message as string | undefined}>
          <input
            type="date"
            className="input"
            disabled={mode === 'edit'}
            {...register('date')}
          />
        </Field>
        <Field
          label="時間 (分)"
          error={errors.durationMinutes?.message as string | undefined}
        >
          <input
            type="number"
            inputMode="numeric"
            className="input"
            {...register('durationMinutes')}
          />
        </Field>
        <Field
          label="距離 (km・任意)"
          error={errors.distanceKm?.message as string | undefined}
        >
          <input
            type="number"
            step="0.1"
            inputMode="decimal"
            className="input"
            {...register('distanceKm')}
          />
        </Field>
        <Field
          label="平均心拍 (bpm)"
          error={errors.avgHeartRate?.message as string | undefined}
        >
          <input
            type="number"
            inputMode="numeric"
            className="input"
            {...register('avgHeartRate')}
          />
        </Field>
        <Field
          label="最大心拍 (bpm)"
          error={errors.maxHeartRate?.message as string | undefined}
        >
          <input
            type="number"
            inputMode="numeric"
            className="input"
            {...register('maxHeartRate')}
          />
        </Field>
        <Field
          label="RPE (1-10・任意)"
          error={errors.rpe?.message as string | undefined}
          hint={
            rpeGuess != null
              ? `未入力なら自動算出: 約 ${rpeGuess}（round(平均/最大×10)、1〜10 にクランプ）`
              : '未入力かつ平均/最大心拍があれば backend が自動算出します'
          }
        >
          <input
            type="number"
            inputMode="numeric"
            className="input"
            {...register('rpe')}
          />
        </Field>
      </Card>

      <Card className="space-y-3">
        <h3 className="font-semibold text-gray-800">心拍ゾーン内訳（任意）</h3>
        <p className="text-xs text-gray-500">
          各ゾーンの時間を mm:ss で入力すると、notes に
          「心拍ゾーン内訳: ウォームアップ7:25/…」の形式で保存します。
        </p>
        <div className="grid gap-3 sm:grid-cols-2">
          {HR_ZONES.map((zone) => (
            <Field
              key={zone}
              label={zone}
              error={errors.hrZones?.[zone]?.message as string | undefined}
            >
              <input
                className="input"
                placeholder="mm:ss"
                {...register(`hrZones.${zone}` as const)}
              />
            </Field>
          ))}
        </div>
        <Button
          type="button"
          variant="ghost"
          onClick={() => setValue('hrZones', blankZones())}
        >
          ゾーン入力をクリア
        </Button>
      </Card>

      <Card>
        <Field label="メモ（自由記述・任意）">
          <textarea className="input" rows={3} {...register('freeNotes')} />
        </Field>
      </Card>

      <div className="flex gap-2">
        <Button type="submit" variant="primary" disabled={isSubmitting}>
          {isSubmitting ? '保存中…' : mode === 'new' ? '記録する' : '保存する'}
        </Button>
        <Button type="button" variant="secondary" onClick={() => router.back()}>
          キャンセル
        </Button>
      </div>
    </form>
  );
}

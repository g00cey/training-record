'use client';

import { useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { useFieldArray, useForm, type Resolver } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import useSWR from 'swr';
import { apiMutate, ApiError, fetcher } from '@/lib/client';
import { numOrNull, strengthFormSchema } from '@/lib/schemas';
import { NOTES_PRESETS } from '@/lib/domain';
import type {
  ExercisesList,
  Routine,
  StrengthSession,
} from '@/lib/types';
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

type ExerciseRow = {
  name: string;
  bodyweight: boolean;
  weight: string;
  reps: string;
  sets: string;
  notes: string;
};

type FormValues = {
  date: string;
  notes: string;
  submitMode: 'replace' | 'append';
  exercises: ExerciseRow[];
};

const resolver = zodResolver(strengthFormSchema) as unknown as Resolver<FormValues>;

function emptyRow(): ExerciseRow {
  return { name: '', bodyweight: false, weight: '', reps: '', sets: '1', notes: '' };
}

function toRows(session: StrengthSession): ExerciseRow[] {
  return session.exercises.map((ex) => ({
    name: ex.name,
    bodyweight: ex.weight === null,
    weight: ex.weight === null ? '' : String(ex.weight),
    reps: String(ex.reps),
    sets: String(ex.sets ?? 1),
    notes: ex.notes ?? '',
  }));
}

export function StrengthSessionForm({
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

  const exercisesList = useSWR<ExercisesList>('/exercises', fetcher);
  const existing = useSWR<StrengthSession>(
    mode === 'edit' && routeDate ? `/strength-sessions/${routeDate}` : null,
    fetcher,
  );

  const [serverError, setServerError] = useState<string | null>(null);
  const [conflictDate, setConflictDate] = useState<string | null>(null);

  const defaultValues = useMemo<FormValues>(
    () => ({
      date: routeDate ?? todayJST(),
      notes: '',
      submitMode: 'replace',
      exercises: [emptyRow()],
    }),
    [routeDate],
  );

  const form = useForm<FormValues>({ resolver, defaultValues, mode: 'onBlur' });
  const {
    register,
    control,
    handleSubmit,
    reset,
    setValue,
    watch,
    formState: { errors, isSubmitting },
  } = form;
  const fieldArray = useFieldArray({ control, name: 'exercises' });

  const submitMode = watch('submitMode');

  // 既存セッション読み込み後にフォームへ流し込む
  useEffect(() => {
    if (mode === 'edit' && existing.data) {
      setInitialDate(existing.data.date);
      reset({
        date: existing.data.date,
        notes: existing.data.notes ?? '',
        submitMode: 'replace',
        exercises:
          existing.data.exercises.length > 0
            ? toRows(existing.data)
            : [emptyRow()],
      });
    }
  }, [mode, existing.data, reset]);

  const existingSession = existing.data;

  function switchMode(next: 'replace' | 'append') {
    setValue('submitMode', next);
    if (next === 'append') {
      fieldArray.replace([emptyRow()]);
    } else if (existingSession) {
      fieldArray.replace(
        existingSession.exercises.length > 0
          ? toRows(existingSession)
          : [emptyRow()],
      );
    }
  }

  function applyRoutine(routine: Routine) {
    fieldArray.replace(
      routine.exercises.map((ex) => ({
        name: ex.name,
        bodyweight: ex.weight === null,
        weight: ex.weight === null ? '' : String(ex.weight),
        reps: String(ex.reps),
        sets: String(ex.sets ?? 1),
        notes: '',
      })),
    );
  }

  async function loadRoutineIntoForm() {
    setServerError(null);
    try {
      const routine = await fetcher<Routine>('/routine');
      applyRoutine(routine);
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) {
        setServerError('ルーティンが未登録です。先に「ルーティン」画面で作成してください。');
      } else {
        setServerError(e instanceof Error ? e.message : String(e));
      }
    }
  }

  const onSubmit = handleSubmit(async (values) => {
    setServerError(null);
    setConflictDate(null);

    const exercisesPayload = values.exercises.map((row) => ({
      name: row.name.trim(),
      weight: row.bodyweight ? null : numOrNull(row.weight),
      reps: numOrNull(row.reps) ?? 0,
      sets: numOrNull(row.sets) ?? 1,
      notes: row.notes ?? '',
    }));

    try {
      if (mode === 'new') {
        const created = await apiMutate<StrengthSession>('/strength-sessions', 'POST', {
          date: values.date,
          notes: values.notes ?? '',
          exercises: exercisesPayload,
        });
        router.push(`/sessions/${created?.date ?? values.date}`);
        return;
      }

      // edit
      const target = initialDate ?? routeDate ?? values.date;
      if (values.submitMode === 'append') {
        await apiMutate(`/strength-sessions/${target}/exercises`, 'POST', {
          exercises: exercisesPayload,
          notes: values.notes ?? '',
        });
      } else {
        await apiMutate(`/strength-sessions/${target}`, 'PUT', {
          notes: values.notes ?? '',
          exercises: exercisesPayload,
        });
      }
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
    if (!window.confirm(`${longLabel(initialDate)} の筋トレ記録を削除します。よろしいですか？`)) {
      return;
    }
    setServerError(null);
    try {
      await apiMutate(`/strength-sessions/${initialDate}`, 'DELETE');
      router.push('/');
    } catch (e) {
      setServerError(e instanceof Error ? e.message : String(e));
    }
  }

  if (mode === 'edit' && existing.isLoading) return <Spinner />;
  if (mode === 'edit' && existing.error) {
    return <ErrorBox error={existing.error} onRetry={() => existing.mutate()} />;
  }

  const nameListId = 'exercise-name-list';

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
        {mode === 'new' ? '筋トレを記録' : `筋トレを編集（${routeDate}）`}
      </SectionTitle>

      <datalist id={nameListId}>
        {(exercisesList.data?.exercises ?? []).map((n) => (
          <option key={n} value={n} />
        ))}
      </datalist>

      {conflictDate && (
        <Toast kind="error">
          {conflictDate} は既に記録があります。
          <Link
            href={`/sessions/${conflictDate}/edit`}
            className="ml-1 underline"
          >
            その日を編集
          </Link>
          するか、編集画面で「追記」を選んでください。
        </Toast>
      )}
      {serverError && <Toast kind="error">{serverError}</Toast>}

      <Card className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <Field
            label="日付"
            error={errors.date?.message as string | undefined}
          >
            <input
              type="date"
              className="input"
              disabled={mode === 'edit'}
              {...register('date')}
            />
          </Field>
          <Field label="記録区分 (notes)">
            <input
              className="input"
              list="notes-preset-list"
              placeholder="自重のみ / 自重＋フリーウェイト など"
              {...register('notes')}
            />
            <datalist id="notes-preset-list">
              {NOTES_PRESETS.map((p) => (
                <option key={p} value={p} />
              ))}
            </datalist>
            <div className="mt-2 flex flex-wrap gap-2">
              {NOTES_PRESETS.map((p) => (
                <button
                  key={p}
                  type="button"
                  className="btn-ghost border border-gray-200 px-2 py-1 text-xs"
                  onClick={() => setValue('notes', p, { shouldDirty: true })}
                >
                  {p}
                </button>
              ))}
            </div>
          </Field>
        </div>

        {mode === 'edit' && (
          <div className="rounded-md bg-gray-50 p-3">
            <p className="mb-2 text-sm font-medium text-gray-700">
              既存の記録をどうする？
            </p>
            <div className="flex flex-wrap gap-4 text-sm">
              <label className="flex items-center gap-2">
                <input
                  type="radio"
                  checked={submitMode === 'replace'}
                  onChange={() => switchMode('replace')}
                />
                置換（下の内容で全種目を差し替え）
              </label>
              <label className="flex items-center gap-2">
                <input
                  type="radio"
                  checked={submitMode === 'append'}
                  onChange={() => switchMode('append')}
                />
                追記（下の種目を既存に追加）
              </label>
            </div>
            {submitMode === 'append' && existingSession && (
              <div className="mt-3 text-xs text-gray-500">
                <p className="font-medium">既存の種目（変更しません）:</p>
                <ul className="mt-1 list-disc pl-5">
                  {existingSession.exercises.map((ex) => (
                    <li key={ex.id}>
                      {ex.name} — {ex.weight === null ? '自重' : `${ex.weight}kg`} ×{' '}
                      {ex.reps} × {ex.sets}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}
      </Card>

      <Card className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h3 className="font-semibold text-gray-800">
            種目{submitMode === 'append' ? '（追記する分）' : ''}
          </h3>
          <div className="flex gap-2">
            <Button type="button" onClick={loadRoutineIntoForm}>
              プリセットから記録
            </Button>
            <Button
              type="button"
              variant="primary"
              onClick={() => fieldArray.append(emptyRow())}
            >
              種目を追加
            </Button>
          </div>
        </div>

        {typeof errors.exercises?.message === 'string' && (
          <p className="text-xs text-red-600">{errors.exercises.message}</p>
        )}

        <div className="space-y-3">
          {fieldArray.fields.map((f, i) => {
            const rowErr = errors.exercises?.[i];
            const isBw = watch(`exercises.${i}.bodyweight`);
            return (
              <div
                key={f.id}
                className="rounded-md border border-gray-200 p-3"
              >
                <div className="grid gap-3 sm:grid-cols-12">
                  <div className="sm:col-span-4">
                    <Field
                      label={`種目 ${i + 1}`}
                      error={rowErr?.name?.message as string | undefined}
                    >
                      <input
                        className="input"
                        list={nameListId}
                        placeholder="種目名"
                        {...register(`exercises.${i}.name`)}
                      />
                    </Field>
                  </div>
                  <div className="sm:col-span-2">
                    <Field
                      label="重量(kg)"
                      error={rowErr?.weight?.message as string | undefined}
                    >
                      <input
                        className="input"
                        type="number"
                        step="0.5"
                        inputMode="decimal"
                        disabled={!!isBw}
                        placeholder={isBw ? '自重' : ''}
                        {...register(`exercises.${i}.weight`)}
                      />
                    </Field>
                  </div>
                  <div className="sm:col-span-2">
                    <Field
                      label="回数"
                      error={rowErr?.reps?.message as string | undefined}
                    >
                      <input
                        className="input"
                        type="number"
                        inputMode="numeric"
                        {...register(`exercises.${i}.reps`)}
                      />
                    </Field>
                  </div>
                  <div className="sm:col-span-2">
                    <Field
                      label="セット"
                      error={rowErr?.sets?.message as string | undefined}
                    >
                      <input
                        className="input"
                        type="number"
                        inputMode="numeric"
                        {...register(`exercises.${i}.sets`)}
                      />
                    </Field>
                  </div>
                  <div className="flex items-end sm:col-span-2">
                    <label className="mb-2 flex items-center gap-2 text-sm">
                      <input
                        type="checkbox"
                        {...register(`exercises.${i}.bodyweight`)}
                        onChange={(e) => {
                          setValue(
                            `exercises.${i}.bodyweight`,
                            e.target.checked,
                            { shouldDirty: true },
                          );
                          if (e.target.checked) {
                            setValue(`exercises.${i}.weight`, '');
                          }
                        }}
                      />
                      自重
                    </label>
                  </div>
                </div>

                <div className="mt-2 grid gap-3 sm:grid-cols-12">
                  <div className="sm:col-span-8">
                    <input
                      className="input"
                      placeholder="種目メモ（任意）"
                      {...register(`exercises.${i}.notes`)}
                    />
                  </div>
                  <div className="flex gap-1 sm:col-span-4">
                    <Button
                      type="button"
                      variant="ghost"
                      className="px-2"
                      disabled={i === 0}
                      onClick={() => fieldArray.move(i, i - 1)}
                      aria-label="上へ"
                    >
                      ↑
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      className="px-2"
                      disabled={i === fieldArray.fields.length - 1}
                      onClick={() => fieldArray.move(i, i + 1)}
                      aria-label="下へ"
                    >
                      ↓
                    </Button>
                    <Button
                      type="button"
                      variant="danger"
                      className="px-2"
                      disabled={fieldArray.fields.length <= 1}
                      onClick={() => fieldArray.remove(i)}
                    >
                      削除
                    </Button>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </Card>

      <div className="flex gap-2">
        <Button type="submit" variant="primary" disabled={isSubmitting}>
          {isSubmitting
            ? '保存中…'
            : mode === 'new'
              ? '記録する'
              : submitMode === 'append'
                ? '追記する'
                : '置換して保存'}
        </Button>
        <Button
          type="button"
          variant="secondary"
          onClick={() => router.back()}
        >
          キャンセル
        </Button>
      </div>
    </form>
  );
}

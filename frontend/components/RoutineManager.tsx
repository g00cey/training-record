'use client';

import { useEffect, useMemo, useState } from 'react';
import { useFieldArray, useForm, type Resolver } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import useSWR from 'swr';
import { apiMutate, ApiError, fetcher } from '@/lib/client';
import { numOrNull, routineFormSchema } from '@/lib/schemas';
import type {
  ExercisesList,
  Routine,
  RoutineHistory,
} from '@/lib/types';
import { longLabel } from '@/lib/date';
import {
  Button,
  Card,
  EmptyState,
  ErrorBox,
  Field,
  SectionTitle,
  Spinner,
  Toast,
} from './ui';

type RoutineRow = {
  name: string;
  bodyweight: boolean;
  weight: string;
  reps: string;
  sets: string;
};

type FormValues = {
  date: string;
  exercises: RoutineRow[];
};

const resolver = zodResolver(routineFormSchema) as unknown as Resolver<FormValues>;

function toRows(routine: Routine): RoutineRow[] {
  return routine.exercises.map((ex) => ({
    name: ex.name,
    bodyweight: ex.weight === null,
    weight: ex.weight === null ? '' : String(ex.weight),
    reps: String(ex.reps),
    sets: String(ex.sets ?? 1),
  }));
}

export function RoutineManager() {
  const routine = useSWR<Routine | null>('/routine', async (k: string) => {
    try {
      return await fetcher<Routine>(k);
    } catch (e) {
      if (e instanceof ApiError && e.status === 404) return null;
      throw e;
    }
  });
  const history = useSWR<RoutineHistory>('/routine/history', fetcher);
  const exercisesList = useSWR<ExercisesList>('/exercises', fetcher);

  const [editing, setEditing] = useState(false);
  const [msg, setMsg] = useState<{ kind: 'success' | 'error'; text: string } | null>(
    null,
  );

  if (routine.isLoading) return <Spinner />;
  if (routine.error) {
    return <ErrorBox error={routine.error} onRetry={() => routine.mutate()} />;
  }

  return (
    <div className="space-y-6">
      <SectionTitle
        action={
          !editing && (
            <Button variant="primary" onClick={() => setEditing(true)}>
              全体を編集
            </Button>
          )
        }
      >
        ルーティン管理
      </SectionTitle>

      {msg && <Toast kind={msg.kind}>{msg.text}</Toast>}

      {editing ? (
        <RoutineEditor
          current={routine.data ?? null}
          exerciseNames={exercisesList.data?.exercises ?? []}
          onCancel={() => setEditing(false)}
          onSaved={() => {
            setEditing(false);
            setMsg({ kind: 'success', text: 'ルーティンを更新しました（新しいスナップショットを作成）。' });
            routine.mutate();
            history.mutate();
          }}
          onError={(text) => setMsg({ kind: 'error', text })}
        />
      ) : (
        <CurrentRoutine
          routine={routine.data ?? null}
          onPatched={(text) => {
            setMsg({ kind: 'success', text });
            routine.mutate();
            history.mutate();
          }}
          onError={(text) => setMsg({ kind: 'error', text })}
        />
      )}

      <div>
        <h3 className="mb-2 font-semibold text-gray-800">スナップショット履歴</h3>
        {history.error && (
          <ErrorBox error={history.error} onRetry={() => history.mutate()} />
        )}
        {history.data && <HistoryList snapshots={history.data.snapshots} />}
      </div>
    </div>
  );
}

function CurrentRoutine({
  routine,
  onPatched,
  onError,
}: {
  routine: Routine | null;
  onPatched: (text: string) => void;
  onError: (text: string) => void;
}) {
  if (!routine) {
    return (
      <EmptyState>
        ルーティンが未登録です。「全体を編集」から作成してください。
      </EmptyState>
    );
  }
  return (
    <Card>
      <p className="mb-3 text-sm text-gray-500">
        現在のルーティン（{routine.date} 時点 / {routine.exercises.length} 種目）。番号順 = 実施順。
      </p>
      <div className="space-y-2">
        {routine.exercises.map((ex, i) => (
          <RoutineRowInline
            key={`${ex.name}-${i}`}
            index={i}
            name={ex.name}
            weight={ex.weight}
            reps={ex.reps}
            sets={ex.sets}
            onPatched={onPatched}
            onError={onError}
          />
        ))}
      </div>
    </Card>
  );
}

function RoutineRowInline({
  index,
  name,
  weight,
  reps,
  sets,
  onPatched,
  onError,
}: {
  index: number;
  name: string;
  weight: number | null;
  reps: number;
  sets: number;
  onPatched: (text: string) => void;
  onError: (text: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [w, setW] = useState(weight === null ? '' : String(weight));
  const [r, setR] = useState(String(reps));
  const [s, setS] = useState(String(sets));
  const [busy, setBusy] = useState(false);

  async function save() {
    setBusy(true);
    try {
      const body: Record<string, number> = {};
      const wn = numOrNull(w);
      const rn = numOrNull(r);
      const sn = numOrNull(s);
      if (wn !== null) body.weight = wn;
      if (rn !== null) body.reps = rn;
      if (sn !== null) body.sets = sn;
      const res = await apiMutate<{ action?: string; routineDate?: string }>(
        `/routine/exercises/${encodeURIComponent(name)}`,
        'PATCH',
        body,
      );
      onPatched(
        `「${name}」を${res?.action === 'added' ? '追加' : '更新'}しました${
          res?.routineDate ? `（${res.routineDate}）` : ''
        }。`,
      );
      setOpen(false);
    } catch (e) {
      onError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="rounded-md border border-gray-200 p-2 text-sm">
      <div className="flex items-center justify-between gap-2">
        <span>
          <span className="mr-2 tabular-nums text-gray-400">{index + 1}.</span>
          <span className="font-medium">{name}</span>
          <span className="ml-2 text-gray-500">
            {weight === null ? '自重' : `${weight}kg`} × {reps} × {sets}
          </span>
        </span>
        <Button
          type="button"
          variant="ghost"
          className="px-2 py-1 text-xs"
          onClick={() => setOpen((o) => !o)}
        >
          {open ? '閉じる' : '編集'}
        </Button>
      </div>
      {open && (
        <div className="mt-2 flex flex-wrap items-end gap-2">
          <label className="text-xs text-gray-500">
            重量
            <input
              className="input mt-1 w-24"
              value={w}
              onChange={(e) => setW(e.target.value)}
              placeholder="自重は空"
            />
          </label>
          <label className="text-xs text-gray-500">
            回数
            <input
              className="input mt-1 w-20"
              value={r}
              onChange={(e) => setR(e.target.value)}
            />
          </label>
          <label className="text-xs text-gray-500">
            セット
            <input
              className="input mt-1 w-20"
              value={s}
              onChange={(e) => setS(e.target.value)}
            />
          </label>
          <Button
            type="button"
            variant="primary"
            disabled={busy}
            onClick={save}
          >
            {busy ? '保存中…' : 'この種目を保存'}
          </Button>
        </div>
      )}
    </div>
  );
}

function RoutineEditor({
  current,
  exerciseNames,
  onCancel,
  onSaved,
  onError,
}: {
  current: Routine | null;
  exerciseNames: string[];
  onCancel: () => void;
  onSaved: () => void;
  onError: (text: string) => void;
}) {
  const defaultValues = useMemo<FormValues>(
    () => ({
      date: '',
      exercises:
        current && current.exercises.length > 0
          ? toRows(current)
          : [{ name: '', bodyweight: false, weight: '', reps: '', sets: '1' }],
    }),
    [current],
  );

  const {
    register,
    control,
    handleSubmit,
    watch,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({ resolver, defaultValues });
  const fieldArray = useFieldArray({ control, name: 'exercises' });

  useEffect(() => {
    // current が後から来た場合に反映
    if (current && current.exercises.length > 0) {
      fieldArray.replace(toRows(current));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current]);

  const onSubmit = handleSubmit(async (values) => {
    const payload: {
      date?: string;
      exercises: { name: string; weight: number | null; reps: number; sets: number }[];
    } = {
      exercises: values.exercises.map((row) => ({
        name: row.name.trim(),
        weight: row.bodyweight ? null : numOrNull(row.weight),
        reps: numOrNull(row.reps) ?? 0,
        sets: numOrNull(row.sets) ?? 1,
      })),
    };
    if (values.date) payload.date = values.date;
    try {
      await apiMutate('/routine', 'PUT', payload);
      onSaved();
    } catch (e) {
      onError(e instanceof Error ? e.message : String(e));
    }
  });

  const nameListId = 'routine-name-list';

  return (
    <form onSubmit={onSubmit}>
      <Card className="space-y-3">
        <datalist id={nameListId}>
          {exerciseNames.map((n) => (
            <option key={n} value={n} />
          ))}
        </datalist>

        <div className="flex flex-wrap items-end justify-between gap-2">
          <div className="w-52">
            <Field
              label="スナップショット日付（任意・既定は今日）"
              error={errors.date?.message as string | undefined}
            >
              <input type="date" className="input" {...register('date')} />
            </Field>
          </div>
          <Button
            type="button"
            variant="primary"
            onClick={() =>
              fieldArray.append({
                name: '',
                bodyweight: false,
                weight: '',
                reps: '',
                sets: '1',
              })
            }
          >
            種目を追加
          </Button>
        </div>

        {typeof errors.exercises?.message === 'string' && (
          <p className="text-xs text-red-600">{errors.exercises.message}</p>
        )}

        <div className="space-y-2">
          {fieldArray.fields.map((f, i) => {
            const rowErr = errors.exercises?.[i];
            const isBw = watch(`exercises.${i}.bodyweight`);
            return (
              <div
                key={f.id}
                className="grid gap-2 rounded-md border border-gray-200 p-2 sm:grid-cols-12"
              >
                <div className="sm:col-span-4">
                  <input
                    className="input"
                    list={nameListId}
                    placeholder="種目名"
                    {...register(`exercises.${i}.name`)}
                  />
                  {rowErr?.name?.message && (
                    <p className="mt-1 text-xs text-red-600">
                      {rowErr.name.message as string}
                    </p>
                  )}
                </div>
                <div className="sm:col-span-2">
                  <input
                    className="input"
                    type="number"
                    step="0.5"
                    placeholder={isBw ? '自重' : '重量'}
                    disabled={!!isBw}
                    {...register(`exercises.${i}.weight`)}
                  />
                </div>
                <div className="sm:col-span-2">
                  <input
                    className="input"
                    type="number"
                    placeholder="回数"
                    {...register(`exercises.${i}.reps`)}
                  />
                </div>
                <div className="sm:col-span-2">
                  <input
                    className="input"
                    type="number"
                    placeholder="セット"
                    {...register(`exercises.${i}.sets`)}
                  />
                </div>
                <div className="flex items-center gap-1 sm:col-span-2">
                  <label className="flex items-center gap-1 text-xs">
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
                  <button
                    type="button"
                    className="btn-ghost px-1 text-xs"
                    disabled={i === 0}
                    onClick={() => fieldArray.move(i, i - 1)}
                  >
                    ↑
                  </button>
                  <button
                    type="button"
                    className="btn-ghost px-1 text-xs"
                    disabled={i === fieldArray.fields.length - 1}
                    onClick={() => fieldArray.move(i, i + 1)}
                  >
                    ↓
                  </button>
                  <button
                    type="button"
                    className="btn-ghost px-1 text-xs text-red-600"
                    disabled={fieldArray.fields.length <= 1}
                    onClick={() => fieldArray.remove(i)}
                  >
                    ×
                  </button>
                </div>
              </div>
            );
          })}
        </div>

        <div className="flex gap-2">
          <Button type="submit" variant="primary" disabled={isSubmitting}>
            {isSubmitting ? '保存中…' : 'ルーティンを保存（PUT）'}
          </Button>
          <Button type="button" variant="secondary" onClick={onCancel}>
            キャンセル
          </Button>
        </div>
      </Card>
    </form>
  );
}

function HistoryList({
  snapshots,
}: {
  snapshots: { date: string; exerciseCount: number }[];
}) {
  const [openDate, setOpenDate] = useState<string | null>(null);
  const detail = useSWR<Routine>(
    openDate ? `/routine/${openDate}` : null,
    fetcher,
  );

  if (snapshots.length === 0) {
    return <EmptyState>スナップショットはまだありません。</EmptyState>;
  }

  return (
    <Card>
      <ul className="divide-y divide-gray-100 text-sm">
        {snapshots.map((snap) => (
          <li key={snap.date} className="py-2">
            <div className="flex items-center justify-between">
              <span>
                <span className="font-medium">{longLabel(snap.date)}</span>
                <span className="ml-2 text-gray-500">
                  {snap.exerciseCount} 種目
                </span>
              </span>
              <Button
                type="button"
                variant="ghost"
                className="px-2 py-1 text-xs"
                onClick={() =>
                  setOpenDate((d) => (d === snap.date ? null : snap.date))
                }
              >
                {openDate === snap.date ? '閉じる' : '内容を見る'}
              </Button>
            </div>
            {openDate === snap.date && (
              <div className="mt-2 rounded-md bg-gray-50 p-2 text-xs">
                {detail.isLoading && <Spinner label="取得中…" />}
                {detail.error && <ErrorBox error={detail.error} />}
                {detail.data && (
                  <ol className="list-decimal space-y-0.5 pl-5">
                    {detail.data.exercises.map((ex, i) => (
                      <li key={`${ex.name}-${i}`}>
                        {ex.name} — {ex.weight === null ? '自重' : `${ex.weight}kg`}{' '}
                        × {ex.reps} × {ex.sets}
                      </li>
                    ))}
                  </ol>
                )}
              </div>
            )}
          </li>
        ))}
      </ul>
    </Card>
  );
}

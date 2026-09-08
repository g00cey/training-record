'use client';

import { useMemo, useState } from 'react';
import { useFieldArray, useForm, type Resolver } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import useSWR from 'swr';
import { apiMutate, ApiError, fetcher } from '@/lib/client';
import { numOrNull, routineFormSchema } from '@/lib/schemas';
import type {
  ExercisesList,
  Preset,
  PresetHistory,
  PresetList,
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
  cn,
} from './ui';

type PresetRow = {
  name: string;
  bodyweight: boolean;
  weight: string;
  reps: string;
  sets: string;
};

type FormValues = {
  date: string;
  exercises: PresetRow[];
};

const resolver = zodResolver(routineFormSchema) as unknown as Resolver<FormValues>;

function toRows(preset: Preset): PresetRow[] {
  return preset.exercises.map((ex) => ({
    name: ex.name,
    bodyweight: ex.weight === null,
    weight: ex.weight === null ? '' : String(ex.weight),
    reps: String(ex.reps),
    sets: String(ex.sets ?? 1),
  }));
}

const enc = encodeURIComponent;

export function PresetManager() {
  const list = useSWR<PresetList>('/presets', fetcher);
  const exercisesList = useSWR<ExercisesList>('/exercises', fetcher);

  const [msg, setMsg] = useState<{ kind: 'success' | 'error'; text: string } | null>(
    null,
  );
  const [newName, setNewName] = useState('');
  const [busy, setBusy] = useState(false);

  const presets = useMemo(
    () =>
      [...(list.data?.presets ?? [])].sort((a, b) => a.sortOrder - b.sortOrder),
    [list.data],
  );

  const flash = (kind: 'success' | 'error', text: string) =>
    setMsg({ kind, text });
  const errText = (e: unknown) => (e instanceof Error ? e.message : String(e));

  async function createPreset() {
    const name = newName.trim();
    if (!name) return;
    setBusy(true);
    setMsg(null);
    try {
      await apiMutate('/presets', 'POST', { name });
      setNewName('');
      flash('success', `プリセット「${name}」を作成しました。`);
      list.mutate();
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        flash('error', `「${name}」は既に存在します。`);
      } else {
        flash('error', errText(e));
      }
    } finally {
      setBusy(false);
    }
  }

  async function deletePreset(name: string) {
    if (
      !window.confirm(
        `プリセット「${name}」を削除します（履歴スナップショットも削除されます）。よろしいですか？`,
      )
    ) {
      return;
    }
    setMsg(null);
    try {
      await apiMutate(`/presets/${enc(name)}`, 'DELETE');
      flash('success', `「${name}」を削除しました。`);
      list.mutate();
    } catch (e) {
      flash('error', errText(e));
    }
  }

  async function reorder(from: number, to: number) {
    if (to < 0 || to >= presets.length) return;
    const order = presets.map((p) => p.name);
    const [moved] = order.splice(from, 1);
    order.splice(to, 0, moved);
    setMsg(null);
    try {
      await apiMutate('/presets:reorder', 'PUT', { order });
      list.mutate();
    } catch (e) {
      flash('error', errText(e));
    }
  }

  if (list.isLoading) return <Spinner />;
  if (list.error) {
    return <ErrorBox error={list.error} onRetry={() => list.mutate()} />;
  }

  return (
    <div className="space-y-6">
      <SectionTitle>プリセット管理</SectionTitle>
      <p className="text-sm text-gray-500">
        名前付きメニュー（自重 / FW など）。記録フォームでは複数プリセットを加算的に選んで種目を補完できます。
      </p>

      {msg && <Toast kind={msg.kind}>{msg.text}</Toast>}

      <Card>
        <div className="flex flex-wrap items-end gap-2">
          <div className="w-64">
            <Field label="新しいプリセット名">
              <input
                className="input"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder="例: コンディショニング"
              />
            </Field>
          </div>
          <Button
            variant="primary"
            onClick={createPreset}
            disabled={busy || !newName.trim()}
          >
            作成
          </Button>
        </div>
      </Card>

      {presets.length === 0 ? (
        <EmptyState>プリセットがありません。上のフォームから作成してください。</EmptyState>
      ) : (
        <div className="space-y-3">
          {presets.map((p, i) => (
            <PresetCard
              key={p.name}
              summary={p}
              index={i}
              total={presets.length}
              exerciseNames={exercisesList.data?.exercises ?? []}
              onMoveUp={() => reorder(i, i - 1)}
              onMoveDown={() => reorder(i, i + 1)}
              onDelete={() => deletePreset(p.name)}
              onChanged={(text) => {
                flash('success', text);
                list.mutate();
              }}
              onError={(text) => flash('error', text)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function PresetCard({
  summary,
  index,
  total,
  exerciseNames,
  onMoveUp,
  onMoveDown,
  onDelete,
  onChanged,
  onError,
}: {
  summary: { name: string; sortOrder: number; exerciseCount: number; latestDate: string | null };
  index: number;
  total: number;
  exerciseNames: string[];
  onMoveUp: () => void;
  onMoveDown: () => void;
  onDelete: () => void;
  onChanged: (text: string) => void;
  onError: (text: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState(false);

  const detail = useSWR<Preset | null>(
    open ? `/presets/${enc(summary.name)}` : null,
    async (k: string) => {
      try {
        return await fetcher<Preset>(k);
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) return null;
        throw e;
      }
    },
  );
  const history = useSWR<PresetHistory>(
    open ? `/presets/${enc(summary.name)}/history` : null,
    fetcher,
  );

  return (
    <Card>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <span className="text-base font-semibold">{summary.name}</span>
          <span className="ml-2 text-sm text-gray-500">
            {summary.exerciseCount} 種目
            {summary.latestDate ? ` / 最新 ${summary.latestDate}` : ' / 未作成'}
          </span>
        </div>
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            className="px-2"
            disabled={index === 0}
            onClick={onMoveUp}
            aria-label="上へ"
          >
            ↑
          </Button>
          <Button
            variant="ghost"
            className="px-2"
            disabled={index === total - 1}
            onClick={onMoveDown}
            aria-label="下へ"
          >
            ↓
          </Button>
          <Button
            variant="ghost"
            className="px-2 py-1 text-sm"
            onClick={() => setOpen((o) => !o)}
          >
            {open ? '閉じる' : '開く'}
          </Button>
          <Button variant="danger" className="px-2 py-1 text-sm" onClick={onDelete}>
            削除
          </Button>
        </div>
      </div>

      {open && (
        <div className="mt-3 space-y-4 border-t border-gray-100 pt-3">
          {detail.isLoading && <Spinner label="取得中…" />}
          {detail.error && (
            <ErrorBox error={detail.error} onRetry={() => detail.mutate()} />
          )}

          {!detail.isLoading && !detail.error && (
            <>
              <div className="flex items-center justify-between">
                <h4 className="font-semibold text-gray-800">
                  現在の種目
                  {detail.data ? `（${detail.data.date} 時点）` : ''}
                </h4>
                {!editing && (
                  <Button
                    variant="primary"
                    className="px-2 py-1 text-sm"
                    onClick={() => setEditing(true)}
                  >
                    全体を編集
                  </Button>
                )}
              </div>

              {editing ? (
                <PresetEditor
                  presetName={summary.name}
                  current={detail.data ?? null}
                  exerciseNames={exerciseNames}
                  onCancel={() => setEditing(false)}
                  onSaved={() => {
                    setEditing(false);
                    onChanged(`「${summary.name}」を更新しました（新スナップショット）。`);
                    detail.mutate();
                    history.mutate();
                  }}
                  onError={onError}
                />
              ) : !detail.data || detail.data.exercises.length === 0 ? (
                <EmptyState>
                  スナップショット未作成です。「全体を編集」から種目を登録してください。
                </EmptyState>
              ) : (
                <div className="space-y-2">
                  {detail.data.exercises.map((ex, i) => (
                    <PresetRowInline
                      key={`${ex.name}-${i}`}
                      presetName={summary.name}
                      index={i}
                      name={ex.name}
                      weight={ex.weight}
                      reps={ex.reps}
                      sets={ex.sets}
                      onPatched={(text) => {
                        onChanged(text);
                        detail.mutate();
                      }}
                      onError={onError}
                    />
                  ))}
                </div>
              )}

              <div>
                <h4 className="mb-2 font-semibold text-gray-800">
                  スナップショット履歴
                </h4>
                {history.error && (
                  <ErrorBox error={history.error} onRetry={() => history.mutate()} />
                )}
                {history.data && (
                  <PresetHistoryList
                    presetName={summary.name}
                    snapshots={history.data.snapshots}
                  />
                )}
              </div>
            </>
          )}
        </div>
      )}
    </Card>
  );
}

function PresetRowInline({
  presetName,
  index,
  name,
  weight,
  reps,
  sets,
  onPatched,
  onError,
}: {
  presetName: string;
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
      const res = await apiMutate<{ action?: string; presetDate?: string }>(
        `/presets/${enc(presetName)}/exercises/${enc(name)}`,
        'PATCH',
        body,
      );
      onPatched(
        `「${name}」を${res?.action === 'added' ? '追加' : '更新'}しました${
          res?.presetDate ? `（${res.presetDate}）` : ''
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
          <Button variant="primary" disabled={busy} onClick={save}>
            {busy ? '保存中…' : 'この種目を保存'}
          </Button>
        </div>
      )}
    </div>
  );
}

function PresetEditor({
  presetName,
  current,
  exerciseNames,
  onCancel,
  onSaved,
  onError,
}: {
  presetName: string;
  current: Preset | null;
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
      await apiMutate(`/presets/${enc(presetName)}`, 'PUT', payload);
      onSaved();
    } catch (e) {
      onError(e instanceof Error ? e.message : String(e));
    }
  });

  const nameListId = `preset-name-list-${presetName}`;

  return (
    <form onSubmit={onSubmit} className="space-y-3">
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
                      setValue(`exercises.${i}.bodyweight`, e.target.checked, {
                        shouldDirty: true,
                      });
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
          {isSubmitting ? '保存中…' : 'このプリセットを保存（PUT）'}
        </Button>
        <Button type="button" variant="secondary" onClick={onCancel}>
          キャンセル
        </Button>
      </div>
    </form>
  );
}

function PresetHistoryList({
  presetName,
  snapshots,
}: {
  presetName: string;
  snapshots: { date: string; exerciseCount: number }[];
}) {
  const [openDate, setOpenDate] = useState<string | null>(null);
  const detail = useSWR<Preset>(
    openDate ? `/presets/${enc(presetName)}/${openDate}` : null,
    fetcher,
  );

  if (snapshots.length === 0) {
    return <EmptyState>スナップショットはまだありません。</EmptyState>;
  }

  return (
    <ul className="divide-y divide-gray-100 rounded-md border border-gray-200 text-sm">
      {snapshots.map((snap) => (
        <li key={snap.date} className="px-2 py-2">
          <div className="flex items-center justify-between">
            <span>
              <span className="font-medium">{longLabel(snap.date)}</span>
              <span className="ml-2 text-gray-500">{snap.exerciseCount} 種目</span>
            </span>
            <Button
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
            <div className={cn('mt-2 rounded-md bg-gray-50 p-2 text-xs')}>
              {detail.isLoading && <Spinner label="取得中…" />}
              {detail.error && <ErrorBox error={detail.error} />}
              {detail.data && (
                <ol className="list-decimal space-y-0.5 pl-5">
                  {detail.data.exercises.map((ex, i) => (
                    <li key={`${ex.name}-${i}`}>
                      {ex.name} — {ex.weight === null ? '自重' : `${ex.weight}kg`} ×{' '}
                      {ex.reps} × {ex.sets}
                    </li>
                  ))}
                </ol>
              )}
            </div>
          )}
        </li>
      ))}
    </ul>
  );
}

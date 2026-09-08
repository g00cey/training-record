'use client';

import { useEffect, useState } from 'react';
import { useForm, type Resolver } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import useSWR from 'swr';
import { apiMutate, fetcher } from '@/lib/client';
import { numOrNull, profileFormSchema } from '@/lib/schemas';
import type { Profile } from '@/lib/types';
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
  bodyweightKg: string;
  heightCm: string;
  maxHrEst: string;
};

const resolver = zodResolver(profileFormSchema) as unknown as Resolver<FormValues>;

export function SettingsForm() {
  const { data, error, isLoading, mutate } = useSWR<Profile>('/profile', fetcher);
  const [msg, setMsg] = useState<{ kind: 'success' | 'error'; text: string } | null>(
    null,
  );

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver,
    defaultValues: { bodyweightKg: '', heightCm: '', maxHrEst: '' },
  });

  useEffect(() => {
    if (data) {
      reset({
        bodyweightKg: data.bodyweightKg != null ? String(data.bodyweightKg) : '',
        heightCm: data.heightCm != null ? String(data.heightCm) : '',
        maxHrEst: data.maxHrEst != null ? String(data.maxHrEst) : '',
      });
    }
  }, [data, reset]);

  const onSubmit = handleSubmit(async (values) => {
    setMsg(null);
    try {
      await apiMutate<Profile>('/profile', 'PUT', {
        bodyweightKg: numOrNull(values.bodyweightKg),
        heightCm: numOrNull(values.heightCm),
        maxHrEst: numOrNull(values.maxHrEst),
      });
      setMsg({ kind: 'success', text: 'プロフィールを更新しました。' });
      mutate();
    } catch (e) {
      setMsg({
        kind: 'error',
        text: e instanceof Error ? e.message : String(e),
      });
    }
  });

  if (isLoading) return <Spinner />;
  if (error) return <ErrorBox error={error} onRetry={() => mutate()} />;

  return (
    <form onSubmit={onSubmit} className="space-y-4">
      <SectionTitle>プロフィール設定</SectionTitle>
      {msg && <Toast kind={msg.kind}>{msg.text}</Toast>}
      <Card className="space-y-4">
        <Field
          label="体重 (kg)"
          error={errors.bodyweightKg?.message as string | undefined}
          hint="自重種目の Volume Load 計算に使われます。"
        >
          <input
            type="number"
            step="0.1"
            className="input"
            {...register('bodyweightKg')}
          />
        </Field>
        <Field
          label="身長 (cm)"
          error={errors.heightCm?.message as string | undefined}
          hint="BMI 表示に使われます。"
        >
          <input
            type="number"
            step="0.1"
            className="input"
            {...register('heightCm')}
          />
        </Field>
        <Field
          label="推定最大心拍 (bpm)"
          error={errors.maxHrEst?.message as string | undefined}
          hint="TRIMP（有酸素負荷）計算に使われます。既定 180。"
        >
          <input
            type="number"
            className="input"
            {...register('maxHrEst')}
          />
        </Field>
        <Button type="submit" variant="primary" disabled={isSubmitting}>
          {isSubmitting ? '保存中…' : '保存する'}
        </Button>
      </Card>
    </form>
  );
}

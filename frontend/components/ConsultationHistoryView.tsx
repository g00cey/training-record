'use client';

import { useState } from 'react';
import useSWR from 'swr';
import { apiMutate, fetcher } from '@/lib/client';
import { longLabel } from '@/lib/date';
import type { ConsultationListResponse } from '@/lib/types';
import { ConsultationModal } from './ConsultationModal';
import {
  Button,
  Card,
  EmptyState,
  ErrorBox,
  SectionTitle,
  Spinner,
  Toast,
} from './ui';

export function ConsultationHistoryView() {
  const { data, error, isLoading, mutate } = useSWR<ConsultationListResponse>(
    '/consultations?limit=100',
    fetcher,
  );

  const [modalOpen, setModalOpen] = useState(false);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [toast, setToast] = useState<{ kind: 'success' | 'error'; text: string } | null>(null);

  const flash = (kind: 'success' | 'error', text: string) => {
    setToast({ kind, text });
  };

  async function handleDelete(id: number, goal: string) {
    const preview = goal.length > 20 ? `${goal.slice(0, 20)}…` : goal;
    if (!window.confirm(`相談「${preview}」を削除しますか？対話履歴も削除されます。`)) {
      return;
    }

    try {
      await apiMutate(`/consultations/${id}`, 'DELETE');
      flash('success', '相談セッションを削除しました。');
      mutate();
    } catch (err) {
      flash('error', err instanceof Error ? err.message : String(err));
    }
  }

  function openNew() {
    setSelectedId(null);
    setModalOpen(true);
  }

  function openExisting(id: number) {
    setSelectedId(id);
    setModalOpen(true);
  }

  return (
    <div className="space-y-4">
      <SectionTitle
        action={
          <Button variant="primary" onClick={openNew}>
            ＋新しく相談する
          </Button>
        }
      >
        相談履歴
      </SectionTitle>

      <p className="text-sm text-gray-500">
        AIパーソナルトレーナーとの相談履歴です。過去のアドバイスや提案メニューの確認、続きからの再開が可能です。
      </p>

      {toast && <Toast kind={toast.kind}>{toast.text}</Toast>}

      {error && <ErrorBox error={error} onRetry={() => mutate()} />}
      {isLoading && !error && <Spinner />}

      {!isLoading && !error && (data?.items.length === 0 || !data) && (
        <EmptyState>
          相談履歴はまだありません。「＋新しく相談する」ボタンからAIに相談してみましょう。
        </EmptyState>
      )}

      {data && data.items.length > 0 && (
        <div className="space-y-3">
          {data.items.map((item) => (
            <Card key={item.id} className="transition hover:border-brand-200">
              <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-start">
                <div className="flex-1 space-y-1.5">
                  <div className="flex items-center gap-2">
                    <span className="text-base font-semibold text-gray-900">
                      {item.goal}
                    </span>
                    <span className="rounded bg-brand-50 px-2 py-0.5 text-xs font-medium text-brand-700">
                      {item.messageCount} 件の対話
                    </span>
                  </div>

                  {item.latestMessagePreview && (
                    <p className="text-xs text-gray-600">
                      <span className="font-medium text-gray-500">最新:</span>{' '}
                      {item.latestMessagePreview}
                    </p>
                  )}

                  <div className="flex items-center gap-3 text-xs text-gray-400">
                    <span>
                      開始: {item.createdAt ? longLabel(item.createdAt.slice(0, 10)) : '—'}
                    </span>
                    <span>
                      更新: {item.updatedAt ? longLabel(item.updatedAt.slice(0, 10)) : '—'}
                    </span>
                  </div>
                </div>

                <div className="flex shrink-0 items-center gap-2">
                  <Button
                    variant="secondary"
                    className="text-xs"
                    onClick={() => openExisting(item.id)}
                  >
                    続きを相談・詳細
                  </Button>
                  <Button
                    variant="danger"
                    className="text-xs"
                    onClick={() => handleDelete(item.id, item.goal)}
                  >
                    削除
                  </Button>
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}

      <ConsultationModal
        isOpen={modalOpen}
        initialId={selectedId}
        onClose={() => setModalOpen(false)}
        onUpdated={() => mutate()}
      />
    </div>
  );
}

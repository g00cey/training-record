'use client';

import { useEffect, useRef, useState } from 'react';
import { apiMutate, fetcher } from '@/lib/client';
import type { ConsultationDetail } from '@/lib/types';
import { Markdown } from './Markdown';
import { Button, ErrorBox, Field, Spinner, cn } from './ui';

interface ConsultationModalProps {
  isOpen: boolean;
  onClose: () => void;
  initialId?: number | null;
  onUpdated?: () => void;
}

export function ConsultationModal({
  isOpen,
  onClose,
  initialId,
  onUpdated,
}: ConsultationModalProps) {
  const [consultation, setConsultation] = useState<ConsultationDetail | null>(null);
  const [goal, setGoal] = useState('');
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(false);
  const [initialLoading, setInitialLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen) {
      setConsultation(null);
      setGoal('');
      setMessage('');
      setError(null);
      return;
    }

    if (initialId) {
      setInitialLoading(true);
      setError(null);
      fetcher<ConsultationDetail>(`/consultations/${initialId}`)
        .then((data) => {
          setConsultation(data);
        })
        .catch((err) => {
          setError(err instanceof Error ? err.message : String(err));
        })
        .finally(() => {
          setInitialLoading(false);
        });
    } else {
      setConsultation(null);
    }
  }, [isOpen, initialId]);

  useEffect(() => {
    if (consultation) {
      messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [consultation?.messages.length]);

  if (!isOpen) return null;

  async function handleStart() {
    const trimmed = goal.trim();
    if (!trimmed) return;
    setLoading(true);
    setError(null);
    try {
      const res = await apiMutate<ConsultationDetail>('/consultations', 'POST', {
        goal: trimmed,
      });
      setConsultation(res);
      setGoal('');
      onUpdated?.();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }

  async function handleSendMessage(requestProposal = false) {
    if (!consultation) return;
    const trimmed = message.trim();
    if (!requestProposal && !trimmed) return;

    setLoading(true);
    setError(null);
    try {
      const res = await apiMutate<ConsultationDetail>(
        `/consultations/${consultation.id}/messages`,
        'POST',
        {
          content: trimmed,
          requestMenuProposal: requestProposal,
        },
      );
      setConsultation(res);
      setMessage('');
      onUpdated?.();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
      <div className="flex h-[88vh] w-full max-w-2xl flex-col rounded-lg bg-white shadow-xl">
        {/* ヘッダー */}
        <div className="flex items-center justify-between border-b border-gray-200 px-5 py-3">
          <div className="flex items-center gap-2">
            <span className="text-xl">💬</span>
            <h3 className="text-base font-semibold text-gray-900">
              AIトレーニング相談
            </h3>
          </div>
          <button
            type="button"
            className="text-gray-400 hover:text-gray-600"
            onClick={onClose}
            aria-label="閉じる"
          >
            ✕
          </button>
        </div>

        {/* コンテンツ領域 */}
        <div className="flex flex-1 flex-col overflow-hidden p-5">
          {error && (
            <div className="mb-3">
              <ErrorBox error={error} onRetry={() => setError(null)} />
            </div>
          )}

          {initialLoading ? (
            <div className="flex flex-1 items-center justify-center">
              <Spinner label="相談データを読み込み中…" />
            </div>
          ) : !consultation ? (
            /* 新規相談開始フォーム */
            <div className="flex flex-1 flex-col justify-center space-y-4">
              <div className="rounded-md bg-brand-50 p-4 text-sm text-brand-800">
                <p className="font-semibold">直近の記録をもとにアドバイス</p>
                <p className="mt-1 text-xs text-brand-700">
                  直近2週間のトレーニング実績（種目・重量・レップ・セット）や現在のプリセット、ACWRなどの負荷指標を自動的にAIへ共有し、あなたの目的に合わせた助言を行います。
                </p>
              </div>

              <Field
                label="トレーニングの目的・相談したいこと"
                hint="例: 筋肥大と体力維持を目的に自重とフリーウェイトを併用しています。直近のメニュー構成や負荷設定が適切かアドバイスをお願いします。"
              >
                <textarea
                  className="input min-h-[140px] resize-y"
                  value={goal}
                  onChange={(e) => setGoal(e.target.value)}
                  placeholder="ここに相談内容を入力してください…"
                  disabled={loading}
                />
              </Field>

              <div className="flex justify-end gap-2 pt-2">
                <Button variant="ghost" onClick={onClose} disabled={loading}>
                  キャンセル
                </Button>
                <Button
                  variant="primary"
                  onClick={handleStart}
                  disabled={loading || !goal.trim()}
                >
                  {loading ? 'AIが分析・回答を作成中…' : '相談を開始する'}
                </Button>
              </div>
            </div>
          ) : (
            /* チャット履歴・対話画面 */
            <div className="flex flex-1 flex-col overflow-hidden">
              <div className="mb-2 flex items-center justify-between border-b border-gray-100 pb-2 text-xs text-gray-500">
                <div className="truncate pr-2">
                  <span className="font-medium text-gray-700">目的:</span>{' '}
                  <span className="text-gray-600">{consultation.goal}</span>
                </div>
                <button
                  type="button"
                  className="shrink-0 text-brand-600 hover:underline"
                  onClick={() => setConsultation(null)}
                >
                  ＋新しく相談する
                </button>
              </div>

              {/* メッセージ一覧 */}
              <div className="flex-1 space-y-3 overflow-y-auto pr-1">
                {consultation.messages.map((m) => (
                  <div
                    key={m.id}
                    className={`flex flex-col ${
                      m.role === 'user' ? 'items-end' : 'items-start'
                    }`}
                  >
                    <span className="mb-1 text-[11px] text-gray-400">
                      {m.role === 'user' ? 'あなた' : 'AIトレーナー'}
                    </span>
                    <div
                      className={cn(
                        'rounded-lg px-4 py-2.5 text-sm leading-relaxed',
                        m.role === 'user'
                          ? 'max-w-[85%] bg-brand-600 text-white'
                          : 'max-w-[92%] border border-gray-200 bg-gray-50 text-gray-800',
                      )}
                    >
                      <Markdown
                        content={m.content}
                        variant={m.role === 'user' ? 'inverse' : 'light'}
                      />
                    </div>
                  </div>
                ))}
                {loading && (
                  <div className="flex items-start">
                    <div className="rounded-lg border border-gray-200 bg-gray-50 p-3">
                      <Spinner label="AIトレーナーが考え中…" />
                    </div>
                  </div>
                )}
                <div ref={messagesEndRef} />
              </div>

              {/* 入力欄 & アクションボタン */}
              <div className="mt-3 border-t border-gray-200 pt-3">
                <textarea
                  className="input min-h-[70px] resize-none text-sm"
                  placeholder="追加の質問や回答を入力…（空欄のまま「メニューを提案してもらう」を押すこともできます）"
                  value={message}
                  onChange={(e) => setMessage(e.target.value)}
                  disabled={loading}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
                      e.preventDefault();
                      handleSendMessage(false);
                    }
                  }}
                />

                <div className="mt-2 flex flex-wrap items-center justify-between gap-2">
                  <span className="text-[11px] text-gray-400">
                    Ctrl + Enter でも送信可能
                  </span>
                  <div className="flex gap-2">
                    <Button
                      variant="primary"
                      onClick={() => handleSendMessage(false)}
                      disabled={loading || !message.trim()}
                    >
                      送信
                    </Button>
                    <button
                      type="button"
                      className="rounded-md border border-amber-300 bg-amber-50 px-3 py-1.5 text-sm font-medium text-amber-800 hover:bg-amber-100 disabled:opacity-50"
                      onClick={() => handleSendMessage(true)}
                      disabled={loading}
                    >
                      ✨ メニューを提案してもらう
                    </button>
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

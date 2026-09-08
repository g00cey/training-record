'use client';

import Link from 'next/link';
import type { AnchorHTMLAttributes, ButtonHTMLAttributes, ReactNode } from 'react';

export function cn(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(' ');
}

export function Card({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return <div className={cn('card p-4', className)}>{children}</div>;
}

export function SectionTitle({
  children,
  action,
}: {
  children: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div className="mb-3 flex items-center justify-between gap-2">
      <h2 className="text-lg font-semibold text-gray-900">{children}</h2>
      {action}
    </div>
  );
}

type BtnVariant = 'primary' | 'secondary' | 'danger' | 'ghost';

const variantClass: Record<BtnVariant, string> = {
  primary: 'btn-primary',
  secondary: 'btn-secondary',
  danger: 'btn-danger',
  ghost: 'btn-ghost',
};

export function Button({
  variant = 'secondary',
  className,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: BtnVariant }) {
  return <button className={cn(variantClass[variant], className)} {...props} />;
}

export function LinkButton({
  variant = 'secondary',
  className,
  href,
  children,
  ...props
}: AnchorHTMLAttributes<HTMLAnchorElement> & {
  variant?: BtnVariant;
  href: string;
}) {
  return (
    <Link href={href} className={cn(variantClass[variant], className)} {...props}>
      {children}
    </Link>
  );
}

export function Spinner({ label = '読み込み中…' }: { label?: string }) {
  return (
    <div className="flex items-center gap-2 py-8 text-sm text-gray-500">
      <span className="h-4 w-4 animate-spin rounded-full border-2 border-gray-300 border-t-brand-600" />
      {label}
    </div>
  );
}

export function ErrorBox({
  error,
  onRetry,
}: {
  error: unknown;
  onRetry?: () => void;
}) {
  const message =
    error instanceof Error ? error.message : '不明なエラーが発生しました';
  return (
    <div className="rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-800">
      <p className="font-medium">データを取得できませんでした</p>
      <p className="mt-1 whitespace-pre-wrap text-red-700">{message}</p>
      <p className="mt-1 text-xs text-red-600">
        backend が起動しているか、API キーが設定されているか確認してください。
      </p>
      {onRetry && (
        <button
          type="button"
          onClick={onRetry}
          className="btn-secondary mt-3"
        >
          再試行
        </button>
      )}
    </div>
  );
}

export function EmptyState({ children }: { children: ReactNode }) {
  return (
    <div className="rounded-md border border-dashed border-gray-300 bg-white p-8 text-center text-sm text-gray-500">
      {children}
    </div>
  );
}

export function Field({
  label,
  error,
  hint,
  children,
}: {
  label: string;
  error?: string;
  hint?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div>
      <label className="label">{label}</label>
      {children}
      {hint && <p className="mt-1 text-xs text-gray-500">{hint}</p>}
      {error && <p className="mt-1 text-xs text-red-600">{error}</p>}
    </div>
  );
}

export function Toast({
  kind,
  children,
}: {
  kind: 'success' | 'error';
  children: ReactNode;
}) {
  return (
    <div
      className={cn(
        'rounded-md border p-3 text-sm',
        kind === 'success'
          ? 'border-emerald-200 bg-emerald-50 text-emerald-800'
          : 'border-red-200 bg-red-50 text-red-800',
      )}
    >
      {children}
    </div>
  );
}

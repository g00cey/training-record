'use client';

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <div className="rounded-md border border-red-200 bg-red-50 p-6 text-sm text-red-800">
      <p className="text-base font-semibold">エラーが発生しました</p>
      <p className="mt-2 whitespace-pre-wrap text-red-700">{error.message}</p>
      <button type="button" onClick={reset} className="btn-secondary mt-4">
        再読み込み
      </button>
    </div>
  );
}

import Link from 'next/link';

export default function NotFound() {
  return (
    <div className="rounded-md border border-gray-200 bg-white p-8 text-center text-sm text-gray-600">
      <p className="text-base font-semibold text-gray-900">ページが見つかりません</p>
      <p className="mt-2">
        <Link href="/" className="text-brand-700 underline">
          一覧へ戻る
        </Link>
      </p>
    </div>
  );
}

'use client';

import MarkdownRenderer, { type Components } from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkBreaks from 'remark-breaks';

// AI 相談の回答など、マークダウンで返るテキストを描画するための共通コンポーネント。
// GFM 表（メニュー案の「種目 | 重量 | 回数 | セット」等）と改行を維持する。
// variant: 'light' は白〜グレー背景、'inverse' はブランド色背景（白文字）で使う。

function buildComponents(inverse: boolean): Components {
  return {
    h1: ({ children }) => (
      <h1
        className={`mb-1 mt-3 text-base font-bold first:mt-0 ${
          inverse ? 'text-white' : 'text-gray-900'
        }`}
      >
        {children}
      </h1>
    ),
    h2: ({ children }) => (
      <h2
        className={`mb-1 mt-3 text-sm font-bold first:mt-0 ${
          inverse ? 'text-white' : 'text-gray-900'
        }`}
      >
        {children}
      </h2>
    ),
    h3: ({ children }) => (
      <h3
        className={`mb-1 mt-2 text-sm font-semibold first:mt-0 ${
          inverse ? 'text-gray-50' : 'text-gray-800'
        }`}
      >
        {children}
      </h3>
    ),
    h4: ({ children }) => (
      <h4
        className={`mb-1 mt-2 text-[13px] font-semibold first:mt-0 ${
          inverse ? 'text-gray-50' : 'text-gray-800'
        }`}
      >
        {children}
      </h4>
    ),
    p: ({ children }) => (
      <p className="my-1 leading-relaxed first:mt-0 last:mb-0">{children}</p>
    ),
    ul: ({ children }) => (
      <ul className="my-1 list-disc space-y-0.5 pl-5">{children}</ul>
    ),
    ol: ({ children }) => (
      <ol className="my-1 list-decimal space-y-0.5 pl-5">{children}</ol>
    ),
    li: ({ children }) => <li className="leading-relaxed">{children}</li>,
    strong: ({ children }) => (
      <strong
        className={`font-semibold ${inverse ? 'text-white' : 'text-gray-900'}`}
      >
        {children}
      </strong>
    ),
    em: ({ children }) => <em className="italic">{children}</em>,
    del: ({ children }) => (
      <del className={inverse ? 'text-white/60 line-through' : 'text-gray-400 line-through'}>
        {children}
      </del>
    ),
    a: ({ href, children }) => (
      <a
        href={href}
        target="_blank"
        rel="noreferrer"
        className={
          inverse
            ? 'text-white underline decoration-white/70 underline-offset-2 hover:decoration-white'
            : 'text-brand-600 underline underline-offset-2 hover:text-brand-700'
        }
      >
        {children}
      </a>
    ),
    blockquote: ({ children }) => (
      <blockquote
        className={`my-2 border-l-4 pl-3 italic ${
          inverse
            ? 'border-white/40 text-gray-100'
            : 'border-gray-300 text-gray-600'
        }`}
      >
        {children}
      </blockquote>
    ),
    hr: () => (
      <hr className={`my-3 ${inverse ? 'border-white/30' : 'border-gray-200'}`} />
    ),
    // 表: メニュー案が横に長いことを考慮し、コンテナ側で横スクロールさせる
    table: ({ children }) => (
      <div className="my-2 overflow-x-auto">
        <table className="w-full border-collapse text-xs">{children}</table>
      </div>
    ),
    thead: ({ children }) => (
      <thead
        className={
          inverse ? 'bg-white/15 text-white' : 'bg-gray-100 text-gray-700'
        }
      >
        {children}
      </thead>
    ),
    th: ({ children }) => (
      <th
        className={`border px-2 py-1 text-left font-semibold whitespace-normal ${
          inverse ? 'border-white/40' : 'border-gray-300'
        }`}
      >
        {children}
      </th>
    ),
    td: ({ children }) => (
      <td
        className={`border px-2 py-1 align-top whitespace-normal ${
          inverse ? 'border-white/30' : 'border-gray-200'
        }`}
      >
        {children}
      </td>
    ),
    tbody: ({ children }) => (
      <tbody
        className={inverse ? '[&_tr:nth-child(even)]:bg-white/10' : '[&_tr:nth-child(even)]:bg-gray-50'}
      >
        {children}
      </tbody>
    ),
    code: ({ className, children }) => (
      <code
        className={
          className
            ? className
            : `rounded px-1 py-0.5 font-mono text-[12px] ${
                inverse ? 'bg-white/20' : 'bg-gray-200/70'
              }`
        }
      >
        {children}
      </code>
    ),
    pre: ({ children }) => (
      <pre
        className={`my-2 overflow-x-auto rounded-md p-3 text-xs [&_code]:bg-transparent [&_code]:p-0 ${
          inverse ? 'bg-white/10 text-white' : 'bg-gray-100 text-gray-800'
        }`}
      >
        {children}
      </pre>
    ),
    input: (props) => (
      <input {...props} className="mr-1 align-middle accent-brand-600" />
    ),
    img: ({ src, alt }) => (
      // eslint-disable-next-line @next/next/no-img-element
      <img src={src} alt={alt} className="my-2 max-w-full rounded" />
    ),
  };
}

export function Markdown({
  content,
  variant = 'light',
}: {
  content: string;
  variant?: 'light' | 'inverse';
}) {
  const components = buildComponents(variant === 'inverse');
  return (
    <MarkdownRenderer
      remarkPlugins={[remarkGfm, remarkBreaks]}
      components={components}
    >
      {content}
    </MarkdownRenderer>
  );
}

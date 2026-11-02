import { memo, useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import ReactMarkdown, { type Components } from 'react-markdown';
import remarkBreaks from 'remark-breaks';
import remarkGfm from 'remark-gfm';

const remarkPlugins = [remarkGfm, remarkBreaks];

function TableBlock({ children }: { children: ReactNode }) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [showFade, setShowFade] = useState(false);

  useEffect(() => {
    const element = scrollRef.current;
    if (!element) return;

    const update = () => {
      const overflowing = element.scrollWidth > element.clientWidth + 1;
      const atEnd =
        element.scrollLeft + element.clientWidth >= element.scrollWidth - 1;
      setShowFade(overflowing && !atEnd);
    };

    update();
    element.addEventListener('scroll', update, { passive: true });
    window.addEventListener('resize', update);

    return () => {
      element.removeEventListener('scroll', update);
      window.removeEventListener('resize', update);
    };
  }, []);

  return (
    <div className="relative max-w-full">
      <div
        ref={scrollRef}
        className="max-w-full overflow-x-auto overscroll-x-contain rounded-xl border border-warm-200"
      >
        {children}
      </div>
      {showFade && (
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-y-0 right-0 w-10 rounded-r-xl bg-gradient-to-l from-surface-card via-surface-card/70 to-transparent"
        />
      )}
    </div>
  );
}

const components: Components = {
  h1: ({ children }) => (
    <h1 className="text-base font-bold tracking-tight text-stone-900">{children}</h1>
  ),
  h2: ({ children }) => (
    <h2 className="text-sm font-bold tracking-tight text-stone-900">{children}</h2>
  ),
  h3: ({ children }) => (
    <h3 className="text-sm font-semibold text-stone-900">{children}</h3>
  ),
  h4: ({ children }) => (
    <h4 className="text-sm font-semibold text-stone-800">{children}</h4>
  ),
  h5: ({ children }) => (
    <h5 className="text-sm font-semibold text-stone-800">{children}</h5>
  ),
  h6: ({ children }) => (
    <h6 className="text-sm font-semibold text-stone-800">{children}</h6>
  ),
  p: ({ children }) => <p className="leading-relaxed">{children}</p>,
  strong: ({ children }) => (
    <strong className="font-semibold text-stone-900">{children}</strong>
  ),
  em: ({ children }) => <em className="italic">{children}</em>,
  del: ({ children }) => (
    <del className="text-stone-500 line-through">{children}</del>
  ),
  a: ({ href, children }) => (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="font-medium text-brand-700 underline decoration-brand-300 underline-offset-2 transition-colors hover:text-brand-800 hover:decoration-brand-500"
    >
      {children}
    </a>
  ),
  ul: ({ children, className }) => (
    <ul
      className={
        className?.includes('contains-task-list')
          ? 'space-y-1.5'
          : 'list-disc space-y-1 pl-5 marker:text-brand-400'
      }
    >
      {children}
    </ul>
  ),
  ol: ({ children }) => (
    <ol className="list-decimal space-y-1 pl-5 marker:font-semibold marker:text-brand-500">
      {children}
    </ol>
  ),
  li: ({ children, className }) => (
    <li
      className={
        className === 'task-list-item'
          ? 'flex items-start gap-2 leading-relaxed'
          : 'pl-0.5 leading-relaxed'
      }
    >
      {children}
    </li>
  ),
  input: ({ type, checked, disabled }) => (
    <input
      type={type}
      checked={checked}
      disabled={disabled}
      readOnly
      className="mt-0.5 h-3.5 w-3.5 shrink-0 accent-brand-500"
    />
  ),
  blockquote: ({ children }) => (
    <blockquote className="rounded-r-lg border-l-2 border-brand-300 bg-brand-50/50 py-1.5 pl-3 pr-2 text-stone-600 italic">
      {children}
    </blockquote>
  ),
  code: ({ children }) => (
    <code className="rounded-md bg-warm-100 px-1.5 py-0.5 font-mono text-[0.8125rem] text-brand-800">
      {children}
    </code>
  ),
  pre: ({ children }) => (
    <pre className="overflow-x-auto rounded-xl bg-stone-900 px-3 py-2.5 text-xs leading-relaxed text-stone-100 [&_code]:bg-transparent [&_code]:p-0 [&_code]:text-inherit">
      {children}
    </pre>
  ),
  table: ({ children }) => (
    <TableBlock>
      <table className="w-full border-collapse text-left text-xs [&_tr:last-child_td]:border-b-0">
        {children}
      </table>
    </TableBlock>
  ),
  thead: ({ children }) => <thead className="bg-warm-100/80">{children}</thead>,
  th: ({ children, style }) => (
    <th
      style={style}
      className="whitespace-nowrap border-b border-warm-200 px-2 py-2 font-semibold text-stone-800 sm:px-3"
    >
      {children}
    </th>
  ),
  td: ({ children, style }) => (
    <td
      style={style}
      className="border-b border-warm-100 px-2 py-2 align-top text-stone-600 sm:px-3"
    >
      {children}
    </td>
  ),
  hr: () => <hr className="border-warm-200" />,
  img: ({ src, alt }) => (
    <img
      src={src}
      alt={alt ?? ''}
      loading="lazy"
      className="max-w-full rounded-xl border border-warm-200"
    />
  ),
};

interface MarkdownMessageProps {
  content: string;
}

export const MarkdownMessage = memo(function MarkdownMessage({
  content,
}: MarkdownMessageProps) {
  return (
    <div className="min-w-0 space-y-2 break-words text-sm text-stone-700">
      <ReactMarkdown remarkPlugins={remarkPlugins} components={components}>
        {content}
      </ReactMarkdown>
    </div>
  );
});

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MarkdownMessage } from '../MarkdownMessage';

describe('MarkdownMessage', () => {
  it('renders headings, bold text and bullet lists', () => {
    render(
      <MarkdownMessage
        content={'## Spending\n\n- **Food**: 120.00\n- Transport: 40.00'}
      />
    );

    expect(
      screen.getByRole('heading', { level: 2, name: 'Spending' })
    ).toBeInTheDocument();
    expect(screen.getByText('Food').tagName).toBe('STRONG');
    expect(screen.getAllByRole('listitem')).toHaveLength(2);
  });

  it('keeps single line breaks visible in chat replies', () => {
    const { container } = render(
      <MarkdownMessage content={'Cash: 100\nBank: 500'} />
    );

    expect(container.querySelector('p br')).not.toBeNull();
  });

  it('renders GFM tables with header and body cells', () => {
    render(
      <MarkdownMessage
        content={
          '| Account | Balance |\n| --- | ---: |\n| Cash | 1,200.00 |'
        }
      />
    );

    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(
      screen.getByRole('columnheader', { name: 'Account' })
    ).toBeInTheDocument();
    expect(screen.getByRole('cell', { name: '1,200.00' })).toBeInTheDocument();
  });

  it('renders inline code and fenced code blocks distinctly', () => {
    render(
      <MarkdownMessage
        content={'Run `npm run dev`:\n\n```ts\nconst total = 1200;\n```'}
      />
    );

    const inline = screen.getByText('npm run dev');
    expect(inline.tagName).toBe('CODE');
    expect(inline.closest('pre')).toBeNull();

    const block = screen.getByText(/const total/);
    expect(block.closest('pre')).not.toBeNull();
  });

  it('opens links safely and drops unsafe protocols', () => {
    render(
      <MarkdownMessage
        content={'[Docs](https://example.com)\n\n[Bad](javascript:alert(1))'}
      />
    );

    const safe = screen.getByRole('link', { name: 'Docs' });
    expect(safe).toHaveAttribute('href', 'https://example.com');
    expect(safe).toHaveAttribute('target', '_blank');
    expect(safe).toHaveAttribute('rel', 'noopener noreferrer');

    const unsafe = screen.getByText('Bad').closest('a');
    expect(unsafe?.getAttribute('href') ?? '').not.toContain('javascript');
  });
});

import type { MerchantTotal } from '../../types';
import { formatCurrency } from '../../lib/format';
import { Card } from '../ui/Card';

interface TopMerchantsProps {
  merchants: MerchantTotal[];
  currency: string;
}

export function TopMerchants({ merchants, currency }: TopMerchantsProps) {
  return (
    <Card className="p-5">
      <div>
        <h2 className="text-base font-bold text-stone-900">Top merchants</h2>
        <p className="text-xs text-stone-500">
          Largest totals by description in this range
        </p>
      </div>

      {merchants.length === 0 ? (
        <p className="mt-4 rounded-xl border border-dashed border-warm-300 bg-warm-50/60 px-4 py-10 text-center text-sm text-stone-500">
          No merchant activity recorded in this range.
        </p>
      ) : (
        <div className="mt-4 overflow-x-auto">
          <table className="min-w-full text-sm">
            <caption className="sr-only">
              Top merchants by total amount
            </caption>
            <thead>
              <tr className="border-b border-warm-200 text-left text-xs font-semibold uppercase tracking-wide text-stone-400">
                <th scope="col" className="pb-2 pr-4">
                  Description
                </th>
                <th scope="col" className="pb-2 text-right">
                  Total
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-warm-100">
              {merchants.map((merchant, index) => (
                <tr key={`${merchant.description}-${index}`}>
                  <td className="max-w-0 truncate py-3 pr-4 font-semibold text-stone-800">
                    {merchant.description}
                  </td>
                  <td className="whitespace-nowrap py-3 text-right font-bold text-stone-700">
                    {formatCurrency(merchant.total, currency)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  );
}

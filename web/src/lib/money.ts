// Money is handled as integer cents everywhere; these helpers convert to and
// from what people type and read.

const formatter = new Intl.NumberFormat(undefined, { style: 'currency', currency: 'EUR' });

export function formatCents(cents: number): string {
  return formatter.format(cents / 100);
}

/** Parses "12", "12.5", "12,50" or "1.234,56" into cents. Returns null when invalid. */
export function parseAmount(input: string): number | null {
  let s = input.trim().replace(/[€\s]/g, '');
  if (!s) return null;

  const lastComma = s.lastIndexOf(',');
  const lastDot = s.lastIndexOf('.');
  const decimalSep = lastComma > lastDot ? ',' : '.';
  const thousandsSep = decimalSep === ',' ? '.' : ',';

  s = s.split(thousandsSep).join('');
  if (decimalSep === ',') s = s.replace(',', '.');
  if (!/^\d+(\.\d{0,2})?$/.test(s)) return null;

  const cents = Math.round(parseFloat(s) * 100);
  return cents > 0 ? cents : null;
}

const OPS = /[+\-*/×÷x]/;

/** True when what was typed is a sum like "12.50+8" rather than one amount. */
export function isSum(input: string): boolean {
  return OPS.test(input.trim().replace(/^[+-]/, ''));
}

/**
 * Works out a sum typed in the amount field, such as "12.50+8,30" or
 * "90/3", into cents: + − × ÷ (also * / x), each number read like
 * parseAmount, times and divide before plus and minus. Rounded to the cent;
 * null when it can't be read or isn't more than zero. A single amount
 * gives the same as parseAmount.
 */
export function evalAmount(input: string): number | null {
  const s = input.replace(/[€\s]/g, '').replace(/−/g, '-').replace(/[×x]/gi, '*').replace(/÷/g, '/');
  if (!s) return null;
  if (!isSum(s)) return parseAmount(s);
  const tokens = s.match(/[+\-*/]|[^+\-*/]+/g) ?? [];
  // numbers as euros (fractions allowed while working it out), ops between them
  let terms: number[] = [];
  let signs: number[] = [];
  let cur: number | null = null;
  let pendingOp: '*' | '/' | null = null;
  let sign = 1;
  let expectNumber = true;
  for (const t of tokens) {
    if (expectNumber) {
      const cents = t === '0' ? 0 : parseAmount(t);
      if (cents === null) return null;
      const v = cents / 100;
      if (pendingOp === '*') cur = cur! * v;
      else if (pendingOp === '/') {
        if (v === 0) return null;
        cur = cur! / v;
      } else cur = v;
      pendingOp = null;
      expectNumber = false;
    } else {
      if (t === '*' || t === '/') pendingOp = t;
      else {
        terms.push(cur!);
        signs.push(sign);
        sign = t === '-' ? -1 : 1;
      }
      expectNumber = true;
    }
  }
  if (expectNumber) return null;
  terms.push(cur!);
  signs.push(sign);
  const total = terms.reduce((sum, v, i) => sum + signs[i] * v, 0);
  const cents = Math.round(total * 100 + Number.EPSILON);
  return cents > 0 ? cents : null;
}

/** Same rules as the server's ledger.OwedCents, used for the live preview in the form. */
export function owedCents(
  amount: number,
  mode: string,
  payerPercent: number,
  otherShare: number,
): number | null {
  if (!(amount > 0)) return null;
  switch (mode) {
    case 'equal':
      return Math.floor(amount / 2);
    case 'percent': {
      if (payerPercent < 0 || payerPercent > 100) return null;
      const bp = Math.floor(payerPercent * 100 + 0.5);
      return Math.floor((amount * (10000 - bp)) / 10000);
    }
    case 'exact':
      return otherShare >= 0 && otherShare <= amount ? otherShare : null;
    case 'full':
      return amount;
  }
  return null;
}

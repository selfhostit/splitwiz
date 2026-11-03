// Reading a bank or card statement exported as CSV, for Settings > Import a
// bank statement. Banks all export differently, so this guesses the
// columns and date format, and the person can correct the guess. Pure
// functions, like stats.ts.

import type { Expense } from './pb';

/** Splits CSV text into rows of cells: comma, semicolon or tab separated, with "quoted" cells. */
export function parseCsv(text: string): string[][] {
  const src = text.replace(/^﻿/, '');
  const firstLines = src.split(/\r?\n/).slice(0, 10).join('\n');
  const count = (ch: string) => firstLines.split(ch).length - 1;
  const sep = [';', '\t', ','].reduce((best, ch) => (count(ch) > count(best) ? ch : best), ',');

  const rows: string[][] = [];
  let row: string[] = [];
  let cell = '';
  let quoted = false;
  for (let i = 0; i < src.length; i++) {
    const ch = src[i];
    if (quoted) {
      if (ch === '"' && src[i + 1] === '"') {
        cell += '"';
        i++;
      } else if (ch === '"') quoted = false;
      else cell += ch;
    } else if (ch === '"' && cell.trim() === '') {
      quoted = true;
      cell = '';
    } else if (ch === sep) {
      row.push(cell.trim());
      cell = '';
    } else if (ch === '\n' || ch === '\r') {
      if (ch === '\r' && src[i + 1] === '\n') i++;
      row.push(cell.trim());
      if (row.some((c) => c !== '')) rows.push(row);
      row = [];
      cell = '';
    } else cell += ch;
  }
  row.push(cell.trim());
  if (row.some((c) => c !== '')) rows.push(row);
  return rows;
}

export type DateOrder = 'ymd' | 'dmy' | 'mdy';

const DATE = /^(\d{1,4})[-/.](\d{1,2})[-/.](\d{1,4})(?:[ T].*)?$/;

/** Reads a date in the given order into YYYY-MM-DD; '' if it isn't one. */
export function parseDate(s: string, order: DateOrder): string {
  const m = DATE.exec(s.trim());
  if (!m) return '';
  let [y, mo, d] = [0, 0, 0];
  if (m[1].length === 4) [y, mo, d] = [+m[1], +m[2], +m[3]];
  else if (order === 'mdy') [mo, d, y] = [+m[1], +m[2], +m[3]];
  else [d, mo, y] = [+m[1], +m[2], +m[3]];
  if (y < 100) y += 2000;
  const date = new Date(Date.UTC(y, mo - 1, d));
  if (date.getUTCFullYear() !== y || date.getUTCMonth() !== mo - 1 || date.getUTCDate() !== d) return '';
  return date.toISOString().slice(0, 10);
}

/** Day-first unless a value only makes sense month-first (like 10/25/2026). */
export function guessDateOrder(values: string[]): DateOrder {
  let dmy = 0;
  let mdy = 0;
  for (const v of values) {
    const m = DATE.exec(v.trim());
    if (!m) continue;
    if (m[1].length === 4) return 'ymd';
    if (+m[1] > 12) dmy++;
    if (+m[2] > 12) mdy++;
  }
  return mdy > dmy ? 'mdy' : 'dmy';
}

/**
 * Reads a signed amount into cents: "-12,50", "12.50-", "(12.50)",
 * "€ 1.234,56", "+3". null when it isn't a number.
 */
export function parseSigned(s: string): number | null {
  let t = s.replace(/[€$£\s ]|EUR/gi, '');
  if (!t) return null;
  let sign = 1;
  if (/^\(.*\)$/.test(t)) {
    sign = -1;
    t = t.slice(1, -1);
  }
  if (t.startsWith('-') || t.startsWith('−')) {
    sign = -sign;
    t = t.slice(1);
  } else if (t.endsWith('-')) {
    sign = -sign;
    t = t.slice(0, -1);
  } else if (t.startsWith('+')) t = t.slice(1);
  if (!/^[\d.,']+$/.test(t)) return null;
  t = t.replace(/'/g, '');
  const lastComma = t.lastIndexOf(',');
  const lastDot = t.lastIndexOf('.');
  const dec = lastComma > lastDot ? ',' : '.';
  // "1,234" or "1.234" alone with three digits after is a thousands separator
  const decPos = Math.max(lastComma, lastDot);
  const hasDecimals = decPos >= 0 && t.length - decPos - 1 <= 2 && !(t.split(dec).length > 2);
  const whole = hasDecimals ? t.slice(0, decPos) : t;
  const frac = hasDecimals ? t.slice(decPos + 1) : '';
  const digits = whole.replace(/[.,]/g, '');
  if (!/^\d+$/.test(digits || '0') || (!digits && !frac)) return null;
  return sign * (parseInt(digits || '0', 10) * 100 + parseInt((frac + '00').slice(0, 2), 10));
}

export type Mapping = {
  /** index of the header row; data starts after it */
  header: number;
  date: number;
  description: number;
  /** the signed amount, or money out when `credit` is set */
  amount: number;
  /** a separate money-in column, -1 = none */
  credit: number;
  order: DateOrder;
};

const words = {
  date: ['date', 'datum', 'booking', 'buchungstag', 'valuta', 'transaction date', 'uitvoeringsdatum', "date d'exécution"],
  description: ['description', 'omschrijving', 'libellé', 'libelle', 'communication', 'mededeling', 'tegenpartij', 'counterparty', 'payee', 'merchant', 'naam', 'name', 'details', 'verwendungszweck', 'empfänger', 'beneficiary', 'bénéficiaire'],
  amount: ['amount', 'bedrag', 'montant', 'betrag', 'debit', 'débit', 'money out', 'paid out', 'uitgaven', 'af'],
  credit: ['credit', 'crédit', 'money in', 'paid in', 'bij', 'inkomsten'],
};

function headerScore(cell: string, kind: keyof typeof words): number {
  const c = cell.toLowerCase();
  return words[kind].reduce((best, w) => (c === w ? Math.max(best, 3) : c.includes(w) ? Math.max(best, 2) : best), 0);
}

/** Guesses which row is the header and which columns hold what. */
export function guessMapping(table: string[][]): Mapping {
  // the header is the first row with a date-ish and an amount-ish name
  let header = table.findIndex((r) => r.some((c) => headerScore(c, 'date')) && r.some((c) => headerScore(c, 'amount')));
  if (header < 0) header = 0;
  const head = table[header] ?? [];
  const data = table.slice(header + 1, header + 51);
  const width = Math.max(0, ...data.map((r) => r.length));
  const column = (i: number) => data.map((r) => r[i] ?? '').filter(Boolean);

  const order = guessDateOrder(data.flatMap((r) => r));
  const share = (i: number, ok: (v: string) => boolean) => {
    const vs = column(i);
    return vs.length ? vs.filter(ok).length / vs.length : 0;
  };
  const best = (score: (i: number) => number, skip: number[] = []) => {
    let at = -1;
    let top = 0;
    for (let i = 0; i < width; i++) {
      if (skip.includes(i)) continue;
      const s = score(i);
      if (s > top) [at, top] = [i, s];
    }
    return at;
  };

  const date = best((i) => headerScore(head[i] ?? '', 'date') + 2 * share(i, (v) => !!parseDate(v, order)));
  const amount = best((i) => headerScore(head[i] ?? '', 'amount') + 2 * share(i, (v) => parseSigned(v) !== null) - (i === date ? 9 : 0), [date]);
  const credit = best((i) => (headerScore(head[i] ?? '', 'credit') >= 2 ? 1 + share(i, (v) => parseSigned(v) !== null) : 0), [date, amount]);
  const avgLen = (i: number) => {
    const vs = column(i);
    return vs.length ? vs.reduce((s, v) => s + v.length, 0) / vs.length : 0;
  };
  const description = best(
    (i) => headerScore(head[i] ?? '', 'description') * 20 + Math.min(avgLen(i), 40) * (1 - share(i, (v) => parseSigned(v) !== null || !!parseDate(v, order))),
    [date, amount, credit],
  );
  return { header, date, description, amount, credit, order };
}

export type BankRow = {
  /** stays the same when the same statement is imported again */
  ref: string;
  date: string;
  description: string;
  /** money out, in cents */
  cents: number;
};

function hash(s: string): string {
  let h = 0x811c9dc5;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return (h >>> 0).toString(16).padStart(8, '0');
}

/**
 * The payments out of the account. When the amount column has negative
 * numbers those are the payments; otherwise (a money-out column) every
 * amount is one. Money coming in is left out.
 */
export function readRows(table: string[][], m: Mapping): BankRow[] {
  const data = table.slice(m.header + 1);
  const signed = data.map((r) => parseSigned(r[m.amount] ?? ''));
  const anyNegative = signed.some((v) => v !== null && v < 0);
  const seen = new Map<string, number>();
  const rows: BankRow[] = [];
  data.forEach((r, i) => {
    const date = parseDate(r[m.date] ?? '', m.order);
    const v = signed[i];
    if (!date || v === null || v === 0) return;
    if (m.credit >= 0 && parseSigned(r[m.credit] ?? '')) return;
    if (anyNegative && v > 0) return;
    const cents = Math.abs(v);
    const description = (r[m.description] ?? '').replace(/\s+/g, ' ').trim().slice(0, 200) || 'Card payment';
    // the same payment twice on one day keeps two refs
    const key = `${date}|${cents}|${description.toLowerCase()}`;
    const n = (seen.get(key) ?? 0) + 1;
    seen.set(key, n);
    rows.push({ ref: `bank:${hash(key)}${n > 1 ? `-${n}` : ''}`, date, description, cents });
  });
  return rows;
}

/** An expense already in Splitwiz that looks like this payment: same amount, within 3 days. */
export function lookAlike(expenses: Expense[], row: BankRow): Expense | undefined {
  const t = Date.parse(row.date);
  return expenses.find((e) => e.amount_cents === row.cents && Math.abs(Date.parse(e.date.slice(0, 10)) - t) <= 3 * 86400000);
}

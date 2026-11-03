import { describe, expect, it } from 'vitest';
import type { Expense } from './pb';
import { guessDateOrder, guessMapping, lookAlike, parseCsv, parseDate, parseSigned, readRows } from './bankcsv';

describe('parseCsv', () => {
  it('finds the separator and reads quoted cells', () => {
    expect(parseCsv('a;b;c\r\n1;"x;y";"say ""hi"""\n\n')).toEqual([
      ['a', 'b', 'c'],
      ['1', 'x;y', 'say "hi"'],
    ]);
    expect(parseCsv('﻿a,b\n1,2')).toEqual([
      ['a', 'b'],
      ['1', '2'],
    ]);
  });
});

describe('dates', () => {
  it('reads the common formats', () => {
    expect(parseDate('2026-10-03', 'dmy')).toBe('2026-10-03');
    expect(parseDate('03/10/2026', 'dmy')).toBe('2026-10-03');
    expect(parseDate('03.10.26', 'dmy')).toBe('2026-10-03');
    expect(parseDate('10/03/2026', 'mdy')).toBe('2026-10-03');
    expect(parseDate('2026-10-03 14:22:01', 'dmy')).toBe('2026-10-03');
    expect(parseDate('31/02/2026', 'dmy')).toBe('');
    expect(parseDate('hello', 'dmy')).toBe('');
  });

  it('guesses day-first unless the values say otherwise', () => {
    expect(guessDateOrder(['03/10/2026', '04/10/2026'])).toBe('dmy');
    expect(guessDateOrder(['10/25/2026', '10/03/2026'])).toBe('mdy');
    expect(guessDateOrder(['25/10/2026'])).toBe('dmy');
    expect(guessDateOrder(['2026-10-25'])).toBe('ymd');
  });
});

describe('parseSigned', () => {
  it.each([
    ['-12,50', -1250],
    ['12.50-', -1250],
    ['(12.50)', -1250],
    ['€ 1.234,56', 123456],
    ['-1,234.56', -123456],
    ['+3', 300],
    ['1 234,5', 123450],
    ['EUR -7,05', -705],
    ['1.234', 123400],
  ])('%s -> %i', (input, cents) => {
    expect(parseSigned(input)).toBe(cents);
  });

  it.each(['', 'abc', '12a'])('rejects %s', (input) => {
    expect(parseSigned(input)).toBeNull();
  });
});

// shaped like a Belgian bank's export, with an account line before the header
const belgian = `Rekening: BE12 3456;;;;
Datum;Omschrijving;Bedrag;Valuta;Tegenrekening
03/10/2026;DELHAIZE IXELLES;-54,20;EUR;BE00
03/10/2026;DELHAIZE IXELLES;-54,20;EUR;BE00
04/10/2026;Salary;2500,00;EUR;BE11
05/10/2026;NETFLIX.COM;-13,99;EUR;NL22`;

describe('guessMapping and readRows', () => {
  it('reads a signed amount column and skips money coming in', () => {
    const table = parseCsv(belgian);
    const m = guessMapping(table);
    expect(m).toMatchObject({ header: 1, date: 0, description: 1, amount: 2, order: 'dmy' });
    const rows = readRows(table, m);
    expect(rows.map((r) => [r.date, r.description, r.cents])).toEqual([
      ['2026-10-03', 'DELHAIZE IXELLES', 5420],
      ['2026-10-03', 'DELHAIZE IXELLES', 5420],
      ['2026-10-05', 'NETFLIX.COM', 1399],
    ]);
    // two identical payments on one day stay two, and importing again gives the same refs
    expect(rows[0].ref).not.toBe(rows[1].ref);
    expect(readRows(table, m).map((r) => r.ref)).toEqual(rows.map((r) => r.ref));
  });

  it('reads separate money out and money in columns', () => {
    const table = parseCsv('Date,Description,Money Out,Money In\n2026-10-01,Coffee,3.20,\n2026-10-02,Refund,,10.00\n');
    const m = guessMapping(table);
    expect(m).toMatchObject({ header: 0, date: 0, description: 1, amount: 2, credit: 3 });
    expect(readRows(table, m).map((r) => [r.description, r.cents])).toEqual([['Coffee', 320]]);
  });
});

describe('lookAlike', () => {
  it('finds an expense with the same amount within three days', () => {
    const expenses = [{ id: 'e', amount_cents: 5420, date: '2026-10-05 12:00:00.000Z' }] as Expense[];
    expect(lookAlike(expenses, { ref: '', date: '2026-10-03', description: '', cents: 5420 })?.id).toBe('e');
    expect(lookAlike(expenses, { ref: '', date: '2026-09-28', description: '', cents: 5420 })).toBeUndefined();
    expect(lookAlike(expenses, { ref: '', date: '2026-10-03', description: '', cents: 5421 })).toBeUndefined();
  });
});

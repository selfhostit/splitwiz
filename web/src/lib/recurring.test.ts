import { describe, expect, it } from 'vitest';
import type { Expense, Recurring } from './pb';
import { amountOn, countUntil, hasEnded, nextDate, nthPayment, payments, suggestMonthly, upcoming, type Plan } from './recurring';

const ex = (date: string, description: string, amount_cents = 9839, extra: Partial<Expense> = {}): Expense =>
  ({
    id: date + description,
    date: `${date} 12:00:00.000Z`,
    description,
    amount_cents,
    paid_by: 'a',
    split_mode: 'equal',
    owed_cents: Math.floor(amount_cents / 2),
    recurring: '',
    ...extra,
  }) as Expense;

describe('nextDate', () => {
  it('matches the server rules', () => {
    expect(nextDate('2026-10-16', 'month')).toBe('2026-11-16');
    expect(nextDate('2026-12-05', 'month')).toBe('2027-01-05');
    expect(nextDate('2027-01-31', 'month')).toBe('2027-02-28');
    expect(nextDate('2026-12-28', 'week')).toBe('2027-01-04');
    expect(nextDate('2028-02-29', 'year')).toBe('2029-02-28');
  });
});

describe('suggestMonthly', () => {
  const today = '2026-10-17';

  it('finds a charge repeated on about the same day for three months', () => {
    const list = [
      ex('2026-08-16', 'KBC insurance car'),
      ex('2026-09-16', 'KBC insurance  car '),
      ex('2026-10-16', 'kbc insurance car'),
      ex('2026-10-01', 'Fuel', 7441),
      ex('2026-09-02', 'Fuel', 5380),
      ex('2026-08-05', 'Fuel', 7156),
    ];
    const s = suggestMonthly(list, [], today);
    expect(s).toHaveLength(1);
    expect(s[0].latest.date.slice(0, 10)).toBe('2026-10-16');
    expect(s[0].matches).toHaveLength(3);
  });

  it('skips stopped, irregular, existing or already linked charges', () => {
    const stopped = [ex('2026-05-28', 'Rent'), ex('2026-06-28', 'Rent'), ex('2026-07-28', 'Rent')];
    const gap = [ex('2026-06-14', 'Gym'), ex('2026-08-14', 'Gym'), ex('2026-10-14', 'Gym')];
    const drift = [ex('2026-08-01', 'Box'), ex('2026-09-15', 'Box'), ex('2026-10-01', 'Box')];
    const linked = [ex('2026-08-09', 'Power', 100, { recurring: 'x' }), ex('2026-09-09', 'Power', 100, { recurring: 'x' }), ex('2026-10-09', 'Power', 100, { recurring: 'x' })];
    const known = [ex('2026-08-14', 'KBC'), ex('2026-09-14', 'KBC'), ex('2026-10-14', 'KBC')];
    const series = [{ description: 'kbc' } as Recurring];
    expect(suggestMonthly([...stopped, ...gap, ...drift, ...linked, ...known], series, today)).toEqual([]);
  });
});

describe('upcoming', () => {
  const s = (id: string, next: string, every: Recurring['every'], extra: Partial<Recurring> = {}): Recurring =>
    ({ id, amount_cents: 100, every, next_date: `${next} 12:00:00.000Z`, anchor_day: 0, paused: false, ...extra }) as Recurring;

  it('lists every date in the window, soonest first', () => {
    const list = upcoming([s('rent', '2026-11-01', 'month'), s('gym', '2026-10-20', 'week'), s('off', '2026-10-19', 'week', { paused: true })], '2026-10-18', '2026-11-17');
    expect(list.map((o) => `${o.series.id} ${o.day}`)).toEqual([
      'gym 2026-10-20',
      'gym 2026-10-27',
      'rent 2026-11-01',
      'gym 2026-11-03',
      'gym 2026-11-10',
      'gym 2026-11-17',
    ]);
  });

  it('keeps monthly series on their anchor day after a short month', () => {
    const list = upcoming([s('x', '2027-02-28', 'month', { anchor_day: 31 })], '2027-02-01', '2027-04-30');
    expect(list.map((o) => o.day)).toEqual(['2027-02-28', '2027-03-31', '2027-04-30']);
  });
});

describe('amount changes and end dates', () => {
  const changes = [
    { from: '2027-03-01', amount_cents: 9500 },
    { from: '2027-01-01', amount_cents: 8500 },
  ];

  it('picks the latest change that has started', () => {
    expect(amountOn(8000, changes, '2026-12-15')).toBe(8000);
    expect(amountOn(8000, changes, '2027-01-01')).toBe(8500);
    expect(amountOn(8000, changes, '2027-02-15')).toBe(8500);
    expect(amountOn(8000, changes, '2030-01-01')).toBe(9500);
    expect(amountOn(8000, null, '2030-01-01')).toBe(8000);
  });

  it('lists payments by phase until the end', () => {
    const plan: Plan = { start: '2026-11-15', every: 'month', amount: 8000, changes, ends: '2027-04-30' };
    expect(payments(plan, 100).map((p) => `${p.day} ${p.amount_cents} ${p.phase}`)).toEqual([
      '2026-11-15 8000 0',
      '2026-12-15 8000 0',
      '2027-01-15 8500 1',
      '2027-02-15 8500 1',
      '2027-03-15 9500 2',
      '2027-04-15 9500 2',
    ]);
    expect(countUntil(plan, '2027-01-15')).toBe(3);
    expect(nthPayment(plan, 3)).toBe('2027-01-15');
    // stays on the 30th after February, like the server
    expect(nthPayment({ ...plan, start: '2026-10-30' }, 6)).toBe('2027-03-30');
  });

  it('shows changed amounts and stops after the end in upcoming', () => {
    const s = {
      id: 'net',
      amount_cents: 8000,
      every: 'month',
      next_date: '2026-11-15 12:00:00.000Z',
      anchor_day: 15,
      paused: false,
      ends_on: '2027-02-15 12:00:00.000Z',
      changes,
    } as Recurring;
    const list = upcoming([s], '2026-11-01', '2027-12-31');
    expect(list.map((o) => `${o.day} ${o.amount_cents}${o.changed ? ' new' : ''}${o.last ? ' last' : ''}`)).toEqual([
      '2026-11-15 8000',
      '2026-12-15 8000',
      '2027-01-15 8500 new',
      '2027-02-15 8500 last',
    ]);
  });

  it('knows when a series has ended', () => {
    expect(hasEnded({ ends_on: '', next_date: '2030-01-01 12:00:00.000Z' })).toBe(false);
    expect(hasEnded({ ends_on: '2027-02-15 12:00:00.000Z', next_date: '2027-02-15 12:00:00.000Z' })).toBe(false);
    expect(hasEnded({ ends_on: '2027-02-15 12:00:00.000Z', next_date: '2027-03-15 12:00:00.000Z' })).toBe(true);
  });
});

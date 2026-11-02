import { describe, expect, it } from 'vitest';
import type { Expense } from './pb';
import { favorites, lastLike, suggest } from './quickadd';

let n = 0;
const ex = (date: string, description: string, cents = 100): Expense =>
  ({ id: String(n++), date: `${date} 12:00:00.000Z`, description, amount_cents: cents }) as Expense;

// newest first, as the app keeps them
const list = [
  ex('2026-10-18', 'Groceries', 4200),
  ex('2026-10-15', 'Coffee', 350),
  ex('2026-10-11', 'groceries ', 3900),
  ex('2026-10-10', 'Coffee', 300),
  ex('2026-10-08', 'Coffee', 300),
  ex('2026-10-01', 'Cinema', 2400),
  ex('2026-06-01', 'Rent', 100000),
  ex('2026-05-01', 'Rent', 100000),
];

describe('lastLike', () => {
  it('finds the latest copy, ignoring case and spaces', () => {
    expect(lastLike(list, ' GROCERIES')?.amount_cents).toBe(4200);
    expect(lastLike(list, 'Nope')).toBeUndefined();
    expect(lastLike(list, '  ')).toBeUndefined();
  });
});

describe('favorites', () => {
  it('lists what came back in the last 90 days, most often first', () => {
    const favs = favorites(list, '2026-10-20');
    expect(favs.map((f) => f.description)).toEqual(['Coffee', 'Groceries']);
    expect(favs[0].amount_cents).toBe(350);
  });

  it('stops at n', () => {
    expect(favorites(list, '2026-10-20', 1)).toHaveLength(1);
  });
});

describe('suggest', () => {
  it('puts words starting with the text first, then the most used', () => {
    const more = [...list, ex('2026-09-01', 'Big coffee beans', 1800), ex('2026-09-02', 'Decaf', 900)];
    expect(suggest(more, 'co').map((e) => e.description)).toEqual(['Coffee', 'Big coffee beans']);
    expect(suggest(more, 'caf').map((e) => e.description)).toEqual(['Decaf']);
  });

  it('gives the latest copy, and nothing for an exact match or blank text', () => {
    expect(suggest(list, 'groc')).toHaveLength(1);
    expect(suggest(list, 'groc')[0].amount_cents).toBe(4200);
    expect(suggest(list, 'coffee')).toEqual([]);
    expect(suggest(list, ' ')).toEqual([]);
  });
});

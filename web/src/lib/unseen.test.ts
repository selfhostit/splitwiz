import { describe, expect, it } from 'vitest';
import type { Comment, Expense } from './pb';
import type { Entry } from './state.svelte';
import { describeUnseen, unseen } from './unseen';

const SEEN = '2026-11-01 10:00:00.000Z';
const entry = (id: string, created: string, updated: string, updated_by: string): Entry => {
  const item = { id, created, updated, updated_by, date: created } as Expense;
  return { kind: 'expense', date: created, created, item };
};
const before = '2026-10-30 09:00:00.000Z';
const after = '2026-11-02 09:00:00.000Z';

const entries = [
  entry('new', after, after, 'sam'),
  entry('edited', before, after, 'sam'),
  entry('mine', after, after, 'alex'),
  entry('old', before, before, 'sam'),
  entry('server', after, after, ''),
  entry('talked', before, before, 'alex'),
];
const comments = [
  { id: 'c1', expense: 'talked', author: 'sam', text: 'hi', created: after },
  { id: 'c2', expense: 'mine', author: 'alex', text: 'me', created: after },
] as Comment[];

describe('unseen', () => {
  it("finds what the other person added, changed or commented on", () => {
    expect(Object.fromEntries(unseen(entries, comments, 'alex', SEEN))).toEqual({ new: 'added', edited: 'changed', talked: 'commented' });
  });

  it('shows nothing before a first look', () => {
    expect(unseen(entries, comments, 'alex', '').size).toBe(0);
  });
});

describe('describeUnseen', () => {
  it('sums it up in a sentence', () => {
    const why = unseen(entries, comments, 'alex', SEEN);
    expect(describeUnseen('Sam', why)).toBe('Sam added 1, changed 1 and commented on 1 since you last looked');
    expect(describeUnseen('Sam', new Map([['a', 'added'], ['b', 'added']]))).toBe('Sam added 2 since you last looked');
    expect(describeUnseen('Sam', new Map())).toBe('');
  });
});

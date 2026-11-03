import { describe, expect, it } from 'vitest';
import { evalAmount, isSum, owedCents, parseAmount } from './money';

describe('parseAmount', () => {
  it.each([
    ['12', 1200],
    ['12.5', 1250],
    ['12,50', 1250],
    ['€ 7,05', 705],
    ['1.234,56', 123456],
    ['1,234.56', 123456],
    ['0.01', 1],
  ])('%s -> %i', (input, cents) => {
    expect(parseAmount(input)).toBe(cents);
  });

  it.each(['', 'abc', '0', '-5', '1.234.5', '3.999'])('rejects %s', (input) => {
    expect(parseAmount(input)).toBeNull();
  });
});

describe('owedCents matches the server', () => {
  it('splits equally with the odd cent on the payer', () => {
    expect(owedCents(1001, 'equal', 0, 0)).toBe(500);
  });
  it('splits by percent', () => {
    expect(owedCents(10000, 'percent', 60, 0)).toBe(4000);
    expect(owedCents(1001, 'percent', 60, 0)).toBe(400);
    expect(owedCents(10000, 'percent', 33.33, 0)).toBe(6667);
  });
  it('handles exact and full', () => {
    expect(owedCents(2500, 'exact', 0, 700)).toBe(700);
    expect(owedCents(2500, 'exact', 0, 2600)).toBeNull();
    expect(owedCents(2500, 'full', 0, 0)).toBe(2500);
  });
});

describe('evalAmount', () => {
  it.each([
    ['12.50', 1250],
    ['12.50+8,30', 2080],
    ['90/3', 3000],
    ['100/3', 3333],
    ['3 × 4.20', 1260],
    ['3x4,20', 1260],
    ['20 - 4.5', 1550],
    ['20 − 4.5', 1550],
    ['2+3*4', 1400],
    ['10*0+5', 500],
    ['€ 7 + € 3', 1000],
    ['0.1+0.2', 30],
  ])('%s -> %i', (input, cents) => {
    expect(evalAmount(input)).toBe(cents);
  });

  it.each(['', '5+', '+', '5-10', '4/0', 'abc+1', '1.234.5+1'])('rejects %s', (input) => {
    expect(evalAmount(input)).toBeNull();
  });

  it('tells a sum from an amount', () => {
    expect(isSum('12,50')).toBe(false);
    expect(isSum('12+3')).toBe(true);
  });
});

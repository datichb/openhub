/**
 * Tests unitaires pour FigmaClient
 * Couvre : withRetry, classifyFigmaError, timeout via config
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { classifyFigmaError } from '../client.js';
import { makeAxiosError } from './test-utils.js';

// ── classifyFigmaError ────────────────────────────────────────────────────────

describe('classifyFigmaError', () => {
  it.each([
    ['ECONNABORTED', 30000, ['indisponible', 'timeout 30s']],
    ['ETIMEDOUT',    10000, ['indisponible', 'timeout 10s']],
    ['ERR_NETWORK',  30000, ['indisponible']],
  ] as [string, number, string[]][])('identifie un timeout/réseau (%s)', (code, timeout, keywords) => {
    const err = makeAxiosError({ code, timeout });
    const msg = classifyFigmaError(err, 1, 2);
    for (const kw of keywords) expect(msg).toContain(kw);
  });

  it.each([
    [401, ['401', 'Token Figma invalide', 'oc figma status']],
    [403, ['403', 'scopes']],
    [404, ['404', 'introuvable']],
    [429, ['429']],
    [503, ['503', 'temporairement indisponible']],
    [504, ['504']],
  ] as [number, string[]][])('identifie le statut HTTP %i', (status, keywords) => {
    const err = makeAxiosError({ status });
    const msg = classifyFigmaError(err, 1, 2);
    for (const kw of keywords) expect(msg).toContain(kw);
  });

  it('identifie un rate-limit (429) — contient "limite"', () => {
    const err = makeAxiosError({ status: 429 });
    expect(classifyFigmaError(err, 1, 2).toLowerCase()).toContain('limite');
  });

  it('retourne le message brut pour une erreur non-axios', () => {
    const err = new Error('unexpected failure');
    expect(classifyFigmaError(err, 1, 2)).toContain('unexpected failure');
  });

  it('retourne une erreur générique pour un statut inconnu', () => {
    const err = makeAxiosError({ status: 500, message: 'server crash' });
    expect(classifyFigmaError(err, 1, 2)).toContain('500');
  });

  it('inclut le contexte tentative dans le message timeout', () => {
    const err = makeAxiosError({ code: 'ECONNABORTED', timeout: 30000 });
    expect(classifyFigmaError(err, 3, 2)).toContain('tentative 3/2');
  });
});

// ── getConfig timeout ─────────────────────────────────────────────────────────

describe('getConfig timeout parsing', () => {
  beforeEach(() => {
    vi.resetModules();
    delete process.env.FIGMA_TIMEOUT;
    delete process.env.FIGMA_MAX_RETRIES;
    delete process.env.FIGMA_PERSONAL_ACCESS_TOKEN;
    delete process.env.FIGMA_TEAM_ID;
  });

  afterEach(() => {
    delete process.env.FIGMA_PERSONAL_ACCESS_TOKEN;
    delete process.env.FIGMA_TEAM_ID;
    delete process.env.FIGMA_TIMEOUT;
    delete process.env.FIGMA_MAX_RETRIES;
  });

  it('lève une erreur si FIGMA_PERSONAL_ACCESS_TOKEN est absent', async () => {
    process.env.FIGMA_TEAM_ID = '123';
    delete process.env.FIGMA_PERSONAL_ACCESS_TOKEN;
    const { getConfig } = await import('../config.js');
    expect(() => getConfig()).toThrow('FIGMA_PERSONAL_ACCESS_TOKEN');
  });

  it('lève une erreur si FIGMA_TEAM_ID est absent', async () => {
    process.env.FIGMA_PERSONAL_ACCESS_TOKEN = 'figd_test';
    delete process.env.FIGMA_TEAM_ID;
    const { getConfig } = await import('../config.js');
    expect(() => getConfig()).toThrow('FIGMA_TEAM_ID');
  });

  it.each([
    ['FIGMA_TIMEOUT',     'timeout',    undefined,     30000],
    ['FIGMA_TIMEOUT',     'timeout',    '60000',       60000],
    ['FIGMA_TIMEOUT',     'timeout',    'not-a-number',30000],
    ['FIGMA_MAX_RETRIES', 'maxRetries', undefined,     2],
    ['FIGMA_MAX_RETRIES', 'maxRetries', '3',           3],
  ] as [string, 'timeout' | 'maxRetries', string | undefined, number][])(
    'parse %s=%s → %s=%d',
    async (envKey, configKey, envVal, expected) => {
      process.env.FIGMA_PERSONAL_ACCESS_TOKEN = 'figd_test';
      process.env.FIGMA_TEAM_ID = '123';
      if (envVal !== undefined) process.env[envKey] = envVal;
      const { getConfig } = await import('../config.js');
      expect(getConfig()[configKey]).toBe(expected);
    }
  );
});

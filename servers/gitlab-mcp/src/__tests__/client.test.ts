/**
 * Tests unitaires pour GitLabClient
 * Couvre : classifyGitlabError, withRetry (via client), getConfig
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { classifyGitlabError } from '../client.js';
import { makeAxiosError } from './test-utils.js';

// ── classifyGitlabError ───────────────────────────────────────────────────────

describe('classifyGitlabError', () => {
  it.each([
    [401, ['authentication', 'token']],
    [403, ['forbidden', 'scopes']],
    [404, ['not found']],
    [429, ['rate limit']],
    [503, ['unavailable']],
  ])('identifie le statut HTTP %i', (status, keywords: string[]) => {
    const err = makeAxiosError({ status });
    const msg = classifyGitlabError(err).toLowerCase();
    for (const kw of keywords) expect(msg).toContain(kw);
  });

  it.each([
    ['ECONNABORTED'],
    ['ETIMEDOUT'],
  ])('identifie un timeout (%s)', (code) => {
    const err = makeAxiosError({ code });
    expect(classifyGitlabError(err).toLowerCase()).toContain('timed out');
  });

  it('retourne le message brut pour une erreur non-axios', () => {
    const err = new Error('unexpected failure');
    expect(classifyGitlabError(err)).toContain('unexpected failure');
  });

  it('retourne "Unknown error" pour une valeur non-Error non-axios', () => {
    expect(classifyGitlabError({ foo: 'bar' })).toContain('Unknown error');
  });

  it('inclut le message axios pour un statut inconnu', () => {
    const err = makeAxiosError({ status: 500, message: 'Internal Server Error' });
    const msg = classifyGitlabError(err);
    expect(msg.toLowerCase()).toContain('gitlab api error');
    expect(msg).toContain('Internal Server Error');
  });
});

// ── getConfig ─────────────────────────────────────────────────────────────────

describe('getConfig', () => {
  beforeEach(() => {
    vi.resetModules();
    delete process.env.GITLAB_PERSONAL_ACCESS_TOKEN;
    delete process.env.GITLAB_BASE_URL;
    delete process.env.GITLAB_TIMEOUT;
    delete process.env.GITLAB_MAX_RETRIES;
  });

  afterEach(() => {
    delete process.env.GITLAB_PERSONAL_ACCESS_TOKEN;
    delete process.env.GITLAB_BASE_URL;
    delete process.env.GITLAB_TIMEOUT;
    delete process.env.GITLAB_MAX_RETRIES;
    delete process.env.GITLAB_ALLOW_HTTP;
  });

  it('lève une erreur si GITLAB_PERSONAL_ACCESS_TOKEN est absent', async () => {
    const { getConfig } = await import('../config.js');
    expect(() => getConfig()).toThrow('GITLAB_PERSONAL_ACCESS_TOKEN');
  });

  it('retourne la config avec le token fourni', async () => {
    process.env.GITLAB_PERSONAL_ACCESS_TOKEN = 'glpat-test-token';
    const { getConfig } = await import('../config.js');
    expect(getConfig().token).toBe('glpat-test-token');
  });

  it('utilise https://gitlab.com comme baseUrl par défaut', async () => {
    process.env.GITLAB_PERSONAL_ACCESS_TOKEN = 'glpat-test';
    const { getConfig } = await import('../config.js');
    expect(getConfig().baseUrl).toBe('https://gitlab.com');
  });

  it("lit GITLAB_BASE_URL depuis l'env et supprime le slash final", async () => {
    process.env.GITLAB_PERSONAL_ACCESS_TOKEN = 'glpat-test';
    process.env.GITLAB_BASE_URL = 'https://my-gitlab.example.com/';
    const { getConfig } = await import('../config.js');
    expect(getConfig().baseUrl).toBe('https://my-gitlab.example.com');
  });

  it('rejette une URL HTTP non-HTTPS (sécurité token)', async () => {
    process.env.GITLAB_PERSONAL_ACCESS_TOKEN = 'glpat-test';
    process.env.GITLAB_BASE_URL = 'http://my-gitlab.example.com';
    const { getConfig } = await import('../config.js');
    expect(() => getConfig()).toThrow('HTTPS');
  });

  it('rejette une URL invalide (malformée)', async () => {
    process.env.GITLAB_PERSONAL_ACCESS_TOKEN = 'glpat-test';
    process.env.GITLAB_BASE_URL = 'not-a-valid-url';
    const { getConfig } = await import('../config.js');
    expect(() => getConfig()).toThrow('valid URL');
  });

  it('accepte HTTP si GITLAB_ALLOW_HTTP=1 (opt-out explicite)', async () => {
    process.env.GITLAB_PERSONAL_ACCESS_TOKEN = 'glpat-test';
    process.env.GITLAB_BASE_URL = 'http://internal-gitlab.corp.local';
    process.env.GITLAB_ALLOW_HTTP = '1';
    const { getConfig } = await import('../config.js');
    expect(getConfig().baseUrl).toBe('http://internal-gitlab.corp.local');
    delete process.env.GITLAB_ALLOW_HTTP;
  });

  it('accepte une URL HTTPS valide sans slash final', async () => {
    process.env.GITLAB_PERSONAL_ACCESS_TOKEN = 'glpat-test';
    process.env.GITLAB_BASE_URL = 'https://gitlab.company.io';
    const { getConfig } = await import('../config.js');
    expect(getConfig().baseUrl).toBe('https://gitlab.company.io');
  });

  it.each([
    ['GITLAB_TIMEOUT',     'timeout',    undefined,     30000],
    ['GITLAB_TIMEOUT',     'timeout',    '60000',       60000],
    ['GITLAB_TIMEOUT',     'timeout',    'not-a-number',30000],
    ['GITLAB_MAX_RETRIES', 'maxRetries', undefined,     2],
    ['GITLAB_MAX_RETRIES', 'maxRetries', '5',           5],
    ['GITLAB_MAX_RETRIES', 'maxRetries', 'bad',         2],
  ] as [string, 'timeout' | 'maxRetries', string | undefined, number][])(
    'parse %s=%s → %s=%d',
    async (envKey, configKey, envVal, expected) => {
      process.env.GITLAB_PERSONAL_ACCESS_TOKEN = 'glpat-test';
      if (envVal !== undefined) process.env[envKey] = envVal;
      const { getConfig } = await import('../config.js');
      expect(getConfig()[configKey]).toBe(expected);
    }
  );
});

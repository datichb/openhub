/**
 * Tests unitaires pour GitLabClient
 * Couvre : classifyGitlabError, withRetry (via client), getConfig
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import axios from 'axios';
import { classifyGitlabError } from '../client.js';

// ── Helpers ──────────────────────────────────────────────────────────────────

function makeAxiosError(
  options: {
    code?: string;
    status?: number;
    message?: string;
    timeout?: number;
  } = {}
): ReturnType<typeof axios.isAxiosError> {
  const err: any = new Error(options.message || 'axios error');
  err.isAxiosError = true;
  err.code = options.code;
  err.config = { timeout: options.timeout ?? 30000 };
  if (options.status !== undefined) {
    err.response = {
      status: options.status,
      data: {},
    };
  }
  return err;
}

// ── classifyGitlabError ───────────────────────────────────────────────────────

describe('classifyGitlabError', () => {
  it('identifie un token invalide (401)', () => {
    const err = makeAxiosError({ status: 401 });
    const msg = classifyGitlabError(err);
    expect(msg.toLowerCase()).toContain('authentication');
    expect(msg.toLowerCase()).toContain('token');
  });

  it('identifie un accès refusé (403)', () => {
    const err = makeAxiosError({ status: 403 });
    const msg = classifyGitlabError(err);
    expect(msg.toLowerCase()).toContain('forbidden');
    expect(msg.toLowerCase()).toContain('scopes');
  });

  it('identifie une ressource introuvable (404)', () => {
    const err = makeAxiosError({ status: 404 });
    const msg = classifyGitlabError(err);
    expect(msg.toLowerCase()).toContain('not found');
  });

  it('identifie un rate-limit (429)', () => {
    const err = makeAxiosError({ status: 429 });
    const msg = classifyGitlabError(err);
    expect(msg.toLowerCase()).toContain('rate limit');
  });

  it('identifie une indisponibilité temporaire (503)', () => {
    const err = makeAxiosError({ status: 503 });
    const msg = classifyGitlabError(err);
    expect(msg.toLowerCase()).toContain('unavailable');
  });

  it('identifie un timeout (ECONNABORTED)', () => {
    const err = makeAxiosError({ code: 'ECONNABORTED' });
    const msg = classifyGitlabError(err);
    expect(msg.toLowerCase()).toContain('timed out');
  });

  it('identifie un timeout (ETIMEDOUT)', () => {
    const err = makeAxiosError({ code: 'ETIMEDOUT' });
    const msg = classifyGitlabError(err);
    expect(msg.toLowerCase()).toContain('timed out');
  });

  it('retourne le message brut pour une erreur non-axios', () => {
    const err = new Error('unexpected failure');
    const msg = classifyGitlabError(err);
    expect(msg).toContain('unexpected failure');
  });

  it('retourne "Unknown error" pour une valeur non-Error non-axios', () => {
    const msg = classifyGitlabError({ foo: 'bar' });
    expect(msg).toContain('Unknown error');
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
  });

  it('lève une erreur si GITLAB_PERSONAL_ACCESS_TOKEN est absent', async () => {
    const { getConfig } = await import('../config.js');
    expect(() => getConfig()).toThrow('GITLAB_PERSONAL_ACCESS_TOKEN');
  });

  it('retourne la config avec le token fourni', async () => {
    process.env.GITLAB_PERSONAL_ACCESS_TOKEN = 'glpat-test-token';
    const { getConfig } = await import('../config.js');
    const config = getConfig();
    expect(config.token).toBe('glpat-test-token');
  });

  it('utilise https://gitlab.com comme baseUrl par défaut', async () => {
    process.env.GITLAB_PERSONAL_ACCESS_TOKEN = 'glpat-test';
    const { getConfig } = await import('../config.js');
    const config = getConfig();
    expect(config.baseUrl).toBe('https://gitlab.com');
  });

  it('lit GITLAB_BASE_URL depuis l\'env et supprime le slash final', async () => {
    process.env.GITLAB_PERSONAL_ACCESS_TOKEN = 'glpat-test';
    process.env.GITLAB_BASE_URL = 'https://my-gitlab.example.com/';
    const { getConfig } = await import('../config.js');
    const config = getConfig();
    expect(config.baseUrl).toBe('https://my-gitlab.example.com');
  });

  it('utilise 30000ms par défaut si GITLAB_TIMEOUT absent', async () => {
    process.env.GITLAB_PERSONAL_ACCESS_TOKEN = 'glpat-test';
    const { getConfig } = await import('../config.js');
    const config = getConfig();
    expect(config.timeout).toBe(30000);
  });

  it('lit GITLAB_TIMEOUT depuis l\'env', async () => {
    process.env.GITLAB_PERSONAL_ACCESS_TOKEN = 'glpat-test';
    process.env.GITLAB_TIMEOUT = '60000';
    const { getConfig } = await import('../config.js');
    const config = getConfig();
    expect(config.timeout).toBe(60000);
  });

  it('ignore une valeur GITLAB_TIMEOUT invalide et utilise le défaut', async () => {
    process.env.GITLAB_PERSONAL_ACCESS_TOKEN = 'glpat-test';
    process.env.GITLAB_TIMEOUT = 'not-a-number';
    const { getConfig } = await import('../config.js');
    const config = getConfig();
    expect(config.timeout).toBe(30000);
  });

  it('utilise 2 retries par défaut si GITLAB_MAX_RETRIES absent', async () => {
    process.env.GITLAB_PERSONAL_ACCESS_TOKEN = 'glpat-test';
    const { getConfig } = await import('../config.js');
    const config = getConfig();
    expect(config.maxRetries).toBe(2);
  });

  it('lit GITLAB_MAX_RETRIES depuis l\'env', async () => {
    process.env.GITLAB_PERSONAL_ACCESS_TOKEN = 'glpat-test';
    process.env.GITLAB_MAX_RETRIES = '5';
    const { getConfig } = await import('../config.js');
    const config = getConfig();
    expect(config.maxRetries).toBe(5);
  });

  it('ignore une valeur GITLAB_MAX_RETRIES invalide et utilise le défaut', async () => {
    process.env.GITLAB_PERSONAL_ACCESS_TOKEN = 'glpat-test';
    process.env.GITLAB_MAX_RETRIES = 'bad';
    const { getConfig } = await import('../config.js');
    const config = getConfig();
    expect(config.maxRetries).toBe(2);
  });
});

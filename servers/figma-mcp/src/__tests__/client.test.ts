/**
 * Tests unitaires pour FigmaClient
 * Couvre : withRetry, classifyFigmaError, timeout via config
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import axios from 'axios';
import { classifyFigmaError } from '../client.js';

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

// ── classifyFigmaError ────────────────────────────────────────────────────────

describe('classifyFigmaError', () => {
  it('identifie un timeout (ECONNABORTED)', () => {
    const err = makeAxiosError({ code: 'ECONNABORTED', timeout: 30000 });
    const msg = classifyFigmaError(err, 3, 2);
    expect(msg).toContain('indisponible');
    expect(msg).toContain('timeout 30s');
    expect(msg).toContain('tentative 3/2');
  });

  it('identifie un timeout (ETIMEDOUT)', () => {
    const err = makeAxiosError({ code: 'ETIMEDOUT', timeout: 10000 });
    const msg = classifyFigmaError(err, 1, 2);
    expect(msg).toContain('indisponible');
    expect(msg).toContain('timeout 10s');
  });

  it('identifie une erreur réseau (ERR_NETWORK)', () => {
    const err = makeAxiosError({ code: 'ERR_NETWORK' });
    const msg = classifyFigmaError(err, 1, 2);
    expect(msg).toContain('indisponible');
  });

  it('identifie un token invalide (401)', () => {
    const err = makeAxiosError({ status: 401 });
    const msg = classifyFigmaError(err, 1, 2);
    expect(msg).toContain('401');
    expect(msg).toContain('Token Figma invalide');
    expect(msg).toContain('oc figma status');
  });

  it('identifie un accès refusé (403)', () => {
    const err = makeAxiosError({ status: 403 });
    const msg = classifyFigmaError(err, 1, 2);
    expect(msg).toContain('403');
    expect(msg).toContain('scopes');
  });

  it('identifie une ressource introuvable (404)', () => {
    const err = makeAxiosError({ status: 404 });
    const msg = classifyFigmaError(err, 1, 2);
    expect(msg).toContain('404');
    expect(msg).toContain('introuvable');
  });

  it('identifie un rate-limit (429)', () => {
    const err = makeAxiosError({ status: 429 });
    const msg = classifyFigmaError(err, 1, 2);
    expect(msg).toContain('429');
    expect(msg.toLowerCase()).toContain('limite');
  });

  it('identifie une indisponibilité temporaire (503)', () => {
    const err = makeAxiosError({ status: 503 });
    const msg = classifyFigmaError(err, 1, 2);
    expect(msg).toContain('503');
    expect(msg).toContain('temporairement indisponible');
  });

  it('identifie une indisponibilité temporaire (504)', () => {
    const err = makeAxiosError({ status: 504 });
    const msg = classifyFigmaError(err, 1, 2);
    expect(msg).toContain('504');
  });

  it('retourne le message brut pour une erreur non-axios', () => {
    const err = new Error('unexpected failure');
    const msg = classifyFigmaError(err, 1, 2);
    expect(msg).toContain('unexpected failure');
  });

  it('retourne une erreur générique pour un statut inconnu', () => {
    const err = makeAxiosError({ status: 500, message: 'server crash' });
    const msg = classifyFigmaError(err, 1, 2);
    expect(msg).toContain('500');
  });
});

// ── getConfig timeout ─────────────────────────────────────────────────────────

describe('getConfig timeout parsing', () => {
  beforeEach(() => {
    vi.resetModules();
    // Nettoyer les env vars entre les tests
    delete process.env.FIGMA_TIMEOUT;
    delete process.env.FIGMA_MAX_RETRIES;
    delete process.env.FIGMA_PERSONAL_ACCESS_TOKEN;
    delete process.env.FIGMA_TEAM_ID;
  });

  afterEach(() => {
    // Garantir le nettoyage même si le test échoue
    delete process.env.FIGMA_PERSONAL_ACCESS_TOKEN;
    delete process.env.FIGMA_TEAM_ID;
    delete process.env.FIGMA_TIMEOUT;
    delete process.env.FIGMA_MAX_RETRIES;
  });

  it('utilise 30000ms par défaut si FIGMA_TIMEOUT absent', async () => {
    process.env.FIGMA_PERSONAL_ACCESS_TOKEN = 'figd_test';
    process.env.FIGMA_TEAM_ID = '123';
    delete process.env.FIGMA_TIMEOUT;
    const { getConfig } = await import('../config.js');
    const config = getConfig();
    expect(config.timeout).toBe(30000);
  });

  it('lit FIGMA_TIMEOUT depuis l\'env', async () => {
    process.env.FIGMA_PERSONAL_ACCESS_TOKEN = 'figd_test';
    process.env.FIGMA_TEAM_ID = '123';
    process.env.FIGMA_TIMEOUT = '60000';
    const { getConfig } = await import('../config.js');
    const config = getConfig();
    expect(config.timeout).toBe(60000);
  });

  it('ignore une valeur FIGMA_TIMEOUT invalide et utilise le défaut', async () => {
    process.env.FIGMA_PERSONAL_ACCESS_TOKEN = 'figd_test';
    process.env.FIGMA_TEAM_ID = '123';
    process.env.FIGMA_TIMEOUT = 'not-a-number';
    const { getConfig } = await import('../config.js');
    const config = getConfig();
    expect(config.timeout).toBe(30000);
  });

  it('utilise 2 retries par défaut si FIGMA_MAX_RETRIES absent', async () => {
    process.env.FIGMA_PERSONAL_ACCESS_TOKEN = 'figd_test';
    process.env.FIGMA_TEAM_ID = '123';
    delete process.env.FIGMA_MAX_RETRIES;
    const { getConfig } = await import('../config.js');
    const config = getConfig();
    expect(config.maxRetries).toBe(2);
  });

  it('lit FIGMA_MAX_RETRIES depuis l\'env', async () => {
    process.env.FIGMA_PERSONAL_ACCESS_TOKEN = 'figd_test';
    process.env.FIGMA_TEAM_ID = '123';
    process.env.FIGMA_MAX_RETRIES = '3';
    const { getConfig } = await import('../config.js');
    const config = getConfig();
    expect(config.maxRetries).toBe(3);
  });
});

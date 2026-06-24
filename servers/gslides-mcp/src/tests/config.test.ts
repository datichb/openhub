import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { getConfig } from '../config.js';

// SA key JSON minimal valide encodé en base64
const VALID_SA_JSON = JSON.stringify({
  type: 'service_account',
  project_id: 'test-project',
  private_key_id: 'key-id',
  private_key: '-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA0Z3VS5JJcds3xHn/ygWep4PAtXMSmGPyFgfFRFTYyCMHCMck\n-----END RSA PRIVATE KEY-----\n',
  client_email: 'test@test-project.iam.gserviceaccount.com',
  client_id: '123456789',
  auth_uri: 'https://accounts.google.com/o/oauth2/auth',
  token_uri: 'https://oauth2.googleapis.com/token',
});
const VALID_SA_KEY = Buffer.from(VALID_SA_JSON).toString('base64');

describe('getConfig()', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    process.env = { ...originalEnv };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  it('retourne la config complète quand tous les champs sont présents', () => {
    process.env.GOOGLE_SERVICE_ACCOUNT_KEY = VALID_SA_KEY;
    process.env.GOOGLE_SLIDES_TEMPLATE_ID = '1BxiMTEST';
    process.env.GOOGLE_SLIDES_TIMEOUT = '60000';
    process.env.GOOGLE_SLIDES_MAX_RETRIES = '3';

    const config = getConfig();

    expect(config.serviceAccountKey).toBe(VALID_SA_KEY);
    expect(config.templateId).toBe('1BxiMTEST');
    expect(config.timeout).toBe(60000);
    expect(config.maxRetries).toBe(3);
    expect(config.baseUrl).toBe('https://slides.googleapis.com/v1');
    expect(config.driveBaseUrl).toBe('https://www.googleapis.com/drive/v3');
  });

  it('lance une erreur si GOOGLE_SERVICE_ACCOUNT_KEY est absent', () => {
    delete process.env.GOOGLE_SERVICE_ACCOUNT_KEY;

    expect(() => getConfig()).toThrow('GOOGLE_SERVICE_ACCOUNT_KEY is required');
  });

  it('lance une erreur si GOOGLE_SERVICE_ACCOUNT_KEY n\'est pas un JSON SA valide', () => {
    process.env.GOOGLE_SERVICE_ACCOUNT_KEY = Buffer.from('{"type":"oauth_client"}').toString('base64');

    expect(() => getConfig()).toThrow('GOOGLE_SERVICE_ACCOUNT_KEY invalide');
  });

  it('lance une erreur si GOOGLE_SERVICE_ACCOUNT_KEY n\'est pas du base64 valide', () => {
    process.env.GOOGLE_SERVICE_ACCOUNT_KEY = '!!!not-base64-json!!!';

    expect(() => getConfig()).toThrow('GOOGLE_SERVICE_ACCOUNT_KEY invalide');
  });

  it('utilise 30000 ms comme timeout par défaut si GOOGLE_SLIDES_TIMEOUT est absent', () => {
    process.env.GOOGLE_SERVICE_ACCOUNT_KEY = VALID_SA_KEY;
    delete process.env.GOOGLE_SLIDES_TIMEOUT;

    const config = getConfig();
    expect(config.timeout).toBe(30000);
  });

  it('utilise 30000 ms comme timeout si GOOGLE_SLIDES_TIMEOUT est invalide', () => {
    process.env.GOOGLE_SERVICE_ACCOUNT_KEY = VALID_SA_KEY;
    process.env.GOOGLE_SLIDES_TIMEOUT = 'abc';

    const config = getConfig();
    expect(config.timeout).toBe(30000);
  });

  it('retourne templateId=undefined si GOOGLE_SLIDES_TEMPLATE_ID est absent', () => {
    process.env.GOOGLE_SERVICE_ACCOUNT_KEY = VALID_SA_KEY;
    delete process.env.GOOGLE_SLIDES_TEMPLATE_ID;

    const config = getConfig();
    expect(config.templateId).toBeUndefined();
  });

  it('utilise 2 comme maxRetries par défaut', () => {
    process.env.GOOGLE_SERVICE_ACCOUNT_KEY = VALID_SA_KEY;
    delete process.env.GOOGLE_SLIDES_MAX_RETRIES;

    const config = getConfig();
    expect(config.maxRetries).toBe(2);
  });
});

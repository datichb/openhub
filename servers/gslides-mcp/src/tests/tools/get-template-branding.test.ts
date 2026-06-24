import { describe, it, expect, vi, beforeEach } from 'vitest';
import { getTemplateBranding } from '../../tools/get-template-branding.js';
import type { GSlidesClient, BrandingResult } from '../../client.js';

// Mock minimal du GSlidesClient
function makeMockClient(overrides: Partial<GSlidesClient> = {}): GSlidesClient {
  return {
    getTemplateBranding: vi.fn(),
    getPresentation: vi.fn(),
    listPresentations: vi.fn(),
    ...overrides,
  } as unknown as GSlidesClient;
}

const MOCK_BRANDING: BrandingResult = {
  templateName: 'Acme Corp — Corporate 2024',
  presentationId: 'test-presentation-id',
  backgroundColor: '#1a1a2e',
  accentColor: '#e94560',
  textColor: '#ffffff',
  fontFamily: 'Montserrat',
  cssTheme: [
    'section {',
    '  background: #1a1a2e;',
    '  color: #ffffff;',
    "  font-family: 'Montserrat', Arial, sans-serif;",
    '}',
    'h1, h2, h3 {',
    '  color: #e94560;',
    '}',
  ].join('\n'),
};

describe('getTemplateBranding()', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('retourne le branding structuré et le frontmatter Marp pour un ID valide', async () => {
    const client = makeMockClient({
      getTemplateBranding: vi.fn().mockResolvedValue(MOCK_BRANDING),
    });

    const result = await getTemplateBranding(client, 'test-presentation-id');

    expect(result.isError).toBeFalsy();
    const parsed = JSON.parse(result.content[0].text);
    expect(parsed.templateName).toBe('Acme Corp — Corporate 2024');
    expect(parsed.backgroundColor).toBe('#1a1a2e');
    expect(parsed.accentColor).toBe('#e94560');
    expect(parsed.fontFamily).toBe('Montserrat');
    expect(parsed.marpFrontmatter).toContain('marp: true');
    expect(parsed.marpFrontmatter).toContain('style: |');
    expect(parsed.marpFrontmatter).toContain('background: #1a1a2e');
  });

  it('retourne isError=true si presentationId est absent', async () => {
    const client = makeMockClient();

    const result = await getTemplateBranding(client, '');

    expect(result.isError).toBe(true);
    expect(result.content[0].text).toContain('Argument manquant');
    expect(result.content[0].text).toContain('presentationId');
  });

  it('retourne isError=true si presentationId est une chaîne vide avec espaces', async () => {
    const client = makeMockClient();

    const result = await getTemplateBranding(client, '   ');

    expect(result.isError).toBe(true);
    expect(result.content[0].text).toContain('Argument manquant');
  });

  it('retourne isError=true et le message d\'erreur en cas de 404', async () => {
    const client = makeMockClient({
      getTemplateBranding: vi.fn().mockRejectedValue(
        new Error('ℹ️ Template introuvable (404) — vérifier l\'ID de la présentation'),
      ),
    });

    const result = await getTemplateBranding(client, 'nonexistent-id');

    expect(result.isError).toBe(true);
    expect(result.content[0].text).toContain('introuvable');
    expect(result.content[0].text).toContain('404');
  });

  it('retourne isError=true et le message d\'erreur en cas de 403', async () => {
    const client = makeMockClient({
      getTemplateBranding: vi.fn().mockRejectedValue(
        new Error('⚠️ Accès refusé (403) — partager le template avec l\'email du Service Account'),
      ),
    });

    const result = await getTemplateBranding(client, 'private-id');

    expect(result.isError).toBe(true);
    expect(result.content[0].text).toContain('403');
    expect(result.content[0].text).toContain('Service Account');
  });

  it('trim le presentationId avant de l\'envoyer au client', async () => {
    const mockFn = vi.fn().mockResolvedValue(MOCK_BRANDING);
    const client = makeMockClient({ getTemplateBranding: mockFn });

    await getTemplateBranding(client, '  test-presentation-id  ');

    expect(mockFn).toHaveBeenCalledWith('test-presentation-id');
  });
});

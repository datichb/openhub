import { describe, it, expect, vi, beforeEach } from 'vitest';
import { listPresentations } from '../../tools/list-presentations.js';
import type { GSlidesClient, DriveFile } from '../../client.js';

function makeMockClient(overrides: Partial<GSlidesClient> = {}): GSlidesClient {
  return {
    getTemplateBranding: vi.fn(),
    getPresentation: vi.fn(),
    listPresentations: vi.fn(),
    ...overrides,
  } as unknown as GSlidesClient;
}

const MOCK_FILES: DriveFile[] = [
  {
    id: 'id-acme-template',
    name: 'Acme Corp — Corporate Template 2024',
    modifiedTime: '2024-03-15T10:00:00.000Z',
  },
  {
    id: 'id-product-pitch',
    name: 'Product Pitch — Q1 2024',
    modifiedTime: '2024-01-20T08:30:00.000Z',
  },
];

describe('listPresentations()', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('retourne la liste formatée des présentations accessibles', async () => {
    const client = makeMockClient({
      listPresentations: vi.fn().mockResolvedValue(MOCK_FILES),
    });

    const result = await listPresentations(client);

    expect(result.isError).toBeFalsy();
    expect(result.content[0].text).toContain('Acme Corp — Corporate Template 2024');
    expect(result.content[0].text).toContain('id-acme-template');
    expect(result.content[0].text).toContain('Product Pitch — Q1 2024');
    expect(result.content[0].text).toContain('id-product-pitch');
    expect(result.content[0].text).toContain('get_template_branding');
  });

  it('retourne un message informatif si la liste est vide', async () => {
    const client = makeMockClient({
      listPresentations: vi.fn().mockResolvedValue([]),
    });

    const result = await listPresentations(client);

    expect(result.isError).toBeFalsy();
    expect(result.content[0].text).toContain('Aucune présentation');
    expect(result.content[0].text).toContain('Service Account');
    expect(result.content[0].text).toContain('Partager');
  });

  it('indique le nombre de présentations dans le message', async () => {
    const client = makeMockClient({
      listPresentations: vi.fn().mockResolvedValue(MOCK_FILES),
    });

    const result = await listPresentations(client);

    expect(result.content[0].text).toContain('2 présentation(s)');
  });

  it('retourne isError=true en cas d\'erreur réseau', async () => {
    const client = makeMockClient({
      listPresentations: vi.fn().mockRejectedValue(
        new Error('⚠️ API Google indisponible (timeout 30s, tentative 3/2) — vérifier la connexion réseau'),
      ),
    });

    const result = await listPresentations(client);

    expect(result.isError).toBe(true);
    expect(result.content[0].text).toContain('indisponible');
  });

  it('retourne isError=true en cas d\'erreur 401', async () => {
    const client = makeMockClient({
      listPresentations: vi.fn().mockRejectedValue(
        new Error('⚠️ Token Service Account invalide ou expiré (401)'),
      ),
    });

    const result = await listPresentations(client);

    expect(result.isError).toBe(true);
    expect(result.content[0].text).toContain('401');
  });

  it('gère les fichiers sans modifiedTime', async () => {
    const filesWithoutDate: DriveFile[] = [
      { id: 'no-date-id', name: 'Template sans date' },
    ];
    const client = makeMockClient({
      listPresentations: vi.fn().mockResolvedValue(filesWithoutDate),
    });

    const result = await listPresentations(client);

    expect(result.isError).toBeFalsy();
    expect(result.content[0].text).toContain('Template sans date');
    expect(result.content[0].text).toContain('no-date-id');
  });
});

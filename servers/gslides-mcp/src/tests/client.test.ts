import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { rgbToHex, extractBranding, buildCssTheme, classifyGSlidesError } from '../client.js';
import type { GSlidePresentation, RgbColor } from '../client.js';

// ── Tests rgbToHex ────────────────────────────────────────────────────────────

describe('rgbToHex()', () => {
  it('convertit des valeurs RGB normalisées en hexadécimal', () => {
    // 0.102*255=26.01→26=0x1a, 0.067*255=17.085→17=0x11, 0.180*255=45.9→46=0x2e
    expect(rgbToHex({ red: 0.102, green: 0.067, blue: 0.180 })).toBe('#1a112e');
  });

  it('retourne #000000 pour un objet vide (valeurs absentes = 0)', () => {
    expect(rgbToHex({})).toBe('#000000');
  });

  it('retourne #ffffff pour des valeurs à 1', () => {
    expect(rgbToHex({ red: 1, green: 1, blue: 1 })).toBe('#ffffff');
  });

  it('clamp les valeurs hors plage [0,1]', () => {
    expect(rgbToHex({ red: 1.5, green: -0.5, blue: 0.5 })).toBe('#ff0080');
  });

  it('convertit correctement #e94560 (rouge accent)', () => {
    const rgb: RgbColor = {
      red: 233 / 255,
      green: 69 / 255,
      blue: 96 / 255,
    };
    expect(rgbToHex(rgb)).toBe('#e94560');
  });
});

// ── Tests buildCssTheme ───────────────────────────────────────────────────────

describe('buildCssTheme()', () => {
  it('génère un CSS Marp valide avec les valeurs fournies', () => {
    const css = buildCssTheme({
      backgroundColor: '#1a1a2e',
      textColor: '#ffffff',
      accentColor: '#e94560',
      fontFamily: 'Montserrat',
    });

    expect(css).toContain("background: #1a1a2e");
    expect(css).toContain("color: #ffffff");
    expect(css).toContain("color: #e94560");
    expect(css).toContain("font-family: 'Montserrat'");
    expect(css).toContain("section.lead {");
  });

  it('inclut les règles h1, h2, h3 avec la couleur d\'accent', () => {
    const css = buildCssTheme({
      backgroundColor: '#fff',
      textColor: '#333',
      accentColor: '#0066cc',
      fontFamily: 'Arial',
    });
    expect(css).toContain("h1, h2, h3 {");
    expect(css).toContain("color: #0066cc");
  });
});

// ── Tests extractBranding ─────────────────────────────────────────────────────

describe('extractBranding()', () => {
  it('extrait correctement le branding d\'une présentation avec master complet', () => {
    const presentation: GSlidePresentation = {
      presentationId: 'test-id-123',
      title: 'Test Template',
      masters: [
        {
          masterProperties: { displayName: 'Acme Corp Theme' },
          pageProperties: {
            pageBackgroundFill: {
              solidFill: {
                color: { rgbColor: { red: 0.102, green: 0.067, blue: 0.180 } },
              },
            },
          },
          pageElements: [
            {
              shape: {
                placeholder: { type: 'TITLE' },
                text: {
                  textElements: [
                    {
                      paragraph: {
                        elements: [
                          {
                            textRun: {
                              content: 'Titre',
                              style: {
                                fontFamily: 'Montserrat',
                                foregroundColor: {
                                  rgbColor: { red: 233 / 255, green: 69 / 255, blue: 96 / 255 },
                                },
                              },
                            },
                          },
                        ],
                      },
                    },
                  ],
                },
              },
            },
            {
              shape: {
                placeholder: { type: 'BODY' },
                text: {
                  textElements: [
                    {
                      paragraph: {
                        elements: [
                          {
                            textRun: {
                              content: 'Corps',
                              style: {
                                fontFamily: 'Montserrat',
                                foregroundColor: {
                                  rgbColor: { red: 1, green: 1, blue: 1 },
                                },
                              },
                            },
                          },
                        ],
                      },
                    },
                  ],
                },
              },
            },
          ],
        },
      ],
    };

    const branding = extractBranding(presentation);

    expect(branding.presentationId).toBe('test-id-123');
    expect(branding.templateName).toBe('Acme Corp Theme');
    expect(branding.backgroundColor).toBe('#1a112e');
    expect(branding.accentColor).toBe('#e94560');
    expect(branding.textColor).toBe('#ffffff');
    expect(branding.fontFamily).toBe('Montserrat');
    expect(branding.cssTheme).toContain('background: #1a112e');
    expect(branding.cssTheme).toContain("font-family: 'Montserrat'");
  });

  it('retourne des valeurs par défaut si masters est vide', () => {
    const presentation: GSlidePresentation = {
      presentationId: 'empty-id',
      title: 'Empty Template',
      masters: [],
    };

    const branding = extractBranding(presentation);

    expect(branding.backgroundColor).toBe('#ffffff');
    expect(branding.textColor).toBe('#000000');
    expect(branding.accentColor).toBe('#0066cc');
    expect(branding.fontFamily).toBe('Arial');
    expect(branding.templateName).toBe('Empty Template');
  });

  it('retourne des valeurs par défaut si masters est absent', () => {
    const presentation: GSlidePresentation = {
      presentationId: 'no-masters',
      title: 'No Masters',
    };

    const branding = extractBranding(presentation);

    expect(branding.backgroundColor).toBe('#ffffff');
    expect(branding.cssTheme).toBeDefined();
  });
});

// ── Tests classifyGSlidesError ────────────────────────────────────────────────
// axios.isAxiosError() vérifie error.isAxiosError === true directement sur l'objet.
// Pas besoin de spy : on passe des objets avec la propriété isAxiosError.

describe('classifyGSlidesError()', () => {
  it('retourne un message clair pour une erreur 401', () => {
    const axiosError = {
      isAxiosError: true,
      response: { status: 401, data: {} },
      config: { timeout: 30000 },
      code: undefined,
      message: 'Unauthorized',
    };

    const msg = classifyGSlidesError(axiosError, 1, 2);
    expect(msg).toContain('401');
    expect(msg).toContain('oc gslides status');
  });

  it('retourne un message clair pour une erreur 403', () => {
    const axiosError = {
      isAxiosError: true,
      response: { status: 403, data: {} },
      config: {},
      code: undefined,
      message: 'Forbidden',
    };

    const msg = classifyGSlidesError(axiosError, 1, 2);
    expect(msg).toContain('403');
    expect(msg).toContain('Service Account');
  });

  it('retourne un message clair pour une erreur 404', () => {
    const axiosError = {
      isAxiosError: true,
      response: { status: 404, data: {} },
      config: {},
      code: undefined,
      message: 'Not Found',
    };

    const msg = classifyGSlidesError(axiosError, 1, 2);
    expect(msg).toContain('404');
    expect(msg).toContain('introuvable');
  });

  it('retourne la string brute pour une erreur non-axios', () => {
    const msg = classifyGSlidesError(new Error('unexpected error'), 1, 2);
    expect(msg).toContain('unexpected error');
  });
});

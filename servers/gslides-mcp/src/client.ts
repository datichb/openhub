/**
 * Google Slides MCP Server
 * Client — authentification Service Account + appels API Google Slides & Drive
 */

import axios, { type AxiosInstance, type AxiosError } from 'axios';
import { GoogleAuth } from 'google-auth-library';
import type { GSlidesConfig } from './config.js';

// ── Types API Google Slides ──────────────────────────────────────────────────

export interface RgbColor {
  red?: number;
  green?: number;
  blue?: number;
}

export interface OpaqueColor {
  rgbColor?: RgbColor;
  themeColor?: string;
}

export interface SolidFill {
  color?: OpaqueColor;
  alpha?: number;
}

export interface PageBackgroundFill {
  solidFill?: SolidFill;
}

export interface PageProperties {
  pageBackgroundFill?: PageBackgroundFill;
}

export interface TextStyle {
  fontFamily?: string;
  foregroundColor?: OpaqueColor;
  fontSize?: { magnitude?: number; unit?: string };
}

export interface TextRun {
  content?: string;
  style?: TextStyle;
}

export interface ParagraphElement {
  textRun?: TextRun;
}

export interface Paragraph {
  elements?: ParagraphElement[];
}

export interface TextContent {
  textElements?: Array<{ paragraph?: Paragraph }>;
}

export interface PageElement {
  shape?: {
    text?: TextContent;
    placeholder?: { type?: string };
  };
}

export interface Master {
  pageProperties?: PageProperties;
  masterProperties?: { displayName?: string };
  pageElements?: PageElement[];
}

export interface GSlidePresentation {
  presentationId: string;
  title: string;
  masters?: Master[];
}

// ── Type de sortie branding ──────────────────────────────────────────────────

export interface BrandingResult {
  templateName: string;
  presentationId: string;
  backgroundColor: string;
  accentColor: string;
  textColor: string;
  fontFamily: string;
  cssTheme: string;
}

export interface DriveFile {
  id: string;
  name: string;
  modifiedTime?: string;
}

// ── Utilitaires couleur ──────────────────────────────────────────────────────

/**
 * Convertit un RgbColor Google Slides (valeurs 0–1) en hexadécimal CSS (#rrggbb).
 * Les composantes absentes valent 0 (noir par défaut).
 */
export function rgbToHex(rgb: RgbColor): string {
  const toHex = (v: number): string =>
    Math.round(Math.max(0, Math.min(1, v)) * 255)
      .toString(16)
      .padStart(2, '0');
  return `#${toHex(rgb.red ?? 0)}${toHex(rgb.green ?? 0)}${toHex(rgb.blue ?? 0)}`;
}

/**
 * Extrait une couleur hex depuis une OpaqueColor.
 * Si la couleur est un themeColor (pas de valeur RGB concrète), retourne null.
 */
function opaqueColorToHex(oc: OpaqueColor | undefined): string | null {
  if (!oc?.rgbColor) return null;
  return rgbToHex(oc.rgbColor);
}

// ── Extraction du branding ───────────────────────────────────────────────────

/**
 * Extrait les informations de branding depuis la structure d'une présentation Google Slides.
 * Analyse le premier master (master[0]) pour récupérer couleur de fond, police, couleur texte.
 * L'accentColor est déduite comme complément de la couleur de fond.
 */
export function extractBranding(presentation: GSlidePresentation): BrandingResult {
  const master = presentation.masters?.[0];
  const id = presentation.presentationId;

  // Couleur de fond depuis les propriétés du master
  const bgFill = master?.pageProperties?.pageBackgroundFill?.solidFill?.color;
  const backgroundColor = opaqueColorToHex(bgFill) ?? '#ffffff';

  // Police et couleur texte depuis les placeholders titre/corps du master
  let fontFamily = 'Arial';
  let textColor = '#000000';
  let accentColor = '#0066cc';

  if (master?.pageElements) {
    for (const element of master.pageElements) {
      const placeholderType = element.shape?.placeholder?.type;
      const textElements = element.shape?.text?.textElements ?? [];

      for (const textEl of textElements) {
        const paragraphElements = textEl.paragraph?.elements ?? [];
        for (const pe of paragraphElements) {
          const style = pe.textRun?.style;
          if (!style) continue;

          if (style.fontFamily) {
            fontFamily = style.fontFamily;
          }

          const fgHex = opaqueColorToHex(style.foregroundColor);
          if (fgHex) {
            // Le placeholder TITLE donne la couleur d'accent, BODY donne la couleur texte
            if (placeholderType === 'TITLE' || placeholderType === 'CENTERED_TITLE') {
              accentColor = fgHex;
            } else {
              textColor = fgHex;
            }
          }
        }
      }
    }
  }

  const templateName =
    master?.masterProperties?.displayName ?? presentation.title ?? 'Template sans nom';

  // Génération du CSS Marp
  const cssTheme = buildCssTheme({ backgroundColor, textColor, accentColor, fontFamily });

  return { templateName, presentationId: id, backgroundColor, accentColor, textColor, fontFamily, cssTheme };
}

/**
 * Génère un bloc CSS Marp à partir des couleurs et polices extraites.
 */
export function buildCssTheme(params: {
  backgroundColor: string;
  textColor: string;
  accentColor: string;
  fontFamily: string;
}): string {
  const { backgroundColor, textColor, accentColor, fontFamily } = params;
  return [
    `section {`,
    `  background: ${backgroundColor};`,
    `  color: ${textColor};`,
    `  font-family: '${fontFamily}', Arial, sans-serif;`,
    `}`,
    `h1, h2, h3 {`,
    `  color: ${accentColor};`,
    `}`,
    `a {`,
    `  color: ${accentColor};`,
    `}`,
    `section.lead {`,
    `  background: ${accentColor};`,
    `  color: ${backgroundColor};`,
    `}`,
    `section.lead h1 {`,
    `  color: ${backgroundColor};`,
    `}`,
  ].join('\n');
}

// ── Gestion des erreurs ──────────────────────────────────────────────────────

/** Codes axios retriables */
const RETRYABLE_AXIOS_CODES = new Set(['ECONNABORTED', 'ETIMEDOUT', 'ERR_NETWORK', 'ECONNRESET']);
/** Statuts HTTP retriables */
const RETRYABLE_HTTP_STATUSES = new Set([429, 503, 504]);

export function classifyGSlidesError(error: unknown, attempt: number, maxRetries: number): string {
  if (!axios.isAxiosError(error)) return String(error);

  const axiosErr = error as AxiosError;
  const status = axiosErr.response?.status;
  const timeoutMs = (axiosErr.config?.timeout ?? 30000) / 1000;

  if (axiosErr.code && RETRYABLE_AXIOS_CODES.has(axiosErr.code)) {
    return `⚠️ API Google indisponible (timeout ${timeoutMs}s, tentative ${attempt}/${maxRetries}) — vérifier la connexion réseau`;
  }
  if (status === 401) return `⚠️ Token Service Account invalide ou expiré (401) — vérifier : oc gslides status`;
  if (status === 403) return `⚠️ Accès refusé (403) — partager le template avec l'email du Service Account (rôle Lecteur)`;
  if (status === 404) return `ℹ️ Template introuvable (404) — vérifier l'ID de la présentation`;
  if (status === 429) return `⚠️ Limite de requêtes Google atteinte (429) — réessayer dans quelques secondes`;
  if (status === 503 || status === 504) return `⚠️ API Google temporairement indisponible (${status}) — réessayer plus tard`;

  const errMsg = (axiosErr.response?.data as Record<string, unknown>)?.error ?? axiosErr.message;
  return `Google Slides API erreur ${status ?? 'réseau'} : ${String(errMsg)}`;
}

async function withRetry<T>(
  fn: () => Promise<T>,
  maxRetries: number,
  baseDelayMs = 500,
): Promise<T> {
  let lastError: unknown;

  for (let attempt = 1; attempt <= maxRetries + 1; attempt++) {
    try {
      return await fn();
    } catch (error) {
      lastError = error;

      if (axios.isAxiosError(error)) {
        const status = error.response?.status;
        const code = error.code ?? '';
        const isRetryable =
          RETRYABLE_AXIOS_CODES.has(code) ||
          (status !== undefined && RETRYABLE_HTTP_STATUSES.has(status));

        if (isRetryable && attempt <= maxRetries) {
          const delay = baseDelayMs * Math.pow(2, attempt - 1);
          await new Promise(resolve => setTimeout(resolve, delay));
          continue;
        }
      }
      break;
    }
  }

  throw lastError;
}

// ── Client principal ─────────────────────────────────────────────────────────

export class GSlidesClient {
  private axiosInstance: AxiosInstance;
  private auth: GoogleAuth;
  private maxRetries: number;
  private config: GSlidesConfig;

  constructor(config: GSlidesConfig) {
    this.config = config;
    this.maxRetries = config.maxRetries;

    // Décoder le SA key depuis base64
    const saKeyJson = Buffer.from(config.serviceAccountKey, 'base64').toString('utf8');
    const saKey = JSON.parse(saKeyJson);

    this.auth = new GoogleAuth({
      credentials: saKey,
      scopes: [
        'https://www.googleapis.com/auth/presentations.readonly',
        'https://www.googleapis.com/auth/drive.readonly',
      ],
    });

    this.axiosInstance = axios.create({ timeout: config.timeout });
  }

  /**
   * Récupère un access token OAuth2 depuis le Service Account.
   */
  private async getAccessToken(): Promise<string> {
    const client = await this.auth.getClient();
    const tokenResponse = await client.getAccessToken();
    if (!tokenResponse.token) {
      throw new Error('Impossible d\'obtenir un access token Google — vérifier la clé Service Account');
    }
    return tokenResponse.token;
  }

  /**
   * Récupère la structure complète d'une présentation Google Slides.
   */
  async getPresentation(presentationId: string): Promise<GSlidePresentation> {
    try {
      const token = await this.getAccessToken();
      const { data } = await withRetry(
        () => this.axiosInstance.get(
          `${this.config.baseUrl}/presentations/${presentationId}`,
          { headers: { Authorization: `Bearer ${token}` } },
        ),
        this.maxRetries,
      );
      return data as GSlidePresentation;
    } catch (error) {
      throw new Error(classifyGSlidesError(error, this.maxRetries + 1, this.maxRetries));
    }
  }

  /**
   * Extrait le branding d'une présentation à partir de son ID.
   */
  async getTemplateBranding(presentationId: string): Promise<BrandingResult> {
    const presentation = await this.getPresentation(presentationId);
    return extractBranding(presentation);
  }

  /**
   * Liste les présentations Google Slides accessibles par le Service Account via Drive API.
   */
  async listPresentations(): Promise<DriveFile[]> {
    try {
      const token = await this.getAccessToken();
      const { data } = await withRetry(
        () => this.axiosInstance.get(
          `${this.config.driveBaseUrl}/files`,
          {
            headers: { Authorization: `Bearer ${token}` },
            params: {
              q: "mimeType='application/vnd.google-apps.presentation'",
              fields: 'files(id,name,modifiedTime)',
              orderBy: 'modifiedTime desc',
              pageSize: 50,
            },
          },
        ),
        this.maxRetries,
      );
      return (data.files ?? []) as DriveFile[];
    } catch (error) {
      throw new Error(classifyGSlidesError(error, this.maxRetries + 1, this.maxRetries));
    }
  }
}

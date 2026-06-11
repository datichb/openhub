/**
 * Client pour l'API Figma
 */

import axios, { type AxiosInstance, type AxiosError } from 'axios';
import type { FigmaConfig } from './config.js';

export interface FigmaFile {
  key: string;
  name: string;
  thumbnail_url?: string;
  last_modified: string;
}

export interface FigmaProject {
  id: string;
  name: string;
}

/** Propriétés complètes d'un nœud Figma (layout, visuel, géométrie) */
export interface FigmaNode {
  id: string;
  name: string;
  type: string;
  children?: FigmaNode[];
  // Layout
  layoutMode?: 'NONE' | 'HORIZONTAL' | 'VERTICAL';
  primaryAxisAlignItems?: string;
  counterAxisAlignItems?: string;
  primaryAxisSizingMode?: string;
  counterAxisSizingMode?: string;
  itemSpacing?: number;
  paddingLeft?: number;
  paddingRight?: number;
  paddingTop?: number;
  paddingBottom?: number;
  // Géométrie
  absoluteBoundingBox?: { x: number; y: number; width: number; height: number };
  size?: { x: number; y: number };
  // Visuel
  fills?: Array<{ type: string; color?: { r: number; g: number; b: number; a: number }; opacity?: number }>;
  strokes?: Array<{ type: string; color?: { r: number; g: number; b: number; a: number } }>;
  opacity?: number;
  visible?: boolean;
  // Composant
  componentPropertyDefinitions?: Record<string, {
    type: string;
    defaultValue: unknown;
    variantOptions?: string[];
  }>;
  componentProperties?: Record<string, { type: string; value: unknown }>;
  // Texte
  characters?: string;
  style?: Record<string, unknown>;
}

export interface FigmaFileResponse {
  name: string;
  lastModified: string;
  thumbnailUrl?: string;
  document: FigmaNode;
}

/** Codes d'erreur axios considérés comme des erreurs réseau/timeout retriables */
const RETRYABLE_AXIOS_CODES = new Set(['ECONNABORTED', 'ETIMEDOUT', 'ERR_NETWORK', 'ECONNRESET']);

/** Statuts HTTP retriables (rate-limit, service indisponible) */
const RETRYABLE_HTTP_STATUSES = new Set([429, 503, 504]);

/**
 * Classifie une erreur axios en catégorie lisible par l'agent.
 */
export function classifyFigmaError(error: unknown, attempt: number, maxRetries: number): string {
  if (!axios.isAxiosError(error)) return String(error);

  const axiosErr = error as AxiosError;
  const status = axiosErr.response?.status;
  const timeoutMs = (axiosErr.config?.timeout ?? 30000) / 1000;

  if (axiosErr.code && RETRYABLE_AXIOS_CODES.has(axiosErr.code)) {
    return `⚠️ Figma indisponible (timeout ${timeoutMs}s, tentative ${attempt}/${maxRetries}) — vérifier la connexion réseau`;
  }

  if (status === 401) {
    return `⚠️ Token Figma invalide (401) — vérifier : oc figma status`;
  }
  if (status === 403) {
    return `⚠️ Accès refusé Figma (403) — vérifier les scopes du token : oc figma status`;
  }
  if (status === 404) {
    return `ℹ️ Ressource Figma introuvable (404) — fichier ou team inexistant`;
  }
  if (status === 429) {
    return `⚠️ Limite de requêtes Figma atteinte (429) — réessayer dans quelques secondes`;
  }
  if (status === 503 || status === 504) {
    return `⚠️ API Figma temporairement indisponible (${status}) — réessayer plus tard`;
  }

  const errMsg = (axiosErr.response?.data as any)?.err || axiosErr.message;
  return `Figma API erreur ${status ?? 'réseau'} : ${errMsg}`;
}

/**
 * Exécute une fonction async avec retry et backoff exponentiel.
 * Retriable : timeout, erreur réseau, 429, 503, 504.
 * Non retriable : 401, 403, 404, autres erreurs métier.
 */
async function withRetry<T>(
  fn: () => Promise<T>,
  maxRetries: number,
  baseDelayMs: number = 1000
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

export class FigmaClient {
  private client: AxiosInstance;
  private teamId: string;
  private maxRetries: number;

  constructor(config: FigmaConfig) {
    this.client = axios.create({
      baseURL: config.baseUrl,
      headers: {
        'X-Figma-Token': config.token,
      },
      timeout: config.timeout,
    });
    this.teamId = config.teamId;
    this.maxRetries = config.maxRetries;
  }

  /**
   * Recherche des fichiers Figma par nom
   */
  async searchFiles(query: string): Promise<FigmaFile[]> {
    try {
      const { data: projectsData } = await withRetry(
        () => this.client.get(`/teams/${this.teamId}/projects`),
        this.maxRetries
      );

      const allFiles: FigmaFile[] = [];

      for (const project of projectsData.projects) {
        try {
          const { data: filesData } = await withRetry(
            () => this.client.get(`/projects/${project.id}/files`),
            this.maxRetries
          );

          const matchingFiles = filesData.files.filter((file: FigmaFile) =>
            file.name.toLowerCase().includes(query.toLowerCase())
          );

          allFiles.push(...matchingFiles);
        } catch (error) {
          console.error(`Error fetching files for project ${project.id}:`, error);
        }
      }

      return allFiles;
    } catch (error) {
      throw new Error(classifyFigmaError(error, this.maxRetries + 1, this.maxRetries));
    }
  }

  /**
   * Récupère la structure d'un fichier Figma
   */
  async getFile(fileId: string): Promise<FigmaFileResponse> {
    try {
      const { data } = await withRetry(
        () => this.client.get(`/files/${fileId}`),
        this.maxRetries
      );

      return {
        name: data.name,
        lastModified: data.lastModified,
        thumbnailUrl: data.thumbnailUrl,
        document: data.document,
      };
    } catch (error) {
      throw new Error(classifyFigmaError(error, this.maxRetries + 1, this.maxRetries));
    }
  }

  /**
   * Récupère les détails complets d'un nœud spécifique par son ID.
   * Utilise GET /files/{fileId}/nodes?ids={nodeId}
   */
  async getNode(fileId: string, nodeId: string): Promise<FigmaNode> {
    try {
      const { data } = await withRetry(
        () => this.client.get(`/files/${fileId}/nodes`, {
          params: { ids: nodeId },
        }),
        this.maxRetries
      );

      const nodeData = data.nodes?.[nodeId];
      if (!nodeData) {
        throw new Error(`ℹ️ Nœud Figma introuvable (ID: ${nodeId}) dans le fichier ${fileId}`);
      }

      return nodeData.document as FigmaNode;
    } catch (error) {
      if (error instanceof Error && error.message.startsWith('ℹ️')) throw error;
      throw new Error(classifyFigmaError(error, this.maxRetries + 1, this.maxRetries));
    }
  }

  /**
   * Extrait les frames d'un fichier
   */
  extractFrames(node: FigmaNode): FigmaNode[] {
    const frames: FigmaNode[] = [];

    const traverse = (n: FigmaNode) => {
      if (n.type === 'FRAME' || n.type === 'COMPONENT' || n.type === 'COMPONENT_SET') {
        frames.push(n);
      }
      if (n.children) {
        n.children.forEach(traverse);
      }
    };

    traverse(node);
    return frames;
  }

  /**
   * Compte les composants d'un fichier
   */
  countComponents(node: FigmaNode): number {
    let count = 0;

    const traverse = (n: FigmaNode) => {
      if (n.type === 'COMPONENT' || n.type === 'COMPONENT_SET') {
        count++;
      }
      if (n.children) {
        n.children.forEach(traverse);
      }
    };

    traverse(node);
    return count;
  }

  /**
   * Génère l'URL Figma vers un fichier ou un nœud spécifique
   */
  generateFileUrl(fileId: string, nodeId?: string): string {
    const baseUrl = `https://www.figma.com/file/${fileId}`;
    return nodeId ? `${baseUrl}?node-id=${nodeId}` : baseUrl;
  }

  /**
   * Extrait les design tokens (Figma Variables) depuis un fichier
   */
  async getDesignTokens(fileId: string): Promise<{
    colors: Array<{ name: string; value: string; type: 'color' }>;
    text: Array<{ name: string; fontSize: number; fontFamily: string; fontWeight: number }>;
    spacing: Array<{ name: string; value: number }>;
    effects: Array<{ name: string; type: string; radius?: number; offset?: { x: number; y: number } }>;
  }> {
    try {
      const { data } = await withRetry(
        () => this.client.get(`/files/${fileId}/variables/local`),
        this.maxRetries
      );

      const colors: Array<{ name: string; value: string; type: 'color' }> = [];
      const text: Array<{ name: string; fontSize: number; fontFamily: string; fontWeight: number }> = [];
      const spacing: Array<{ name: string; value: number }> = [];
      const effects: Array<{ name: string; type: string; radius?: number; offset?: { x: number; y: number } }> = [];

      if (data.meta && data.meta.variableCollections) {
        for (const collectionId of Object.keys(data.meta.variableCollections)) {
          const collection = data.meta.variableCollections[collectionId];

          for (const varId of collection.variableIds || []) {
            const variable = data.meta.variables?.[varId];
            if (!variable) continue;

            const varName = variable.name;
            const varType = variable.resolvedType;

            const firstModeId = collection.modes?.[0]?.modeId;
            if (!firstModeId) continue;

            const varValue = variable.valuesByMode?.[firstModeId];
            if (varValue === undefined) continue;

            if (varType === 'COLOR' && typeof varValue === 'object' && varValue !== null && 'r' in (varValue as object)) {
              const { r, g, b, a } = varValue as { r: number; g: number; b: number; a: number };
              const hex = this.rgbaToHex(r, g, b, a);
              colors.push({ name: varName, value: hex, type: 'color' });
            } else if (varType === 'FLOAT') {
              if (/space|spacing|gap|margin|padding/i.test(varName)) {
                spacing.push({ name: varName, value: varValue });
              }
            }
          }
        }
      }

      const { data: stylesData } = await withRetry(
        () => this.client.get(`/files/${fileId}/styles`),
        this.maxRetries
      );

      if (stylesData.meta && stylesData.meta.styles) {
        for (const style of Object.values(stylesData.meta.styles) as any[]) {
          if (style.style_type === 'TEXT') {
            text.push({
              name: style.name,
              fontSize: style.fontSize || 16,
              fontFamily: style.fontFamily || 'Sans-serif',
              fontWeight: style.fontWeight || 400,
            });
          } else if (style.style_type === 'EFFECT') {
            effects.push({
              name: style.name,
              type: style.type || 'UNKNOWN',
              radius: style.radius,
              offset: style.offset,
            });
          }
        }
      }

      return { colors, text, spacing, effects };
    } catch (error) {
      if (axios.isAxiosError(error)) {
        if (error.response?.status === 404 || error.response?.status === 403) {
          return { colors: [], text: [], spacing: [], effects: [] };
        }
      }
      throw new Error(classifyFigmaError(error, this.maxRetries + 1, this.maxRetries));
    }
  }

  /**
   * Convertit une couleur RGBA Figma en hex
   */
  private rgbaToHex(r: number, g: number, b: number, a: number = 1): string {
    const toHex = (n: number) => {
      const hex = Math.round(n * 255).toString(16);
      return hex.length === 1 ? '0' + hex : hex;
    };

    const hex = `#${toHex(r)}${toHex(g)}${toHex(b)}`;
    return a < 1 ? `${hex}${toHex(a)}` : hex;
  }
}


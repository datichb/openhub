/**
 * Client pour l'API Figma
 */

import axios, { AxiosInstance } from 'axios';
import { FigmaConfig } from './config.js';

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

export interface FigmaNode {
  id: string;
  name: string;
  type: string;
  children?: FigmaNode[];
}

export interface FigmaFileResponse {
  name: string;
  lastModified: string;
  thumbnailUrl?: string;
  document: FigmaNode;
}

export class FigmaClient {
  private client: AxiosInstance;
  private teamId: string;

  constructor(config: FigmaConfig) {
    this.client = axios.create({
      baseURL: config.baseUrl,
      headers: {
        'X-Figma-Token': config.token,
      },
      timeout: 10000,
    });
    this.teamId = config.teamId;
  }

  /**
   * Recherche des fichiers Figma par nom
   */
  async searchFiles(query: string): Promise<FigmaFile[]> {
    try {
      // Récupérer tous les projets de la team
      const { data: projectsData } = await this.client.get(
        `/teams/${this.teamId}/projects`
      );

      const allFiles: FigmaFile[] = [];

      // Pour chaque projet, récupérer les fichiers
      for (const project of projectsData.projects) {
        try {
          const { data: filesData } = await this.client.get(
            `/projects/${project.id}/files`
          );

          // Filtrer par nom
          const matchingFiles = filesData.files.filter((file: FigmaFile) =>
            file.name.toLowerCase().includes(query.toLowerCase())
          );

          allFiles.push(...matchingFiles);
        } catch (error) {
          // Ignorer les erreurs de projet individuel
          console.error(`Error fetching files for project ${project.id}:`, error);
        }
      }

      return allFiles;
    } catch (error) {
      if (axios.isAxiosError(error)) {
        throw new Error(
          `Figma API error: ${error.response?.status} - ${error.response?.data?.err || error.message}`
        );
      }
      throw error;
    }
  }

  /**
   * Récupère la structure d'un fichier Figma
   */
  async getFile(fileId: string): Promise<FigmaFileResponse> {
    try {
      const { data } = await this.client.get(`/files/${fileId}`);

      return {
        name: data.name,
        lastModified: data.lastModified,
        thumbnailUrl: data.thumbnailUrl,
        document: data.document,
      };
    } catch (error) {
      if (axios.isAxiosError(error)) {
        throw new Error(
          `Figma API error: ${error.response?.status} - ${error.response?.data?.err || error.message}`
        );
      }
      throw error;
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
   * Génère l'URL Figma vers un fichier
   */
  generateFileUrl(fileId: string, nodeId?: string): string {
    const baseUrl = `https://www.figma.com/file/${fileId}`;
    return nodeId ? `${baseUrl}?node-id=${nodeId}` : baseUrl;
  }
}

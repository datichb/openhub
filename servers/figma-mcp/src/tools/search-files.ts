/**
 * Tool: search_figma_files
 * Recherche de fichiers Figma par nom
 */

import { FigmaClient } from '../client.js';

export const searchFilesTool = {
  name: 'search_figma_files',
  description: 'Search for Figma files by name. Returns a list of matching files with their IDs and URLs.',
  inputSchema: {
    type: 'object',
    properties: {
      query: {
        type: 'string',
        description: 'Search query (file name or keyword)',
      },
    },
    required: ['query'],
  },
};

export async function searchFiles(
  client: FigmaClient,
  query: string
): Promise<{ content: Array<{ type: string; text: string }> }> {
  try {
    const files = await client.searchFiles(query);

    if (files.length === 0) {
      return {
        content: [
          {
            type: 'text',
            text: `Aucun fichier Figma trouvé pour la recherche : "${query}"`,
          },
        ],
      };
    }

    // Formater les résultats
    const results = files.map((file) => {
      const url = client.generateFileUrl(file.key);
      return `- **${file.name}**\n  ID: ${file.key}\n  URL: ${url}\n  Dernière modification: ${new Date(file.last_modified).toLocaleDateString('fr-FR')}`;
    });

    return {
      content: [
        {
          type: 'text',
          text: `## Fichiers Figma trouvés (${files.length})\n\n${results.join('\n\n')}`,
        },
      ],
    };
  } catch (error) {
    return {
      content: [
        {
          type: 'text',
          text: `Erreur lors de la recherche Figma : ${error instanceof Error ? error.message : 'Unknown error'}`,
        },
      ],
    };
  }
}

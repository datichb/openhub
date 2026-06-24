/**
 * Tool : list_presentations
 * Liste les présentations Google Slides accessibles par le Service Account.
 */

import type { Tool } from '@modelcontextprotocol/sdk/types.js';
import type { GSlidesClient, DriveFile } from '../client.js';

export const listPresentationsTool: Tool = {
  name: 'list_presentations',
  description:
    'Liste les présentations Google Slides accessibles par le Service Account configuré (via Google Drive API). ' +
    'Utile pour retrouver l\'ID d\'un template. ' +
    'Note : seules les présentations partagées avec ou appartenant au Service Account sont visibles.',
  inputSchema: {
    type: 'object',
    properties: {},
    required: [],
  },
};

export async function listPresentations(
  client: GSlidesClient,
): Promise<{ content: Array<{ type: 'text'; text: string }>; isError?: boolean }> {
  try {
    const files: DriveFile[] = await client.listPresentations();

    if (files.length === 0) {
      return {
        content: [
          {
            type: 'text',
            text:
              'Aucune présentation Google Slides accessible par ce Service Account.\n\n' +
              'Pour rendre un template accessible :\n' +
              '1. Ouvrir le template dans Google Slides\n' +
              '2. Cliquer sur "Partager"\n' +
              '3. Ajouter l\'email du Service Account avec le rôle "Lecteur"\n\n' +
              'L\'email du Service Account se trouve dans votre fichier SA key JSON (champ "client_email").',
          },
        ],
      };
    }

    const formatted = files
      .map((f, i) => {
        const modified = f.modifiedTime
          ? ` (modifié le ${new Date(f.modifiedTime).toLocaleDateString('fr-FR')})`
          : '';
        return `${i + 1}. ${f.name}${modified}\n   ID : ${f.id}`;
      })
      .join('\n\n');

    return {
      content: [
        {
          type: 'text',
          text:
            `${files.length} présentation(s) accessible(s) :\n\n${formatted}\n\n` +
            `Pour extraire le branding d'un template :\n` +
            `  get_template_branding({ presentationId: "<ID>" })`,
        },
      ],
    };
  } catch (error) {
    return {
      content: [
        {
          type: 'text',
          text: `Erreur lors de la liste des présentations : ${error instanceof Error ? error.message : String(error)}`,
        },
      ],
      isError: true,
    };
  }
}

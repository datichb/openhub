/**
 * Tool : get_template_branding
 * Extrait les couleurs, polices et CSS de branding depuis un template Google Slides.
 */

import type { Tool } from '@modelcontextprotocol/sdk/types.js';
import type { GSlidesClient, BrandingResult } from '../client.js';

export const getTemplateBrandingTool: Tool = {
  name: 'get_template_branding',
  description:
    'Extrait le branding (couleurs, polices, CSS Marp) depuis un template Google Slides. ' +
    'Le template doit être partagé en lecture avec le Service Account configuré. ' +
    'Retourne un objet avec backgroundColor, accentColor, textColor, fontFamily et cssTheme prêt à injecter dans un frontmatter Marp.',
  inputSchema: {
    type: 'object',
    properties: {
      presentationId: {
        type: 'string',
        description:
          'ID de la présentation Google Slides (extrait de l\'URL : ' +
          'https://docs.google.com/presentation/d/{presentationId}/edit)',
      },
    },
    required: ['presentationId'],
  },
};

export async function getTemplateBranding(
  client: GSlidesClient,
  presentationId: string,
): Promise<{ content: Array<{ type: 'text'; text: string }>; isError?: boolean }> {
  if (!presentationId || typeof presentationId !== 'string' || presentationId.trim() === '') {
    return {
      content: [
        {
          type: 'text',
          text: 'Argument manquant : presentationId (string) est requis.\n' +
            'Usage : get_template_branding({ presentationId: "1BxiM..." })\n' +
            'L\'ID se trouve dans l\'URL Google Slides : .../presentation/d/{ID}/edit',
        },
      ],
      isError: true,
    };
  }

  try {
    const branding: BrandingResult = await client.getTemplateBranding(presentationId.trim());

    const output = {
      templateName: branding.templateName,
      presentationId: branding.presentationId,
      backgroundColor: branding.backgroundColor,
      accentColor: branding.accentColor,
      textColor: branding.textColor,
      fontFamily: branding.fontFamily,
      cssTheme: branding.cssTheme,
      marpFrontmatter: formatMarpFrontmatter(branding.cssTheme),
    };

    return {
      content: [
        {
          type: 'text',
          text: JSON.stringify(output, null, 2),
        },
      ],
    };
  } catch (error) {
    return {
      content: [
        {
          type: 'text',
          text: `Erreur lors de l'extraction du branding : ${error instanceof Error ? error.message : String(error)}`,
        },
      ],
      isError: true,
    };
  }
}

/**
 * Formate le bloc cssTheme sous forme de frontmatter Marp prêt à copier-coller.
 */
function formatMarpFrontmatter(cssTheme: string): string {
  const indented = cssTheme
    .split('\n')
    .map(line => `  ${line}`)
    .join('\n');
  return `---\nmarp: true\npaginate: true\nstyle: |\n${indented}\n---`;
}

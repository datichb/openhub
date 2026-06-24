#!/usr/bin/env node

/**
 * Google Slides MCP Server
 * Entry point — serveur MCP pour l'extraction de branding depuis Google Slides
 */

import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from '@modelcontextprotocol/sdk/types.js';

import { getConfig } from './config.js';
import { GSlidesClient } from './client.js';
import { getTemplateBrandingTool, getTemplateBranding } from './tools/get-template-branding.js';
import { listPresentationsTool, listPresentations } from './tools/list-presentations.js';

// Initialisation
function initClient(): GSlidesClient {
  try {
    const config = getConfig();
    return new GSlidesClient(config);
  } catch (error) {
    console.error('Failed to initialize Google Slides MCP Server:', error);
    process.exit(1);
  }
}

const gSlidesClient = initClient();

// Créer le serveur MCP
const server = new Server(
  {
    name: 'gslides-mcp',
    version: '1.0.0',
  },
  {
    capabilities: {
      tools: {},
    },
  }
);

// Handler : Liste des tools disponibles
server.setRequestHandler(ListToolsRequestSchema, async () => {
  return {
    tools: [getTemplateBrandingTool, listPresentationsTool],
  };
});

// Handler : Appel d'un tool
server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;

  try {
    switch (name) {
      case 'get_template_branding': {
        const presentationId = (args as Record<string, unknown>)?.presentationId;
        if (typeof presentationId !== 'string') {
          return {
            content: [
              {
                type: 'text',
                text: 'Argument invalide : presentationId doit être une chaîne de caractères.',
              },
            ],
            isError: true,
          };
        }
        return await getTemplateBranding(gSlidesClient, presentationId);
      }

      case 'list_presentations': {
        return await listPresentations(gSlidesClient);
      }

      default:
        throw new Error(`Outil inconnu : ${name}`);
    }
  } catch (error) {
    return {
      content: [
        {
          type: 'text',
          text: `Erreur lors de l'exécution de ${name} : ${error instanceof Error ? error.message : 'Erreur inconnue'}`,
        },
      ],
      isError: true,
    };
  }
});

// Démarrer le serveur
async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error('Google Slides MCP Server started successfully');
}

main().catch((error) => {
  console.error('Server error:', error);
  process.exit(1);
});

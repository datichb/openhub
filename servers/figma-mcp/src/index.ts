#!/usr/bin/env node

/**
 * Figma MCP Server
 * Entry point pour le serveur MCP
 */

import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from '@modelcontextprotocol/sdk/types.js';

import { getConfig } from './config.js';
import { FigmaClient } from './client.js';
import { searchFilesTool, searchFiles } from './tools/search-files.js';
import { getFileStructureTool, getFileStructure } from './tools/get-file-structure.js';
import { detectUISignalsTool, detectUISignals } from './tools/detect-ui-signals.js';

// Initialisation
let figmaClient: FigmaClient;

try {
  const config = getConfig();
  figmaClient = new FigmaClient(config);
} catch (error) {
  console.error('Failed to initialize Figma MCP Server:', error);
  process.exit(1);
}

// Créer le serveur MCP
const server = new Server(
  {
    name: 'figma-mcp',
    version: '1.0.0',
  },
  {
    capabilities: {
      tools: {},
    },
  }
);

// Handler: Liste des tools disponibles
server.setRequestHandler(ListToolsRequestSchema, async () => {
  return {
    tools: [searchFilesTool, getFileStructureTool, detectUISignalsTool],
  };
});

// Handler: Appel d'un tool
server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;

  try {
    switch (name) {
      case 'search_figma_files': {
        if (!args || typeof args.query !== 'string') {
          throw new Error('Invalid arguments: query (string) is required');
        }
        return await searchFiles(figmaClient, args.query);
      }

      case 'get_file_structure': {
        if (!args || typeof args.fileId !== 'string') {
          throw new Error('Invalid arguments: fileId (string) is required');
        }
        return await getFileStructure(figmaClient, args.fileId);
      }

      case 'detect_ui_signals': {
        if (!args || typeof args.fileId !== 'string') {
          throw new Error('Invalid arguments: fileId (string) is required');
        }
        return await detectUISignals(figmaClient, args.fileId);
      }

      default:
        throw new Error(`Unknown tool: ${name}`);
    }
  } catch (error) {
    return {
      content: [
        {
          type: 'text',
          text: `Error executing tool ${name}: ${error instanceof Error ? error.message : 'Unknown error'}`,
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
  console.error('Figma MCP Server started successfully');
}

main().catch((error) => {
  console.error('Server error:', error);
  process.exit(1);
});

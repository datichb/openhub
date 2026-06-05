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
import { extractDesignTokensTool, extractDesignTokens } from './tools/extract-design-tokens.js';
import { getNodeDetailsTool, getNodeDetails } from './tools/get-node-details.js';

// Initialisation
function initClient(): FigmaClient {
  try {
    const config = getConfig();
    return new FigmaClient(config);
  } catch (error) {
    console.error('Failed to initialize Figma MCP Server:', error);
    process.exit(1);
  }
}

const figmaClient = initClient();

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
    tools: [searchFilesTool, getFileStructureTool, detectUISignalsTool, extractDesignTokensTool, getNodeDetailsTool],
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

      case 'extract_design_tokens': {
        if (!args || typeof args.fileId !== 'string') {
          throw new Error('Invalid arguments: fileId (string) is required');
        }
        return await extractDesignTokens(figmaClient, args.fileId);
      }

      case 'get_node_details': {
        if (!args || typeof args.fileId !== 'string' || typeof args.nodeId !== 'string') {
          throw new Error('Invalid arguments: fileId (string) and nodeId (string) are required');
        }
        return await getNodeDetails(figmaClient, args.fileId, args.nodeId);
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

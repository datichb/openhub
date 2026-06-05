#!/usr/bin/env node

/**
 * GitLab MCP Server
 * Entry point — expose 5 read-only tools to OpenCode agents
 */

import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from '@modelcontextprotocol/sdk/types.js';

import { getConfig } from './config.js';
import { GitLabClient } from './client.js';
import { getIssueTool, getIssue } from './tools/get-issue.js';
import { listIssuesTool, listIssues } from './tools/list-issues.js';
import { getMergeRequestTool, getMergeRequest } from './tools/get-merge-request.js';
import { listLabelsTool, listLabels } from './tools/list-labels.js';
import { listMilestonesTool, listMilestones } from './tools/list-milestones.js';

// ---------------------------------------------------------------------------
// Initialisation
// ---------------------------------------------------------------------------

function initClient(): GitLabClient {
  try {
    const config = getConfig();
    return new GitLabClient(config);
  } catch (error) {
    console.error('Failed to initialize GitLab MCP Server:', error);
    process.exit(1);
  }
}

const gitlabClient = initClient();

// ---------------------------------------------------------------------------
// Serveur MCP
// ---------------------------------------------------------------------------

const server = new Server(
  { name: 'gitlab-mcp', version: '1.0.0' },
  { capabilities: { tools: {} } }
);

// Handler: liste des tools disponibles
server.setRequestHandler(ListToolsRequestSchema, async () => {
  return {
    tools: [
      getIssueTool,
      listIssuesTool,
      getMergeRequestTool,
      listLabelsTool,
      listMilestonesTool,
    ],
  };
});

// Handler: appel d'un tool
server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;

  try {
    switch (name) {
      case 'get_gitlab_issue': {
        if (
          !args ||
          typeof args.project_path !== 'string' ||
          typeof args.issue_iid !== 'number'
        ) {
          throw new Error(
            'Invalid arguments: project_path (string) and issue_iid (number) are required'
          );
        }
        return await getIssue(gitlabClient, args.project_path, args.issue_iid);
      }

      case 'list_gitlab_issues': {
        if (!args || typeof args.project_path !== 'string') {
          throw new Error(
            'Invalid arguments: project_path (string) is required'
          );
        }
        return await listIssues(gitlabClient, args.project_path, {
          state: args.state as 'opened' | 'closed' | 'all' | undefined,
          labels: typeof args.labels === 'string' ? args.labels : undefined,
          search: typeof args.search === 'string' ? args.search : undefined,
          per_page: typeof args.per_page === 'number' ? args.per_page : undefined,
          page: typeof args.page === 'number' ? args.page : undefined,
        });
      }

      case 'get_gitlab_merge_request': {
        if (
          !args ||
          typeof args.project_path !== 'string' ||
          typeof args.merge_request_iid !== 'number'
        ) {
          throw new Error(
            'Invalid arguments: project_path (string) and merge_request_iid (number) are required'
          );
        }
        return await getMergeRequest(
          gitlabClient,
          args.project_path,
          args.merge_request_iid
        );
      }

      case 'list_gitlab_labels': {
        if (!args || typeof args.project_path !== 'string') {
          throw new Error(
            'Invalid arguments: project_path (string) is required'
          );
        }
        return await listLabels(gitlabClient, args.project_path);
      }

      case 'list_gitlab_milestones': {
        if (!args || typeof args.project_path !== 'string') {
          throw new Error(
            'Invalid arguments: project_path (string) is required'
          );
        }
        return await listMilestones(
          gitlabClient,
          args.project_path,
          args.state as 'active' | 'closed' | undefined
        );
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

// ---------------------------------------------------------------------------
// Démarrage
// ---------------------------------------------------------------------------

async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error('GitLab MCP Server started successfully');
}

main().catch((error) => {
  console.error('Server error:', error);
  process.exit(1);
});

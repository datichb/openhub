/**
 * Configuration pour le MCP Server Figma
 */

export interface FigmaConfig {
  token: string;
  teamId: string;
  baseUrl: string;
}

export function getConfig(): FigmaConfig {
  const token = process.env.FIGMA_PERSONAL_ACCESS_TOKEN;
  const teamId = process.env.FIGMA_TEAM_ID;

  if (!token) {
    throw new Error(
      'FIGMA_PERSONAL_ACCESS_TOKEN environment variable is required. ' +
      'Configure it in ~/.config/opencode/config.json'
    );
  }

  if (!teamId) {
    throw new Error(
      'FIGMA_TEAM_ID environment variable is required. ' +
      'Configure it in ~/.config/opencode/config.json'
    );
  }

  return {
    token,
    teamId,
    baseUrl: 'https://api.figma.com/v1',
  };
}

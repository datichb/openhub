/**
 * Tool: detect_ui_signals
 * Détecte automatiquement les signaux UX/UI dans un fichier Figma
 */

import { FigmaClient, FigmaNode } from '../client.js';

export const detectUISignalsTool = {
  name: 'detect_ui_signals',
  description: 'Automatically detect UX/UI signals in a Figma file. Returns complexity estimation and recommendations.',
  inputSchema: {
    type: 'object',
    properties: {
      fileId: {
        type: 'string',
        description: 'Figma file ID (file key)',
      },
    },
    required: ['fileId'],
  },
};

type Complexity = 'XS' | 'S' | 'M' | 'L' | 'XL';

interface UISignals {
  hasUXSignal: boolean;
  hasUISignal: boolean;
  componentsCount: number;
  framesCount: number;
  complexity: Complexity;
  reasoning: string[];
  recommendations: string[];
}

/**
 * Détecte si le fichier contient un flow multi-étapes
 */
function detectMultiStepFlow(frames: FigmaNode[]): boolean {
  // Heuristique : Recherche de frames avec des numéros (Step 1, Étape 2, etc.)
  const stepPattern = /(step|étape|phase)\s*\d+/i;
  const stepsFrames = frames.filter((f) => stepPattern.test(f.name));
  return stepsFrames.length >= 2;
}

/**
 * Détecte si le fichier contient des états visuels
 */
function detectVisualStates(frames: FigmaNode[]): string[] {
  const states = ['hover', 'focus', 'disabled', 'error', 'loading', 'success', 'active'];
  const detected: string[] = [];

  for (const state of states) {
    if (frames.some((f) => f.name.toLowerCase().includes(state))) {
      detected.push(state);
    }
  }

  return detected;
}

/**
 * Calcule la complexité en fonction des métriques
 */
function calculateComplexity(metrics: {
  componentsCount: number;
  framesCount: number;
  hasMultiStepFlow: boolean;
  statesCount: number;
}): Complexity {
  let score = 0;

  // Composants
  if (metrics.componentsCount <= 2) score += 1; // XS
  else if (metrics.componentsCount <= 5) score += 2; // S
  else if (metrics.componentsCount <= 10) score += 3; // M
  else if (metrics.componentsCount <= 20) score += 4; // L
  else score += 5; // XL

  // Frames
  if (metrics.framesCount > 10) score += 1;
  if (metrics.framesCount > 20) score += 1;

  // Flow multi-étapes
  if (metrics.hasMultiStepFlow) score += 2;

  // États visuels
  if (metrics.statesCount >= 3) score += 1;

  // Mapping score → complexité
  if (score <= 2) return 'XS';
  if (score <= 4) return 'S';
  if (score <= 6) return 'M';
  if (score <= 8) return 'L';
  return 'XL';
}

export async function detectUISignals(
  client: FigmaClient,
  fileId: string
): Promise<{ content: Array<{ type: string; text: string }> }> {
  try {
    const file = await client.getFile(fileId);
    const frames = client.extractFrames(file.document);
    const componentsCount = client.countComponents(file.document);

    const hasMultiStepFlow = detectMultiStepFlow(frames);
    const visualStates = detectVisualStates(frames);

    const complexity = calculateComplexity({
      componentsCount,
      framesCount: frames.length,
      hasMultiStepFlow,
      statesCount: visualStates.length,
    });

    // Déterminer les signaux
    const hasUXSignal = hasMultiStepFlow || frames.length >= 5;
    const hasUISignal = componentsCount >= 3 || visualStates.length >= 2;

    // Construire le raisonnement
    const reasoning: string[] = [];
    if (hasMultiStepFlow) {
      reasoning.push('Flow multi-étapes détecté (parcours utilisateur complexe)');
    }
    if (componentsCount >= 10) {
      reasoning.push(`Nombre élevé de composants (${componentsCount})`);
    } else if (componentsCount >= 3) {
      reasoning.push(`${componentsCount} composants détectés`);
    }
    if (visualStates.length > 0) {
      reasoning.push(`États visuels identifiés : ${visualStates.join(', ')}`);
    }
    if (frames.length >= 10) {
      reasoning.push(`${frames.length} frames dans le fichier`);
    }

    // Recommandations
    const recommendations: string[] = [];
    if (hasUXSignal) {
      recommendations.push('Recommandé : Déléguer à **ux-designer** pour spec du parcours utilisateur');
    }
    if (hasUISignal) {
      recommendations.push('Recommandé : Déléguer à **ui-designer** pour spec des composants visuels');
    }
    if (complexity === 'L' || complexity === 'XL') {
      recommendations.push('Complexité élevée : Escalader au **planner** pour découpage détaillé');
    }

    // Formater la sortie
    const output = [
      `## Analyse du fichier : ${file.name}`,
      '',
      `**Complexité estimée :** ${complexity}`,
      '',
      `### Signaux détectés`,
      `- **Signal UX :** ${hasUXSignal ? '⚠️ Oui' : 'Non'}`,
      `- **Signal UI :** ${hasUISignal ? '⚠️ Oui' : 'Non'}`,
      '',
      `### Métriques`,
      `- **Composants :** ${componentsCount}`,
      `- **Frames :** ${frames.length}`,
      `- **États visuels :** ${visualStates.length > 0 ? visualStates.join(', ') : 'Aucun'}`,
      '',
      `### Raisonnement`,
      reasoning.map((r) => `- ${r}`).join('\n'),
    ];

    if (recommendations.length > 0) {
      output.push('', '### Recommandations', recommendations.map((r) => `- ${r}`).join('\n'));
    }

    return {
      content: [
        {
          type: 'text',
          text: output.join('\n'),
        },
      ],
    };
  } catch (error) {
    return {
      content: [
        {
          type: 'text',
          text: `Erreur lors de la détection des signaux : ${error instanceof Error ? error.message : 'Unknown error'}`,
        },
      ],
    };
  }
}

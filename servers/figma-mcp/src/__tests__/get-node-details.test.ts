/**
 * Tests unitaires pour get_node_details tool et FigmaClient.getNode()
 */

import { describe, it, expect, vi } from 'vitest';
import { getNodeDetails } from '../tools/get-node-details.js';
import type { FigmaClient, FigmaNode } from '../client.js';

// ── Helpers ──────────────────────────────────────────────────────────────────

function makeNode(overrides: Partial<FigmaNode> = {}): FigmaNode {
  return {
    id: '122-29189',
    name: 'Panneau de filtres',
    type: 'FRAME',
    children: [],
    ...overrides,
  };
}

function makeClient(nodeOverride?: Partial<FigmaNode> | Error): Partial<FigmaClient> {
  return {
    getNode: vi.fn(async (_fileId: string, _nodeId: string) => {
      if (nodeOverride instanceof Error) throw nodeOverride;
      return makeNode(nodeOverride);
    }),
    generateFileUrl: vi.fn(
      (fileId: string, nodeId?: string) =>
        nodeId
          ? `https://www.figma.com/file/${fileId}?node-id=${nodeId}`
          : `https://www.figma.com/file/${fileId}`
    ),
  };
}

// ── getNodeDetails : cas nominaux ─────────────────────────────────────────────

describe('getNodeDetails', () => {
  it('retourne les informations de base du nœud', async () => {
    const client = makeClient();
    const result = await getNodeDetails(client as FigmaClient, 'ABC123', '122-29189');
    const text = result.content[0].text;
    expect(text).toContain('Panneau de filtres');
    expect(text).toContain('FRAME');
    expect(text).toContain('122-29189');
    expect(text).toContain('ABC123');
  });

  it('affiche le layout HORIZONTAL avec espacements', async () => {
    const client = makeClient({
      layoutMode: 'HORIZONTAL',
      itemSpacing: 16,
      paddingLeft: 24,
      paddingRight: 24,
      paddingTop: 12,
      paddingBottom: 12,
      primaryAxisAlignItems: 'SPACE_BETWEEN',
    });
    const result = await getNodeDetails(client as FigmaClient, 'ABC', '1-1');
    const text = result.content[0].text;
    expect(text).toContain('HORIZONTAL');
    expect(text).toContain('16px');
    expect(text).toContain('24');
    expect(text).toContain('SPACE_BETWEEN');
  });

  it('affiche le layout VERTICAL', async () => {
    const client = makeClient({ layoutMode: 'VERTICAL', itemSpacing: 8 });
    const result = await getNodeDetails(client as FigmaClient, 'ABC', '1-2');
    const text = result.content[0].text;
    expect(text).toContain('VERTICAL');
    expect(text).toContain('8px');
  });

  it('affiche les dimensions et position si absoluteBoundingBox présent', async () => {
    const client = makeClient({
      absoluteBoundingBox: { x: 100, y: 200, width: 320, height: 480 },
    });
    const result = await getNodeDetails(client as FigmaClient, 'ABC', '1-3');
    const text = result.content[0].text;
    expect(text).toContain('320');
    expect(text).toContain('480');
  });

  it('affiche les propriétés de composant', async () => {
    const client = makeClient({
      type: 'COMPONENT',
      componentPropertyDefinitions: {
        Variant: {
          type: 'VARIANT',
          defaultValue: 'primary',
          variantOptions: ['primary', 'secondary', 'ghost'],
        },
        Disabled: {
          type: 'BOOLEAN',
          defaultValue: false,
        },
      },
    });
    const result = await getNodeDetails(client as FigmaClient, 'ABC', '1-4');
    const text = result.content[0].text;
    expect(text).toContain('Variant');
    expect(text).toContain('primary');
    expect(text).toContain('secondary');
    expect(text).toContain('Disabled');
  });

  it('affiche les enfants directs', async () => {
    const client = makeClient({
      children: [
        makeNode({ id: '1-10', name: 'Header', type: 'FRAME', layoutMode: 'HORIZONTAL' }),
        makeNode({ id: '1-11', name: 'Body', type: 'FRAME', layoutMode: 'VERTICAL' }),
        makeNode({ id: '1-12', name: 'Label', type: 'TEXT' }),
      ],
    });
    const result = await getNodeDetails(client as FigmaClient, 'ABC', '1-5');
    const text = result.content[0].text;
    expect(text).toContain('Header');
    expect(text).toContain('HORIZONTAL');
    expect(text).toContain('Body');
    expect(text).toContain('Label');
  });

  it('affiche le contenu textuel si characters présent', async () => {
    const client = makeClient({
      type: 'TEXT',
      characters: 'Filtrer par statut',
    });
    const result = await getNodeDetails(client as FigmaClient, 'ABC', '1-6');
    const text = result.content[0].text;
    expect(text).toContain('Filtrer par statut');
  });

  it('affiche les fills solides', async () => {
    const client = makeClient({
      fills: [{ type: 'SOLID', color: { r: 0, g: 0.47, b: 1, a: 1 } }],
    });
    const result = await getNodeDetails(client as FigmaClient, 'ABC', '1-7');
    const text = result.content[0].text;
    expect(text).toContain('Solide');
    expect(text).toContain('#');
  });

  // ── Gestion des erreurs ──────────────────────────────────────────────────

  it('retourne un message timeout si erreur indisponible', async () => {
    // On simule le message produit par classifyFigmaError pour un timeout
    const err = new Error('indisponible timeout 30s tentative 3/2');
    const client = makeClient(err);
    const result = await getNodeDetails(client as FigmaClient, 'ABC', '122-29189');
    const text = result.content[0].text;
    expect(text).toContain('indisponible');
    expect(text).toContain('122-29189');
  });

  it('retourne un message authentification si erreur 401/Token Figma', async () => {
    // getNodeDetails classifie l'erreur via son propre catch — on teste le comportement,
    // pas le format exact produit par classifyFigmaError
    const err = new Error('Token Figma invalide — 401');
    const client = makeClient(err);
    const result = await getNodeDetails(client as FigmaClient, 'ABC', '1-8');
    const text = result.content[0].text;
    expect(text).toContain('authentification');
  });

  it('retourne le message d\'erreur et ne propage pas l\'exception', async () => {
    // getNodeDetails doit capturer toute erreur et retourner un résultat MCP valide
    const err = new Error('erreur réseau inattendue');
    const client = makeClient(err);
    const result = await getNodeDetails(client as FigmaClient, 'ABC', '1-9');
    expect(result.content).toHaveLength(1);
    expect(result.content[0].type).toBe('text');
    expect(result.content[0].text).toContain('erreur réseau inattendue');
  });

  it('retourne un message introuvable si nœud absent', async () => {
    const err = new Error('introuvable ID: 999-999');
    const client = makeClient(err);
    const result = await getNodeDetails(client as FigmaClient, 'ABC', '999-999');
    const text = result.content[0].text;
    expect(text).toContain('introuvable');
  });
});

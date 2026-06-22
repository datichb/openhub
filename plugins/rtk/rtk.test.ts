/**
 * Tests unitaires pour le plugin RTK
 * Couvre : tool.execute.before, tool.execute.after, dispose
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { Plugin } from '@opencode-ai/plugin';
import { RtkOpenCodePlugin } from './rtk.js';

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Réponses shell communes à tous les tests nécessitant RTK disponible */
const RTK_BASE_RESPONSES = {
  'which rtk': { stdout: '/usr/local/bin/rtk' },
  'rtk --version': { stdout: 'rtk 0.42.0' },
} as const;

/** Réponses shell pour une session avec réécriture activée */
const RTK_SESSION_RESPONSES = {
  ...RTK_BASE_RESPONSES,
  'rtk gain': { stdout: JSON.stringify({ summary: { total_commands: 1, total_saved: 5000, avg_savings_pct: 10 } }) },
  'hook check': { stdout: 'rtk read file.ts' },
} as const;

/** Crée un ProcessPromise-like : thenable avec .quiet()/.nothrow() sur l'objet lui-même */
function makeChainable(result: { stdout: string; stderr: string; exitCode: number }): any {
  const chain: any = {
    ...result,
    // Thenable — await chain résout à result
    then: (resolve: any, reject?: any) => Promise.resolve(result).then(resolve, reject),
    catch: (fn: any) => Promise.resolve(result).catch(fn),
    finally: (fn: any) => Promise.resolve(result).finally(fn),
    // Méthodes chaînées — retournent un nouveau chainable ou une Promise simple
    quiet: () => makeChainable(result),
    nothrow: () => Promise.resolve(result),
  };
  return chain;
}

/** Crée un mock de la fonction shell $ */
function makeShell(responses: Record<string, { stdout: string; exitCode?: number }> = {}) {
  const impl = (strings: any, ...values: unknown[]) => {
    const cmd = typeof strings === 'string' ? strings : String.raw({ raw: strings }, ...values).trim();
    const match = Object.entries(responses).find(([key]) => cmd.includes(key));
    const stdout = match ? match[1].stdout : '';
    const result = { stdout, stderr: '', exitCode: match?.[1].exitCode ?? 0 };
    return makeChainable(result);
  };
  return vi.fn(impl);
}

/** Crée un mock du client OpenCode */
function makeClient() {
  return {
    app: {
      log: vi.fn().mockResolvedValue(undefined),
    },
    tui: {
      toast: vi.fn().mockResolvedValue(undefined),
    },
  };
}

/** Initialise le plugin et retourne ses hooks */
async function initPlugin(
  shellResponses: Record<string, { stdout: string; exitCode?: number }> = {},
  clientOverrides: Partial<ReturnType<typeof makeClient>> = {}
) {
  const $ = makeShell(shellResponses);
  const client = { ...makeClient(), ...clientOverrides };
  const hooks = await RtkOpenCodePlugin({ $, client } as any);
  return { $, client, hooks };
}

/**
 * Initialise le plugin avec une session déjà démarrée (une commande bash réécrite).
 * Utilisé par les tests dispose qui testent le comportement post-session.
 */
async function initWithSession(
  extraShellResponses: Record<string, { stdout: string; exitCode?: number }> = {}
) {
  const { hooks, client, $ } = await initPlugin({ ...RTK_SESSION_RESPONSES, ...extraShellResponses });
  const before = hooks['tool.execute.before']!;
  // Déclenche une réécriture pour initier la session
  await before({ tool: 'bash' }, { args: { command: 'cat file.ts' } } as any);
  return { hooks, client, $ };
}

// ── Initialisation ────────────────────────────────────────────────────────────

describe('RtkOpenCodePlugin — initialisation', () => {
  it('retourne les 3 hooks même si rtk est absent (lazy init)', async () => {
    // Avec lazy init, RtkOpenCodePlugin TOUJOURS retourne les 3 hooks.
    // C'est à l'exécution des hooks que rtkAvailable est vérifié.
    const $ = makeShell();
    $.mockImplementation((strings: any) => {
      const cmd = typeof strings === 'string' ? strings : String.raw({ raw: strings });
      if (cmd.includes('which rtk')) throw new Error('not found');
      return makeChainable({ stdout: '', stderr: '', exitCode: 0 });
    });
    const client = makeClient();
    const hooks = await RtkOpenCodePlugin({ $, client } as any);
    expect(hooks).toHaveProperty('tool.execute.before');
    expect(hooks).toHaveProperty('tool.execute.after');
    expect(hooks).toHaveProperty('dispose');
    // Les hooks sont des no-ops quand rtk est absent
    await hooks['tool.execute.before']!({ tool: 'bash' }, { args: { command: 'cat file' } } as any);
    expect((client.tui.toast as any).mock.calls.length).toBe(0);
  });

  it('retourne les hooks si rtk est disponible', async () => {
    const { hooks } = await initPlugin({
      ...RTK_BASE_RESPONSES,
      'rtk gain': { stdout: JSON.stringify({ summary: { total_commands: 0, total_saved: 0, avg_savings_pct: 0 } }) },
    });
    expect(hooks).toHaveProperty('tool.execute.before');
    expect(hooks).toHaveProperty('tool.execute.after');
    expect(hooks).toHaveProperty('dispose');
  });
});

// ── tool.execute.before ───────────────────────────────────────────────────────

describe('tool.execute.before', () => {
  it('ignore les outils non-bash/non-websearch (ex: read)', async () => {
    const { hooks, $ } = await initPlugin(RTK_BASE_RESPONSES);
    const before = hooks['tool.execute.before']!;
    await before({ tool: 'read' }, { args: { path: '/foo' } } as any);
    // Aucun appel rtk hook check attendu
    const rtkCalls = ($ as ReturnType<typeof vi.fn>).mock.calls.filter((c: any) => {
      const cmd = typeof c[0] === 'string' ? c[0] : String.raw({ raw: c[0] });
      return cmd.includes('hook check');
    });
    expect(rtkCalls.length).toBe(0);
  });

  it('réécrit une commande bash via rtk hook check', async () => {
    const args = { command: 'cat large-file.ts' };
    const { hooks } = await initPlugin({
      ...RTK_BASE_RESPONSES,
      'rtk gain': { stdout: JSON.stringify({ summary: { total_commands: 0, total_saved: 0, avg_savings_pct: 0 } }) },
      'hook check': { stdout: 'rtk read large-file.ts' },
    });
    const before = hooks['tool.execute.before']!;
    await before({ tool: 'bash' }, { args } as any);
    expect(args.command).toBe('rtk read large-file.ts');
  });

  it('ne réécrit pas si rtk répond "No rewrite"', async () => {
    const args = { command: 'echo hello' };
    const { hooks } = await initPlugin({
      ...RTK_BASE_RESPONSES,
      'rtk gain': { stdout: JSON.stringify({ summary: { total_commands: 0, total_saved: 0, avg_savings_pct: 0 } }) },
      'hook check': { stdout: 'No rewrite needed' },
    });
    const before = hooks['tool.execute.before']!;
    await before({ tool: 'bash' }, { args } as any);
    expect(args.command).toBe('echo hello');
  });

  it('ne réécrit pas une commande déjà préfixée rtk', async () => {
    const args = { command: 'rtk read file.ts' };
    const originalCmd = args.command;
    const { hooks } = await initPlugin({
      ...RTK_BASE_RESPONSES,
      'rtk gain': { stdout: JSON.stringify({ summary: { total_commands: 0, total_saved: 0, avg_savings_pct: 0 } }) },
    });
    const before = hooks['tool.execute.before']!;
    await before({ tool: 'bash' }, { args } as any);
    expect(args.command).toBe(originalCmd);
  });

  it('track les appels websearch', async () => {
    const { hooks, client } = await initPlugin({
      ...RTK_BASE_RESPONSES,
      'rtk gain': { stdout: JSON.stringify({ summary: { total_commands: 0, total_saved: 0, avg_savings_pct: 0 } }) },
    });
    const before = hooks['tool.execute.before']!;
    await before({ tool: 'websearch' }, {} as any);
    const logCalls = (client.app.log as any).mock.calls;
    const wsLog = logCalls.find((c: any) => c[0]?.body?.extra?.session_websearch_calls === 1);
    expect(wsLog).toBeDefined();
  });

  it('track les appels webfetch', async () => {
    const { hooks, client } = await initPlugin({
      ...RTK_BASE_RESPONSES,
      'rtk gain': { stdout: JSON.stringify({ summary: { total_commands: 0, total_saved: 0, avg_savings_pct: 0 } }) },
    });
    const before = hooks['tool.execute.before']!;
    await before({ tool: 'webfetch' }, {} as any);
    const logCalls = (client.app.log as any).mock.calls;
    const wfLog = logCalls.find((c: any) => c[0]?.body?.extra?.session_webfetch_calls === 1);
    expect(wfLog).toBeDefined();
  });
});

// ── tool.execute.after ────────────────────────────────────────────────────────

describe('tool.execute.after', () => {
  it('track un rate-limit websearch', async () => {
    // rtkAvailable doit être initialisé avant d'appeler after
    // On déclenche before avec un tool 'read' (no-op métier) pour forcer initRtk
    const { hooks, client } = await initPlugin(RTK_BASE_RESPONSES);
    await hooks['tool.execute.before']!({ tool: 'read' }, { args: { path: '/foo' } } as any);

    const after = hooks['tool.execute.after']!;
    await after({ tool: 'websearch' }, { error: 'rate limit exceeded' } as any);
    const warnLogs = (client.app.log as any).mock.calls.filter(
      (c: any) => c[0]?.body?.level === 'warn'
    );
    expect(warnLogs.length).toBeGreaterThan(0);
    expect(warnLogs[0][0].body.extra.session_rate_limits).toBe(1);
  });

  it('ignore les outils non-bash/non-websearch dans after', async () => {
    const { hooks, client } = await initPlugin(RTK_BASE_RESPONSES);
    const after = hooks['tool.execute.after']!;
    const callsBefore = (client.app.log as any).mock.calls.length;
    await after({ tool: 'read' }, {} as any);
    expect((client.app.log as any).mock.calls.length).toBe(callsBefore);
  });
});

// ── dispose ───────────────────────────────────────────────────────────────────
// Le hook "dispose" est le hook officiel de fin de session (@opencode-ai/plugin).
// Il remplace le hook non-existant "session.idle".

describe('dispose', () => {
  it('ne fait rien si sessionStarted est false', async () => {
    // Pas d'appel à before — session jamais démarrée
    const { hooks, client } = await initPlugin(RTK_BASE_RESPONSES);
    await hooks['dispose']!();
    expect((client.tui.toast as any).mock.calls.length).toBe(0);
  });

  it('affiche un résumé si des commandes ont été réécrites', async () => {
    const { hooks, client } = await initWithSession();
    await hooks['dispose']!();
    expect((client.tui.toast as any).mock.calls.length).toBeGreaterThan(0);
    const toastMsg = (client.tui.toast as any).mock.calls[0][0].body.message;
    expect(toastMsg).toContain('RTK');
  });

  it('remet sessionStarted à false après dispose (permet une 2e session)', async () => {
    const { hooks, client } = await initWithSession();
    await hooks['dispose']!();

    // Réinitialiser les mocks et s'assurer qu'une seconde session peut démarrer
    (client.app.log as any).mockClear();
    (client.tui.toast as any).mockClear();

    // Un nouvel appel à before doit ré-appeler initSession (log "RTK session started")
    // Note: initRtk ne sera pas rappelé (rtkAvailable déjà établi) mais initSession oui
    await hooks['tool.execute.before']!({ tool: 'bash' }, { args: { command: 'ls' } } as any);
    const sessionLog = (client.app.log as any).mock.calls.find(
      (c: any) => c[0]?.body?.message === 'RTK session started'
    );
    expect(sessionLog).toBeDefined();
  });

  it('affiche un résumé websearch si des appels ont été effectués', async () => {
    const { hooks, client } = await initWithSession();
    // Ajouter un appel websearch après la session initiée
    await hooks['tool.execute.before']!({ tool: 'websearch' }, {} as any);
    await hooks['dispose']!();
    const toastCalls = (client.tui.toast as any).mock.calls;
    const wsToast = toastCalls.find((c: any) => c[0]?.body?.message?.toLowerCase().includes('websearch'));
    expect(wsToast).toBeDefined();
  });
});

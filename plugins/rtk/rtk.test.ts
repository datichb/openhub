/**
 * Tests unitaires pour le plugin RTK
 * Couvre : tool.execute.before, tool.execute.after, dispose
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { Plugin } from '@opencode-ai/plugin';
import { RtkOpenCodePlugin } from './rtk.js';

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Crée un mock de la fonction shell $ */
function makeShell(responses: Record<string, { stdout: string; exitCode?: number }> = {}) {
  const shell = vi.fn(async (strings: TemplateStringsArray, ...values: unknown[]) => {
    const cmd = String.raw({ raw: strings }, ...values).trim();
    const match = Object.entries(responses).find(([key]) => cmd.includes(key));
    const stdout = match ? match[1].stdout : '';
    const result = {
      stdout,
      stderr: '',
      exitCode: match?.[1].exitCode ?? 0,
    };
    return {
      ...result,
      quiet: () => ({ ...result, nothrow: () => Promise.resolve(result) }),
      nothrow: () => Promise.resolve(result),
    };
  }) as any;

  // Support tagged template literal syntax
  shell.mockImplementation(async (strings: any, ...values: unknown[]) => {
    const cmd = typeof strings === 'string' ? strings : String.raw({ raw: strings }, ...values).trim();
    const match = Object.entries(responses).find(([key]) => cmd.includes(key));
    const stdout = match ? match[1].stdout : '';
    const result = { stdout, stderr: '', exitCode: match?.[1].exitCode ?? 0 };
    const chainable = {
      ...result,
      quiet: () => chainable,
      nothrow: () => Promise.resolve(result),
    };
    return chainable;
  });

  return shell;
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

// ── Initialisation ────────────────────────────────────────────────────────────

describe('RtkOpenCodePlugin — initialisation', () => {
  it('retourne {} si rtk est absent du PATH', async () => {
    const $ = makeShell();
    $.mockImplementation(async (strings: any) => {
      const cmd = typeof strings === 'string' ? strings : String.raw({ raw: strings });
      if (cmd.includes('which rtk')) throw new Error('not found');
      return { stdout: '', stderr: '', exitCode: 0, quiet: () => ({ nothrow: () => Promise.resolve({ stdout: '', exitCode: 0 }) }), nothrow: () => Promise.resolve({ stdout: '', exitCode: 0 }) };
    });
    const client = makeClient();
    const hooks = await RtkOpenCodePlugin({ $, client } as any);
    expect(hooks).toEqual({});
  });

  it('retourne les hooks si rtk est disponible', async () => {
    const { hooks } = await initPlugin({
      'which rtk': { stdout: '/usr/local/bin/rtk' },
      'rtk --version': { stdout: 'rtk 0.42.0' },
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
    const { hooks, $ } = await initPlugin({
      'which rtk': { stdout: '/usr/local/bin/rtk' },
      'rtk --version': { stdout: 'rtk 0.42.0' },
    });
    const before = hooks['tool.execute.before']!;
    await before({ tool: 'read' }, { args: { path: '/foo' } } as any);
    // Aucun appel rtk hook check attendu
    const rtkCalls = ($ as any).mock?.calls?.filter?.((c: any) => {
      const cmd = typeof c[0] === 'string' ? c[0] : String.raw({ raw: c[0] });
      return cmd.includes('hook check');
    }) ?? [];
    expect(rtkCalls.length).toBe(0);
  });

  it('réécrit une commande bash via rtk hook check', async () => {
    const args = { command: 'cat large-file.ts' };
    const { hooks } = await initPlugin({
      'which rtk': { stdout: '/usr/local/bin/rtk' },
      'rtk --version': { stdout: 'rtk 0.42.0' },
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
      'which rtk': { stdout: '/usr/local/bin/rtk' },
      'rtk --version': { stdout: 'rtk 0.42.0' },
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
      'which rtk': { stdout: '/usr/local/bin/rtk' },
      'rtk --version': { stdout: 'rtk 0.42.0' },
      'rtk gain': { stdout: JSON.stringify({ summary: { total_commands: 0, total_saved: 0, avg_savings_pct: 0 } }) },
    });
    const before = hooks['tool.execute.before']!;
    await before({ tool: 'bash' }, { args } as any);
    expect(args.command).toBe(originalCmd);
  });

  it('track les appels websearch', async () => {
    const { hooks, client } = await initPlugin({
      'which rtk': { stdout: '/usr/local/bin/rtk' },
      'rtk --version': { stdout: 'rtk 0.42.0' },
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
      'which rtk': { stdout: '/usr/local/bin/rtk' },
      'rtk --version': { stdout: 'rtk 0.42.0' },
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
    const { hooks, client } = await initPlugin({
      'which rtk': { stdout: '/usr/local/bin/rtk' },
      'rtk --version': { stdout: 'rtk 0.42.0' },
    });
    const after = hooks['tool.execute.after']!;
    await after({ tool: 'websearch' }, { error: 'rate limit exceeded' } as any);
    const warnLogs = (client.app.log as any).mock.calls.filter(
      (c: any) => c[0]?.body?.level === 'warn'
    );
    expect(warnLogs.length).toBeGreaterThan(0);
    expect(warnLogs[0][0].body.extra.session_rate_limits).toBe(1);
  });

  it('ignore les outils non-bash/non-websearch dans after', async () => {
    const { hooks, client } = await initPlugin({
      'which rtk': { stdout: '/usr/local/bin/rtk' },
      'rtk --version': { stdout: 'rtk 0.42.0' },
    });
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
    const { hooks, client } = await initPlugin({
      'which rtk': { stdout: '/usr/local/bin/rtk' },
      'rtk --version': { stdout: 'rtk 0.42.0' },
    });
    const dispose = hooks['dispose']!;
    await dispose();
    expect((client.tui.toast as any).mock.calls.length).toBe(0);
  });

  it('affiche un résumé si des commandes ont été réécrites', async () => {
    const { hooks, client } = await initPlugin({
      'which rtk': { stdout: '/usr/local/bin/rtk' },
      'rtk --version': { stdout: 'rtk 0.42.0' },
      'rtk gain': { stdout: JSON.stringify({ summary: { total_commands: 5, total_saved: 50000, avg_savings_pct: 30 } }) },
      'hook check': { stdout: 'rtk read file.ts' },
    });

    // Déclencher une réécriture pour initier la session
    const before = hooks['tool.execute.before']!;
    await before({ tool: 'bash' }, { args: { command: 'cat file.ts' } } as any);

    const dispose = hooks['dispose']!;
    await dispose();

    expect((client.tui.toast as any).mock.calls.length).toBeGreaterThan(0);
    const toastMsg = (client.tui.toast as any).mock.calls[0][0].body.message;
    expect(toastMsg).toContain('RTK');
  });

  it('remet sessionStarted à false après dispose (permet une 2e session)', async () => {
    const { hooks, client } = await initPlugin({
      'which rtk': { stdout: '/usr/local/bin/rtk' },
      'rtk --version': { stdout: 'rtk 0.42.0' },
      'rtk gain': { stdout: JSON.stringify({ summary: { total_commands: 1, total_saved: 5000, avg_savings_pct: 10 } }) },
      'hook check': { stdout: 'rtk read file.ts' },
    });

    const before = hooks['tool.execute.before']!;
    await before({ tool: 'bash' }, { args: { command: 'cat file.ts' } } as any);

    const dispose = hooks['dispose']!;
    await dispose();

    // Réinitialiser les mocks et s'assurer qu'une seconde session peut démarrer
    (client.app.log as any).mockClear();
    (client.tui.toast as any).mockClear();

    // Un nouvel appel à before doit ré-appeler initSession (log "RTK plugin initialized")
    await before({ tool: 'bash' }, { args: { command: 'ls' } } as any);
    const initLog = (client.app.log as any).mock.calls.find(
      (c: any) => c[0]?.body?.message === 'RTK plugin initialized'
    );
    expect(initLog).toBeDefined();
  });

  it('affiche un résumé websearch si des appels ont été effectués', async () => {
    const { hooks, client } = await initPlugin({
      'which rtk': { stdout: '/usr/local/bin/rtk' },
      'rtk --version': { stdout: 'rtk 0.42.0' },
      'rtk gain': { stdout: JSON.stringify({ summary: { total_commands: 1, total_saved: 5000, avg_savings_pct: 10 } }) },
      'hook check': { stdout: 'rtk read file.ts' },
    });

    const before = hooks['tool.execute.before']!;
    // Déclencher une réécriture bash pour initier session + activer compteurs
    await before({ tool: 'bash' }, { args: { command: 'cat file.ts' } } as any);
    await before({ tool: 'websearch' }, {} as any);

    const dispose = hooks['dispose']!;
    await dispose();

    const toastCalls = (client.tui.toast as any).mock.calls;
    const wsToast = toastCalls.find((c: any) => c[0]?.body?.message?.toLowerCase().includes('websearch'));
    expect(wsToast).toBeDefined();
  });
});

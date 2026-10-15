// oh plugin for opencode V2 — embedded in the oh binary and written into each
// server group. It carries no business logic: everything it applies comes from
// the options rendered by oh.
//
//   1. Agent prompt injection: the body of the current oh agent is added to
//      the system prompt (after opencode's base prompt) instead of replacing
//      it through the `system` agent field.
//   2. Closed world: agents and skills that are not part of the session bundle
//      are removed from the registries (defence in depth on top of the
//      rendered `disabled` / permission rules).
//   3. Checkpoints (level 3): before a permission is asked, the request is
//      relayed to the oh daemon, which may turn it into allow / deny. The
//      plugin only forwards: oh decides. Without an answer (oh unreachable,
//      timeout) the rendered decision stands, except for the requests held
//      until oh confirms the session rules (option pending): refused.
//   4. Guard: the deny/ask rules of oh that come last for every agent are
//      applied to each evaluated permission (a line already refused by the
//      rules never reaches the hook); the shell never gets the server
//      secrets (scrubEnv).
//
// Options (set by oh):
//   agentsDir     directory holding <agent-id>.md bodies
//   agents        agent IDs of the bundle
//   skills        skill IDs of the bundle
//   systemAgents  internal opencode agents to keep (compaction, title, summary)
//   traceFile     optional JSONL trace of injected prompts (tests only)
//   hookURL       oh hooks base URL (credential proxy listener)
//   tokenEnv      environment variable holding the session token for hookURL
//   gated         actions relayed to oh even when allowed (an "always"
//                 answer saved by the tool never passes a checkpoint)
//   scrubEnv      server variables removed from every shell environment
//                 (password of the server, inline config, tokens)
//   guard         deny/ask rules of oh in force last for every agent (global
//                 guard, applied when a permission is evaluated instead of
//                 being rendered in every agent)
//   scopedGuard   deny rules of oh in force last for every agent but those
//                 of their `except` list (session scratch folders)
//   pending       requests held until oh confirms the session rules of the
//                 session (operations locked by a checkpoint): relayed to
//                 oh even when allowed, refused when oh does not answer
//                 (subagent session not adopted yet, daemon unreachable)
//   pendingMessage refusal of a held request oh did not answer

import { appendFileSync, readFileSync } from "fs"
import { join } from "path"

type Options = {
  agentsDir?: string
  agents?: string[]
  skills?: string[]
  systemAgents?: string[]
  traceFile?: string
  hookURL?: string
  tokenEnv?: string
  gated?: string[]
  scrubEnv?: string[]
  guard?: Rule[]
  scopedGuard?: ScopedRule[]
  pending?: Rule[]
  pendingMessage?: string
}

type Rule = { action: string; resource: string; effect: string }
type ScopedRule = Rule & { except?: string[] }

const hookTimeoutMs = 2000
// oh may adopt a subagent session before it answers a held request.
const pendingTimeoutMs = 6000
const maxCalls = 500

const bodies = new Map<string, string | null>()

function agentBody(dir: string | undefined, id: string): string | null {
  if (!dir || !id) return null
  if (!bodies.has(id)) {
    try {
      bodies.set(id, readFileSync(join(dir, `${id}.md`), "utf8"))
    } catch {
      bodies.set(id, null)
    }
  }
  return bodies.get(id) ?? null
}

// match is the wildcard of the tool permission rules (same semantics as
// opencode 2.0.20 util/wildcard.ts): whole value, '*' any run, '?' one
// character, a trailing " *" also matches the bare command.
function match(input: string, pattern: string): boolean {
  const normalized = input.replaceAll("\\", "/")
  let escaped = pattern
    .replaceAll("\\", "/")
    .replace(/[.+^${}()|[\]\\]/g, "\\$&")
    .replace(/\*/g, ".*")
    .replace(/\?/g, ".")
  if (escaped.endsWith(" .*")) escaped = escaped.slice(0, -3) + "( .*)?"
  return new RegExp("^" + escaped + "$", process.platform === "win32" ? "si" : "s").test(normalized)
}

// guardEffect returns the most restrictive effect the guard gives to the
// resources of a request ("" when no guard rule matches).
function guardEffect(guard: Rule[], action: string, resources: string[]): string {
  let out = ""
  for (const resource of resources) {
    for (const r of guard) {
      if (!match(action ?? "", r.action) || !match(resource, r.resource)) continue
      if (r.effect === "deny") return "deny"
      out = "ask"
    }
  }
  return out
}

function text(part: any): string {
  return typeof part === "string" ? part : (part?.text ?? "")
}

export default {
  id: "oh",
  async setup(ctx: any) {
    const opts: Options = ctx.options ?? {}
    const agents = new Set(opts.agents ?? [])
    const skills = new Set(opts.skills ?? [])
    const system = new Set(opts.systemAgents ?? ["compaction", "title", "summary"])

    if (agents.size > 0) {
      await ctx.agent.transform((editor: any) => {
        for (const a of editor.list()) {
          if (!agents.has(a.id) && !system.has(a.id)) editor.remove(a.id)
        }
      })
    }
    if (opts.skills) {
      await ctx.skill.transform((editor: any) => {
        for (const s of editor.list()) {
          if (!skills.has(s.id)) editor.remove(s.id)
        }
      })
    }

    if (opts.scrubEnv && opts.scrubEnv.length > 0) {
      // A shell without a session environment inherits the server process
      // environment: its secrets never reach the agent.
      const scrub = [...opts.scrubEnv]
      await ctx.shell.hook("create.before", (event: any) => {
        if (!event?.env) return
        for (const k of scrub) delete event.env[k]
      })
    }

    const guard = (opts.guard ?? []).filter((r) => r.effect === "deny" || r.effect === "ask")
    const scoped = (opts.scopedGuard ?? []).filter((r) => r.effect === "deny")
    // The scoped rules that apply to an agent (not in their except list).
    const guardOf = (agent: any): Rule[] => {
      if (scoped.length === 0) return guard
      const id = typeof agent === "string" ? agent : agent?.id
      return [...guard, ...scoped.filter((r) => !(r.except ?? []).includes(id))]
    }
    const relay = opts.hookURL && opts.tokenEnv
    const gated = new Set(opts.gated ?? [])
    // The permission event does not carry the tool input: keep it by call id.
    const inputs = new Map<string, any>()
    if (relay) {
      await ctx.tool.hook("execute.before", (event: any) => {
        if (!event?.id) return
        inputs.set(event.id, event.input)
        if (inputs.size > maxCalls) inputs.delete(inputs.keys().next().value as string)
      })
    }
    // Held requests (p6-debts, closed by default): the rendered rules of a
    // session do not hold the locks of its checkpoints until oh has set its
    // session rules; only oh's answer lets them through.
    const pending = (opts.pending ?? []).filter((r) => r.effect === "deny")
    const pendingMessage = opts.pendingMessage || "Refused by oh: the rules of this session are not confirmed."
    if (relay || guard.length > 0 || scoped.length > 0 || pending.length > 0) {
      await ctx.permission.hook("evaluate", async (event: any) => {
        const held =
          pending.length > 0 && event?.effect !== "deny" && guardEffect(pending, event?.action, event?.resources ?? []) === "deny"
        if (held) {
          const answered = relay ? await relayToOh(event, true) : false
          if (!answered) {
            event.effect = "deny"
            event.message = pendingMessage
          }
        } else if (relay && (event?.effect === "ask" || (event?.effect === "allow" && gated.has(event?.action)))) {
          await relayToOh(event, false)
        }
        // The guard of oh, last of every agent: never loosened by the relay.
        const restricted = guardEffect(guardOf(event?.agent), event?.action, event?.resources ?? [])
        if (restricted === "deny" || (restricted === "ask" && event.effect === "allow")) event.effect = restricted
      })
    }

    // relayToOh asks oh about a permission; it reports whether oh answered.
    async function relayToOh(event: any, held: boolean): Promise<boolean> {
      const callID = event.source?.id
      try {
        const res = await fetch(`${opts.hookURL}/permission`, {
          method: "POST",
          headers: { "content-type": "application/json", authorization: `Bearer ${process.env[opts.tokenEnv!] ?? ""}` },
          body: JSON.stringify({
            sessionID: event.sessionID,
            agent: typeof event.agent === "string" ? event.agent : event.agent?.id,
            action: event.action,
            resources: event.resources,
            callID,
            input: callID ? inputs.get(callID) : undefined,
            pending: held || undefined,
          }),
          signal: AbortSignal.timeout(held ? pendingTimeoutMs : hookTimeoutMs),
        })
        if (!res.ok) return false
        const out: any = await res.json()
        if (out?.effect === "allow" || out?.effect === "deny" || out?.effect === "ask") event.effect = out.effect
        if (typeof out?.message === "string" && out.message) event.message = out.message
        return true
      } catch {
        // oh unreachable: the rendered decision stands (a held request is refused)
        return false
      }
    }

    await ctx.session.hook("context", (event: any) => {
      const id = typeof event.agent === "string" ? event.agent : event.agent?.id
      const body = agentBody(opts.agentsDir, id)
      if (!body || !Array.isArray(event.system)) return
      const at = event.system.length > 0 ? 1 : 0
      event.system.splice(at, 0, { type: "text", text: body })
      if (opts.traceFile) {
        try {
          appendFileSync(
            opts.traceFile,
            JSON.stringify({ agent: id, parts: event.system.map((p: any) => text(p).slice(0, 120)) }) + "\n",
          )
        } catch {
          // tracing must never break a session
        }
      }
    })
  },
}

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
//      timeout) the rendered decision stands.
//
// Options (set by oh):
//   agentsDir     directory holding <agent-id>.md bodies
//   agents        agent IDs of the bundle
//   skills        skill IDs of the bundle
//   systemAgents  internal opencode agents to keep (compaction, title, summary)
//   traceFile     optional JSONL trace of injected prompts (tests only)
//   hookURL       oh hooks base URL (credential proxy listener)
//   tokenEnv      environment variable holding the session token for hookURL

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
}

const hookTimeoutMs = 2000
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

    if (opts.hookURL && opts.tokenEnv) {
      // The permission event does not carry the tool input: keep it by call id.
      const inputs = new Map<string, any>()
      await ctx.tool.hook("execute.before", (event: any) => {
        if (!event?.id) return
        inputs.set(event.id, event.input)
        if (inputs.size > maxCalls) inputs.delete(inputs.keys().next().value as string)
      })
      await ctx.permission.hook("evaluate", async (event: any) => {
        if (event?.effect !== "ask") return
        const callID = event.source?.id
        try {
          const res = await fetch(`${opts.hookURL}/permission`, {
            method: "POST",
            headers: { "content-type": "application/json", authorization: `Bearer ${process.env[opts.tokenEnv!] ?? ""}` },
            body: JSON.stringify({
              sessionID: event.sessionID,
              agent: event.agent,
              action: event.action,
              resources: event.resources,
              callID,
              input: callID ? inputs.get(callID) : undefined,
            }),
            signal: AbortSignal.timeout(hookTimeoutMs),
          })
          if (!res.ok) return
          const out: any = await res.json()
          if (out?.effect === "allow" || out?.effect === "deny" || out?.effect === "ask") event.effect = out.effect
          if (typeof out?.message === "string" && out.message) event.message = out.message
        } catch {
          // oh unreachable: the rendered decision stands
        }
      })
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

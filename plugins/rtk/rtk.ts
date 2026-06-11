import type { Plugin } from "@opencode-ai/plugin"

// RTK OpenCode Plugin — Enhanced Edition
// 
// Automatically rewrites bash commands to use RTK for token optimization,
// tracks savings per project, and provides visual feedback via toasts.
//
// Requirements:
// - RTK >= 0.42.0 in PATH with JSON support
// - OpenCode with plugin support
//
// Installation:
// - Copy this file to ~/.config/opencode/plugins/rtk.ts
// - Or use: oc plugin install rtk
//
// Version: 1.1.0 (2026-06-08)
// Compatible with: RTK 0.42.0+, OpenCode 1.15.0+

export const RtkOpenCodePlugin: Plugin = async ({ $, client }) => {
  // ───────────────────────────────────────────────────────────────────────────
  // Initialization & Validation
  // ───────────────────────────────────────────────────────────────────────────
  
  try {
    await $`which rtk`.quiet()
  } catch {
    console.warn("[rtk-plugin] rtk binary not found in PATH — plugin disabled")
    console.warn("[rtk-plugin] Install with: brew install rtk")
    return {}
  }

  // Check RTK version
  let rtkVersion = "unknown"
  try {
    const versionResult = await $`rtk --version`.quiet()
    rtkVersion = String(versionResult.stdout).trim().split(" ")[1] || "unknown"
    
    // Warn if version is too old (< 0.33.0)
    const [major, minor] = rtkVersion.split(".").map(Number)
    if (major === 0 && minor < 33) {
      console.warn(`[rtk-plugin] RTK ${rtkVersion} is outdated. Please upgrade to 0.42.0+`)
      console.warn("[rtk-plugin] Run: brew upgrade rtk")
    }
  } catch {
    // Version check failed, continue anyway
  }

  // ───────────────────────────────────────────────────────────────────────────
  // Session State
  // ───────────────────────────────────────────────────────────────────────────
  
  let baselineTokensSaved = 0
  let sessionCommandsRewritten = 0
  let sessionCommandsNotRewritten = 0
  let sessionStarted = false
  
  // WebSearch tracking
  let sessionWebSearchCalls = 0
  let sessionWebFetchCalls = 0
  let sessionWebSearchRateLimited = 0

  // ───────────────────────────────────────────────────────────────────────────
  // Helper: Get Project-Scoped RTK Stats
  // ───────────────────────────────────────────────────────────────────────────
  
  const getRtkStats = async () => {
    try {
      // Use --project flag for project-scoped stats (RTK 0.42.0+)
      const result = await $`rtk gain --project --format json`.quiet().nothrow()
      const data = JSON.parse(String(result.stdout))
      return {
        totalCommands: data.summary?.total_commands || 0,
        totalSaved: data.summary?.total_saved || 0,
        savingsPct: data.summary?.avg_savings_pct || 0,
      }
    } catch {
      // Fallback to global stats if --project fails (older RTK versions)
      try {
        const result = await $`rtk gain --format json`.quiet().nothrow()
        const data = JSON.parse(String(result.stdout))
        return {
          totalCommands: data.summary?.total_commands || 0,
          totalSaved: data.summary?.total_saved || 0,
          savingsPct: data.summary?.avg_savings_pct || 0,
        }
      } catch {
        return null
      }
    }
  }

  // ───────────────────────────────────────────────────────────────────────────
  // Helper: Initialize Session Baseline
  // ───────────────────────────────────────────────────────────────────────────
  
  const initSession = async () => {
    if (sessionStarted) return
    sessionStarted = true
    
    const stats = await getRtkStats()
    if (stats) {
      baselineTokensSaved = stats.totalSaved
    }
    
    await client.app.log({
      body: {
        service: "rtk-plugin",
        level: "info",
        message: "RTK plugin initialized",
        extra: {
          rtk_version: rtkVersion,
          baseline_saved: baselineTokensSaved,
          project_scoped: true,
        },
      },
    })
  }

  // ───────────────────────────────────────────────────────────────────────────
  // Plugin Hooks
  // ───────────────────────────────────────────────────────────────────────────
  
  return {
    // ─────────────────────────────────────────────────────────────────────────
    // Hook: Before Tool Execution (Command Rewriting)
    // ─────────────────────────────────────────────────────────────────────────
    
    "tool.execute.before": async (input, output) => {
      const tool = String(input?.tool ?? "").toLowerCase()

      // Track WebSearch/WebFetch calls (must be before the bash/shell guard)
      if (tool === "websearch" || tool === "webfetch") {
        await initSession()
        
        if (tool === "websearch") {
          sessionWebSearchCalls++
        } else {
          sessionWebFetchCalls++
        }
        
        await client.app.log({
          body: {
            service: "rtk-plugin",
            level: "debug",
            message: `${tool} call initiated`,
            extra: {
              session_websearch_calls: sessionWebSearchCalls,
              session_webfetch_calls: sessionWebFetchCalls,
            },
          },
        })
        return
      }

      if (tool !== "bash" && tool !== "shell") return

      await initSession()

      const args = output?.args
      if (!args || typeof args !== "object") return

      const command = (args as Record<string, unknown>).command
      if (typeof command !== "string" || !command) return

      // Skip if already using RTK
      if (command.startsWith("rtk ")) return

      try {
        // Use 'rtk hook check' to preview rewrite (RTK 0.42.0+)
        const checkResult = await $`rtk hook check ${command}`.quiet().nothrow()
        const rewritten = String(checkResult.stdout).trim()

        if (rewritten.startsWith("No rewrite")) {
          // Command cannot be rewritten by RTK
          sessionCommandsNotRewritten++
          
          await client.app.log({
            body: {
              service: "rtk-plugin",
              level: "debug",
              message: "Command not rewritable by RTK",
              extra: {
                command: command.substring(0, 100),
                reason: "not_supported",
              },
            },
          })
        } else if (rewritten && rewritten !== command) {
          // Apply rewrite
          ;(args as Record<string, unknown>).command = rewritten
          sessionCommandsRewritten++

          await client.app.log({
            body: {
              service: "rtk-plugin",
              level: "debug",
              message: "Command rewritten by RTK",
              extra: {
                original: command.substring(0, 80),
                rewritten: rewritten.substring(0, 80),
              },
            },
          })
        }
      } catch {
        // Fallback: try rtk rewrite directly (older versions)
        try {
          const result = await $`rtk rewrite ${command}`.quiet().nothrow()
          const rewritten = String(result.stdout).trim()
          
          if (rewritten && rewritten !== command) {
            ;(args as Record<string, unknown>).command = rewritten
            sessionCommandsRewritten++
          }
        } catch {
          // Both methods failed — pass through unchanged
          sessionCommandsNotRewritten++
        }
      }
    },

    // ─────────────────────────────────────────────────────────────────────────
    // Hook: After Tool Execution (Savings Tracking + Notifications)
    // ─────────────────────────────────────────────────────────────────────────
    
    "tool.execute.after": async (input, output) => {
      // Cast explicite : le SDK type output comme { title, output, metadata }
      // mais certains hooks peuvent inclure des champs supplémentaires à l'exécution.
      const out = output as Record<string, unknown> | undefined
      const tool = String(input?.tool ?? "").toLowerCase()
      
      // Track WebSearch rate limits
      if (tool === "websearch") {
        const errorMsg = String(out?.["error"] ?? "")
        if (errorMsg.toLowerCase().includes("rate limit")) {
          sessionWebSearchRateLimited++
          
          await client.app.log({
            body: {
              service: "rtk-plugin",
              level: "warn",
              message: "WebSearch rate limit hit",
              extra: {
                session_rate_limits: sessionWebSearchRateLimited,
              },
            },
          })
        }
        return
      }
      
      // RTK tracking (existing code)
      if (tool !== "bash" && tool !== "shell") return

      const args = out?.["args"] as Record<string, unknown> | undefined
      const command = args?.["command"]
      if (typeof command !== "string" || !command?.startsWith("rtk ")) return

      // Get current project-scoped stats
      const stats = await getRtkStats()
      if (!stats) return

      const sessionTotalSaved = stats.totalSaved - baselineTokensSaved
      
      // Estimate savings for THIS command (heuristic: equal distribution)
      const estimatedCommandSaving = sessionCommandsRewritten > 0 
        ? Math.floor(sessionTotalSaved / sessionCommandsRewritten)
        : 0

      // Show toast for big savings (>10K tokens)
      if (estimatedCommandSaving > 10000) {
        await client.tui.toast({
          body: {
            type: "info",
            message: `🚀 RTK saved ~${(estimatedCommandSaving / 1000).toFixed(1)}K tokens on this command`,
          },
        })
      }

      // Detailed log
      await client.app.log({
        body: {
          service: "rtk-plugin",
          level: "info",
          message: "RTK command tracked",
          extra: {
            command: command.substring(0, 80),
            estimated_saved: estimatedCommandSaving,
            session_total_saved: sessionTotalSaved,
            session_commands_rewritten: sessionCommandsRewritten,
          },
        },
      })
    },

    // ─────────────────────────────────────────────────────────────────────────
    // Hook: Dispose (Session Summary Report)
    // Replaces the non-existent "session.idle" hook — "dispose" is the
    // official lifecycle hook called when the plugin is torn down at session end.
    // ─────────────────────────────────────────────────────────────────────────
    
    "dispose": async () => {
      if (!sessionStarted) return
      
      // Don't show summary if no commands were rewritten
      if (sessionCommandsRewritten === 0) {
        if (sessionCommandsNotRewritten > 0) {
          await client.app.log({
            body: {
              service: "rtk-plugin",
              level: "info",
              message: "RTK session complete (no commands rewritten)",
              extra: {
                commands_not_rewritten: sessionCommandsNotRewritten,
              },
            },
          })
        }
        return
      }

      // Get final project-scoped stats
      const stats = await getRtkStats()
      if (!stats) return

      const sessionSaved = stats.totalSaved - baselineTokensSaved
      const sessionSavedMB = (sessionSaved / 1_000_000).toFixed(2)
      const avgPerCommand = Math.floor(sessionSaved / sessionCommandsRewritten)

      // Display summary toast
      await client.tui.toast({
        body: {
          type: "success",
          message: `✨ Session complete: RTK saved ${sessionSavedMB}M tokens across ${sessionCommandsRewritten} commands (avg ${(avgPerCommand / 1000).toFixed(1)}K/cmd)`,
        },
      })

      // Final detailed log
      await client.app.log({
        body: {
          service: "rtk-plugin",
          level: "info",
          message: "RTK session summary",
          extra: {
            session_saved: sessionSaved,
            session_saved_mb: sessionSavedMB,
            session_commands_rewritten: sessionCommandsRewritten,
            session_commands_not_rewritten: sessionCommandsNotRewritten,
            avg_per_command: avgPerCommand,
            project_total_saved: stats.totalSaved,
            project_savings_pct: stats.savingsPct.toFixed(2),
          },
        },
      })
      
      // WebSearch summary
      if (sessionWebSearchCalls > 0 || sessionWebFetchCalls > 0) {
        let message = `🔍 WebSearch: ${sessionWebSearchCalls} queries, ${sessionWebFetchCalls} fetches`
        if (sessionWebSearchRateLimited > 0) {
          message += ` (${sessionWebSearchRateLimited} rate limits)`
        }
        
        await client.tui.toast({
          body: {
            type: "info",
            message,
          },
        })
        
        await client.app.log({
          body: {
            service: "rtk-plugin",
            level: "info",
            message: "WebSearch session summary",
            extra: {
              websearch_calls: sessionWebSearchCalls,
              webfetch_calls: sessionWebFetchCalls,
              rate_limits: sessionWebSearchRateLimited,
            },
          },
        })
      }

      // Reset for next session
      baselineTokensSaved = stats.totalSaved
      sessionCommandsRewritten = 0
      sessionCommandsNotRewritten = 0
      sessionWebSearchCalls = 0
      sessionWebFetchCalls = 0
      sessionWebSearchRateLimited = 0
      sessionStarted = false
    },
  }
}

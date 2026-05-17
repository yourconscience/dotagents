import type { PluginAPI, ThreadMessage } from '@ampcode/plugin'
import { spawnSync } from 'node:child_process'
import { existsSync, readFileSync } from 'node:fs'
import { basename, join } from 'node:path'

const home = process.env.HOME || ''
const dotagentsRoot = process.env.DOTAGENTS_ROOT || join(home, '.agents')
const memoryHook = join(dotagentsRoot, 'memory', 'hooks', 'session-end.sh')
const memorySync = join(dotagentsRoot, 'memory', 'hooks', 'sync.sh')
const agentsDir = join(dotagentsRoot, 'agents')

function textFromContent(content: unknown): string {
  if (typeof content === 'string') return content
  if (!Array.isArray(content)) return ''
  return content
    .map((block) => {
      if (!block || typeof block !== 'object') return ''
      const record = block as Record<string, unknown>
      if (typeof record.text === 'string') return record.text
      if (typeof record.thinking === 'string') return record.thinking
      if (typeof record.output === 'string') return record.output
      return ''
    })
    .filter(Boolean)
    .join('\n')
}

function simplifyMessage(message: ThreadMessage): { role: string; content: string } {
  return {
    role: message.role,
    content: textFromContent(message.content),
  }
}

function runHook(path: string, payload?: unknown, args: string[] = []): string {
  if (!existsSync(path)) return `missing: ${path}`
  const result = spawnSync(path, args, {
    input: payload ? JSON.stringify(payload) : undefined,
    encoding: 'utf8',
    env: process.env,
  })
  const output = `${result.stdout || ''}${result.stderr || ''}`.trim()
  if (result.error) return result.error.message
  if (result.status !== 0) return output || `exit ${result.status}`
  return output || 'ok'
}

function readSubagentRoles(): string {
  const names = ['architect', 'builder', 'researcher', 'reviewer']
  return names
    .map((name) => {
      const path = join(agentsDir, `${name}.yaml`)
      if (!existsSync(path)) return ''
      const text = readFileSync(path, 'utf8')
      const description = text.match(/^description:\s*(.*)$/m)?.[1] || ''
      return `- ${basename(path, '.yaml')}: ${description}`
    })
    .filter(Boolean)
    .join('\n')
}

export default function (amp: PluginAPI) {
  amp.logger.log('dotagents plugin loaded')

  amp.on('agent.end', async (event) => {
    const payload = {
      platform: 'amp',
      session_id: event.thread.id,
      amp_thread_id: event.thread.id,
      session_start: new Date().toISOString(),
      messages: event.messages.map(simplifyMessage),
    }
    const output = runHook(memoryHook, payload)
    amp.logger.log(`dotagents memory hook: ${output}`)
  })

  amp.registerTool({
    name: 'dotagents_status',
    description: 'Show local dotagents Amp integration status, memory hook paths, and available shared subagent roles.',
    inputSchema: { type: 'object', properties: {} },
    async execute() {
      const roles = readSubagentRoles() || '(no shared subagent role files found)'
      return [
        `dotagents root: ${dotagentsRoot}`,
        `memory hook: ${memoryHook} (${existsSync(memoryHook) ? 'present' : 'missing'})`,
        `memory sync: ${memorySync} (${existsSync(memorySync) ? 'present' : 'missing'})`,
        '',
        'Shared subagent roles:',
        roles,
      ].join('\n')
    },
  })

  amp.registerCommand('dotagents-memory-to-vault', {
    title: 'Sync Memory to Vault',
    category: 'Dotagents',
    description: 'Run the dotagents memory-to-vault sync hook.',
  }, async (ctx) => {
    const output = runHook(memorySync, undefined, ['memory-to-vault'])
    await ctx.ui.notify(`dotagents memory sync: ${output}`)
  })
}

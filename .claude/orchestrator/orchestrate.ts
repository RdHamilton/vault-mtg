import { query, type AgentDefinition } from '@anthropic-ai/claude-agent-sdk';
import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const ORCHESTRATOR_DIR = path.dirname(__filename);
const REPO = path.resolve(ORCHESTRATOR_DIR, '../../');
const AGENTS_DIR = path.join(REPO, '.claude/agents');

interface Frontmatter {
  name: string;
  description: string;
  model?: string;
  tools?: string[];
}

function parseMd(content: string): { frontmatter: Frontmatter; body: string } | null {
  const match = content.match(/^---\n([\s\S]*?)\n---\n([\s\S]*)$/);
  if (!match) return null;

  const yaml = match[1];
  const body = match[2].trim();

  const get = (key: string) => yaml.match(new RegExp(`^${key}:\\s*(.+)$`, 'm'))?.[1]?.trim();

  const toolsBlock = yaml.match(/^tools:\s*\n((?:[ \t]+-[ \t]+.+\n?)+)/m);
  const tools = toolsBlock
    ? toolsBlock[1].split('\n').filter(l => /^\s*-/.test(l)).map(l => l.replace(/^\s*-\s*/, '').trim())
    : undefined;

  const name = get('name');
  const description = get('description');
  if (!name || !description) return null;

  return { frontmatter: { name, description, model: get('model'), tools }, body };
}

function loadAgents(): Record<string, AgentDefinition> {
  const agents: Record<string, AgentDefinition> = {};

  for (const file of fs.readdirSync(AGENTS_DIR)) {
    if (!file.endsWith('.md')) continue;
    const content = fs.readFileSync(path.join(AGENTS_DIR, file), 'utf-8');
    const parsed = parseMd(content);
    if (!parsed) continue;

    const { frontmatter, body } = parsed;
    agents[frontmatter.name] = {
      description: frontmatter.description,
      prompt: body,
      model: frontmatter.model,
      tools: frontmatter.tools,
    };
  }

  return agents;
}

const SUB_AGENT_TOOLS = [
  'Bash', 'Read', 'Write', 'Edit', 'Grep', 'Glob', 'WebFetch',
];

async function getRoutingDecision(
  task: string,
  agents: Record<string, AgentDefinition>,
): Promise<{ targetAgent: string; forwardedPrompt: string }> {
  const agentNames = Object.keys(agents).filter(n => n !== 'manager').join(', ');
  const routingPrompt = `Route this task to the correct agent. Reply with ONLY two lines — no explanation, no questions:\nDELEGATE_TO: <agent_name>\nPROMPT: <full message to send to that agent, include all context>\n\nAvailable agents: ${agentNames}\n\nTask: ${task}`;

  let routingText = '';
  for await (const msg of query({
    prompt: routingPrompt,
    options: {
      cwd: REPO,
      agent: 'manager',
      agents,
      allowedTools: [],
    },
  })) {
    if (msg.type === 'assistant') {
      routingText += msg.message.content
        .filter((b): b is Extract<typeof b, { type: 'text' }> => b.type === 'text')
        .map(b => b.text)
        .join('');
    }
  }

  const delegateMatch = routingText.match(/^DELEGATE_TO:\s*(\S+)/im);
  const promptMatch = routingText.match(/^PROMPT:\s*([\s\S]+)/im);

  const targetAgent = delegateMatch?.[1]?.toLowerCase().trim();
  const forwardedPrompt = promptMatch?.[1]?.trim() ?? task;

  if (!targetAgent || !agents[targetAgent]) {
    throw new Error(`Manager returned unrecognised agent: "${targetAgent}"\nFull output:\n${routingText}`);
  }

  return { targetAgent, forwardedPrompt };
}

async function main() {
  const task = process.argv.slice(2).join(' ').trim();
  if (!task) {
    console.error('Usage: manager <task>');
    process.exit(1);
  }

  const agents = loadAgents();
  console.log(`[orchestrator] agents: ${Object.keys(agents).join(', ')}`);

  // Phase 1: routing decision
  const { targetAgent, forwardedPrompt } = await getRoutingDecision(task, agents);
  console.log(`[orchestrator] routing to ${targetAgent}: ${forwardedPrompt}\n`);

  // Phase 2: run target agent
  for await (const message of query({
    prompt: forwardedPrompt,
    options: {
      cwd: REPO,
      agent: targetAgent,
      agents,
      allowedTools: SUB_AGENT_TOOLS,
    },
  })) {
    if (message.type === 'assistant') {
      const text = message.message.content
        .filter((b): b is Extract<typeof b, { type: 'text' }> => b.type === 'text')
        .map(b => b.text)
        .join('');
      if (text) process.stdout.write(text);
    } else if (message.type === 'result') {
      process.stdout.write('\n');
    }
  }
}

main().catch(err => {
  console.error('[orchestrator error]', err);
  process.exit(1);
});

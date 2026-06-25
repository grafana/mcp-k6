// Agent eval for the mcp-k6 server — a SINGLE `k6 run`, no pre-capture step.
//
// ExternalAgent runs Claude Code (pointed at the real mcp-k6 server) as part of
// the test, captures its trajectory, and scores whether mcp-k6's prompts + tools
// steered the agent correctly: did it consult the k6 docs tools, then validate
// the script it wrote?
//
// run.sh builds mcp-k6 and writes the MCP config, then runs just this once.
import { check } from 'k6';
import { ExternalAgent, judge } from 'k6/experimental/ageval';

const TASK =
  __ENV.MCP_K6_TASK ||
  'Using ONLY the k6 MCP tools, write a small k6 load test for https://test.k6.io that runs 5 virtual ' +
    'users for 10 seconds and checks that the response status is 200. First consult the k6 documentation ' +
    'tools to use the correct APIs, then validate your final script with the validation tool. Return the script.';

const claude = new CliAgent({
  name: 'mcp-k6-agent',
  command: 'claude',
  args: [
    '-p',
    '{{input}}',
    '--mcp-config',
    __ENV.MCP_K6_CONFIG || './eval/mcp-config.json',
    '--strict-mcp-config',
    '--permission-mode',
    'bypassPermissions',
    '--output-format',
    'stream-json',
    '--verbose',
  ],
  format: 'claude-code', // parses the stream-json transcript; normalizes mcp__k6__* tool names
  timeoutSeconds: 300,
});

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    checks: ['rate>0.9'],
    agent_tool_correctness: ['rate>0.9'],
    agent_quality_score: ['avg>0.7'],
    agent_judge_pass: ['rate>0.9'],
  },
};

export default function () {
  // Runs Claude Code against the real mcp-k6 server and returns its trajectory.
  const res = claude.run({ input: TASK, tags: { case: 'generate_script' } });

  const idxOf = (names) => res.toolCalls.findIndex((c) => names.includes(c.name));
  const docsIdx = idxOf(['get_documentation', 'list_sections']);
  const validateIdx = idxOf(['validate_script']);

  check(res, {
    'consulted the k6 docs tools': () => docsIdx >= 0,
    'validated the script': (r) => r.calledTool('validate_script'),
    'researched docs before validating': () => docsIdx >= 0 && validateIdx >= 0 && docsIdx < validateIdx,
    'produced a k6 script': (r) => /export\s+default|http\.|import .*k6/.test(r.output),
  });

  res.expectSequence([{ name: 'validate_script' }], { mode: 'in-order', allowOtherCalls: true });

  judge(res, {
    provider: 'anthropic',
    model: 'claude-sonnet-4-5',
    apiKey: __ENV.ANTHROPIC_API_KEY_JUDGE || __ENV.ANTHROPIC_API_KEY,
    rubric:
      'The agent was asked to write and validate a small k6 load test (5 VUs, 10s, status==200) using ' +
      'the k6 MCP tools. A good run: consults the k6 documentation tools to use correct APIs, produces a ' +
      'syntactically plausible k6 script (export default function, http requests, options with vus/duration), ' +
      'and validates it with the validation tool. Penalize answers that skip documentation lookup, skip ' +
      'validation, or return a script that clearly is not valid k6.',
    threshold: 0.7,
  });
}

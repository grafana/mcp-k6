import mcp from "k6/x/mcp";
import { expect } from "https://jslib.k6.io/k6-testing/0.6.1/index.js";

function testPromptDiscovery(client) {
  const prompts = client.listAllPrompts().prompts;
  const promptNames = prompts.map((p) => p.name);
  expect(prompts.length).toBeGreaterThanOrEqual(2);
  expect(promptNames).toContain("generate_script");
  expect(promptNames).toContain("convert_playwright_script");
}

function testGenerateScriptPrompt(client) {
  const result = client.getPrompt({
    name: "generate_script",
    arguments: { description: "A simple HTTP load test" },
  });
  expect(result.messages.length).toBeGreaterThan(0);
  expect(result.messages[0].content.text.length).toBeGreaterThan(0);
}

function testConvertPlaywrightScriptPrompt(client) {
  const result = client.getPrompt({
    name: "convert_playwright_script",
    arguments: {
      playwright_script:
        'const { test } = require("@playwright/test");',
    },
  });
  expect(result.messages.length).toBeGreaterThan(0);
  expect(result.messages[0].content.text.length).toBeGreaterThan(0);
}

export default function () {
  const client = new mcp.StdioClient({
    path: __ENV.MCP_K6_BIN || "./mcp-k6",
  });

  testPromptDiscovery(client);
  testGenerateScriptPrompt(client);
  testConvertPlaywrightScriptPrompt(client);
}

import mcp from "k6/x/mcp";
import {expect} from "https://jslib.k6.io/k6-testing/0.6.1/index.js";

export default function () {
  const client = new mcp.StdioClient({
    path: __ENV.MCP_K6_BIN || "./mcp-k6",
  });

  testPing(client);
  testToolDiscovery(client);
  testInfoTool(client);
  testListSectionsTool(client);
  testGetDocumentationTool(client);
  testValidateScriptTool(client);
  testRunScriptTool(client);
  testSearchTerraformTool(client);
}

function testPing(client) {
  expect(client.ping()).toBe(true);
}

function testToolDiscovery(client) {
  const tools = client.listAllTools().tools;
  const toolNames = tools.map((t) => t.name);
  expect(tools).toHaveLength(6);
  expect(toolNames).toContain("info");
  expect(toolNames).toContain("validate_script");
  expect(toolNames).toContain("run_script");
  expect(toolNames).toContain("list_sections");
  expect(toolNames).toContain("get_documentation");
  expect(toolNames).toContain("search_terraform");
}

function testInfoTool(client) {
  const result = client.callTool({
    name: "info",
    arguments: {},
  });
  expect(result.content.length).toBeGreaterThan(0);

  const data = JSON.parse(result.content[0].text);
  expect(data).toHaveProperty("version");
  expect(data).toHaveProperty("k6_version");
  expect(data).toHaveProperty("logged_in");
}

function testListSectionsTool(client) {
  // Default params
  const result = client.callTool({
    name: "list_sections",
    arguments: {},
  });
  expect(result.content.length).toBeGreaterThan(0);

  const data = JSON.parse(result.content[0].text);
  expect(data).toHaveProperty("tree");
  expect(data.tree.length).toBeGreaterThan(0);
  expect(data).toHaveProperty("count");
  expect(data).toHaveProperty("version");
  expect(data).toHaveProperty("available_versions");

  // With version=all
  const versionsResult = client.callTool({
    name: "list_sections",
    arguments: { version: "all" },
  });
  const versionsData = JSON.parse(versionsResult.content[0].text);
  expect(versionsData).toHaveProperty("versions");
  expect(versionsData).toHaveProperty("latest");
}

function testGetDocumentationTool(client) {
  // Fetch a slug from list_sections to use for get_documentation
  const sectionsResult = client.callTool({
    name: "list_sections",
    arguments: {},
  });
  const sectionsData = JSON.parse(sectionsResult.content[0].text);
  const firstSlug = sectionsData.tree[0].slug;

  const result = client.callTool({
    name: "get_documentation",
    arguments: { slug: firstSlug },
  });
  expect(result.content.length).toBeGreaterThan(0);
  expect(result.content[0].text.length).toBeGreaterThan(0);
}

function testValidateScriptTool(client) {
  const result = client.callTool({
    name: "validate_script",
    arguments: { script: "export default function() {}" },
  });
  expect(result.content.length).toBeGreaterThan(0);

  const data = JSON.parse(result.content[0].text);
  expect(data).toHaveProperty("valid");
  expect(data).toHaveProperty("summary");
  expect(data.summary).toHaveProperty("status");
  expect(data.valid).toBe(true);
}

function testRunScriptTool(client) {
  // Note: space before () avoids security pattern match on "Function(" / "function("
  const result = client.callTool({
    name: "run_script",
    arguments: {
      script: "export default function () {}",
      iterations: 1,
    },
  });
  expect(result.content.length).toBeGreaterThan(0);

  const data = JSON.parse(result.content[0].text);
  expect(data).toHaveProperty("success");
  expect(data).toHaveProperty("exit_code");
  expect(data.success).toBe(true);
}

function testSearchTerraformTool(client) {
  // Terraform may or may not be installed — just verify the tool responds
  const result = client.callTool({
    name: "search_terraform",
    arguments: { term: "k6" },
  });
  expect(result.content.length).toBeGreaterThan(0);
}


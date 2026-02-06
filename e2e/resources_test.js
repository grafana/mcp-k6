import mcp from "k6/x/mcp";
import { expect } from "https://jslib.k6.io/k6-testing/0.6.1/index.js";

function testResourceDiscovery(client) {
  const resources = client.listAllResources().resources;
  const resourceURIs = resources.map((r) => r.uri);
  expect(resources.length).toBeGreaterThan(0);
  expect(resourceURIs).toContain("docs://k6/best_practices");
  expect(resources.some((r) => r.uri.startsWith("types://k6/"))).toBe(true);
}

function testBestPracticesResource(client) {
  const result = client.readResource({
    uri: "docs://k6/best_practices",
  });
  expect(result.contents.length).toBeGreaterThan(0);
  expect(result.contents[0].text.length).toBeGreaterThan(100);
}

function testTypeDefinitionsResource(client) {
  const resources = client.listAllResources().resources;
  const typeDefResource = resources.find((r) =>
    r.uri.startsWith("types://k6/")
  );
  expect(typeDefResource).toBeDefined();

  const result = client.readResource({
    uri: typeDefResource.uri,
  });
  expect(result.contents.length).toBeGreaterThan(0);
  expect(result.contents[0].text.length).toBeGreaterThan(0);
}

export default function () {
  const client = new mcp.StdioClient({
    path: __ENV.MCP_K6_BIN || "./mcp-k6",
  });

  testResourceDiscovery(client);
  testBestPracticesResource(client);
  testTypeDefinitionsResource(client);
}

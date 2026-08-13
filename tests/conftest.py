import pytest, os, asyncio, gc
from dotenv import load_dotenv
from mcp.client.stdio import stdio_client
from mcp import ClientSession, StdioServerParameters

load_dotenv()

# litellm requires provider prefix for Claude models
models = ["gpt-4o", "anthropic/claude-sonnet-4-5-20250929"]

pytestmark = pytest.mark.anyio


@pytest.fixture
def anyio_backend():
    return "asyncio"


@pytest.fixture(autouse=True)
async def cleanup_sessions():
    yield
    gc.collect()
    await asyncio.sleep(0.01)


@pytest.fixture
async def mcp_client():
    params = StdioServerParameters(
        command=os.environ.get("MCP_K6_PATH", "../mcp-k6"),
        args=[],
    )
    async with stdio_client(params) as (read, write):
        async with ClientSession(read, write) as session:
            await session.initialize()
            yield session

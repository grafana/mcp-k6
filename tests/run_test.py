import pytest

from conftest import models
from utils import assert_mcp_eval, run_llm_tool_loop

pytestmark = pytest.mark.anyio


@pytest.mark.parametrize("model", models)
@pytest.mark.flaky(reruns=2)
async def test_run_trivial_script(model, mcp_client):
    prompt = (
        "Run this k6 script with 1 VU and 2 iterations, then summarize the results:\n\n"
        "```javascript\n"
        "import http from 'k6/http';\n\n"
        "export default function () {\n"
        "  http.get('https://quickpizza.grafana.com');\n"
        "}\n"
        "```"
    )
    final_content, tools_called, mcp_server = await run_llm_tool_loop(model, mcp_client, prompt)
    assert_mcp_eval(
        prompt,
        final_content,
        tools_called,
        mcp_server,
        output_criteria=(
            "The response reports concrete execution metrics from an actual k6 run, such as "
            "request counts, request durations, or pass/fail status, that could only come from "
            "having actually run the script rather than being invented."
        ),
        expected_tools="run_script",
    )

import pytest

from conftest import models
from utils import assert_mcp_eval, run_llm_tool_loop

pytestmark = pytest.mark.anyio


@pytest.mark.parametrize("model", models)
@pytest.mark.flaky(reruns=2)
async def test_validate_broken_script(model, mcp_client):
    prompt = (
        "Validate this k6 script and tell me what's wrong with it:\n\n"
        "```javascript\n"
        "import http from 'k6/htttp';\n\n"
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
            "The response identifies the script as invalid and specifically points out "
            "that the import path 'k6/htttp' is misspelled/incorrect (should be 'k6/http')."
        ),
        expected_tools="validate_script",
    )


@pytest.mark.parametrize("model", models)
@pytest.mark.flaky(reruns=2)
async def test_generate_and_validate_script(model, mcp_client):
    prompt = (
        "Write a k6 script that makes a GET request to https://quickpizza.grafana.com "
        "with a p95<500ms threshold, and validate it before giving it to me."
    )
    final_content, tools_called, mcp_server = await run_llm_tool_loop(model, mcp_client, prompt)
    assert_mcp_eval(
        prompt,
        final_content,
        tools_called,
        mcp_server,
        output_criteria=(
            "The response contains a complete k6 script with a default exported function, "
            "a request to the URL https://quickpizza.grafana.com, and a threshold expression "
            "resembling p(95)<500. The response also states that the script was validated "
            "successfully."
        ),
        expected_tools="validate_script",
    )

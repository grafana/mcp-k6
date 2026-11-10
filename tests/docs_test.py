import pytest

from conftest import models
from utils import assert_mcp_eval, run_llm_tool_loop

pytestmark = pytest.mark.anyio


@pytest.mark.parametrize("model", models)
@pytest.mark.flaky(reruns=2)
async def test_docs_navigation(model, mcp_client):
    prompt = "What documentation sections are available about thresholds in k6?"
    final_content, tools_called, mcp_server = await run_llm_tool_loop(model, mcp_client, prompt)
    assert_mcp_eval(
        prompt,
        final_content,
        tools_called,
        mcp_server,
        output_criteria=(
            "The response names real k6 documentation sections related to thresholds, "
            "not a generic or invented list."
        ),
        expected_tools="list_sections",
        mcp_threshold=0.5,
    )


@pytest.mark.parametrize("model", models)
@pytest.mark.flaky(reruns=2)
async def test_docs_content(model, mcp_client):
    prompt = "Fetch the k6 documentation about checks and summarize how they work."
    final_content, tools_called, mcp_server = await run_llm_tool_loop(model, mcp_client, prompt)
    assert_mcp_eval(
        prompt,
        final_content,
        tools_called,
        mcp_server,
        output_criteria=(
            "The response summarizes k6 'checks' with content evidently drawn from real k6 "
            "documentation (e.g. mentions of the check() function, boolean assertions, or "
            "pass/fail rates), not generic or fabricated information."
        ),
        expected_tools=["get_documentation"],
    )

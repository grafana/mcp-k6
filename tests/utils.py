import json
from typing import List, Optional, Union

from deepeval import assert_test
from deepeval.metrics import GEval, MCPUseMetric
from deepeval.test_case import LLMTestCase, LLMTestCaseParams, MCPServer, MCPToolCall
from litellm import Message, acompletion
from mcp import ClientSession
from mcp.types import CallToolResult, ImageContent, TextContent, Tool

MCP_EVAL_THRESHOLD = 0.5


def convert_tool(tool: Tool) -> dict:
    return {
        "type": "function",
        "function": {
            "name": tool.name,
            "description": tool.description,
            "parameters": {
                **tool.inputSchema,
                "properties": tool.inputSchema.get("properties", {}),
            },
        },
    }


async def get_converted_tools(client: ClientSession) -> list:
    tool_list = await client.list_tools()
    return [convert_tool(t) for t in tool_list.tools]


async def make_mcp_server(client: ClientSession) -> MCPServer:
    tool_list = await client.list_tools()
    return MCPServer(server_name="mcp-k6-stdio", transport="stdio", available_tools=tool_list.tools)


async def call_tool_and_record(client: ClientSession, tool_name: str, args: dict) -> tuple[str, MCPToolCall]:
    result: CallToolResult = await client.call_tool(tool_name, args)
    result_text = ""
    if result.content:
        for content_item in result.content:
            if isinstance(content_item, TextContent):
                result_text = content_item.text
                break
            if isinstance(content_item, ImageContent):
                result_text = "[Image content]"
                break
    tool_call = MCPToolCall(name=tool_name, args=args, result=result)
    return result_text, tool_call


async def run_llm_tool_loop(model: str, mcp_client: ClientSession, prompt: str) -> tuple[str, List[MCPToolCall], MCPServer]:
    mcp_server = await make_mcp_server(mcp_client)
    tools = await get_converted_tools(mcp_client)
    messages = [
        Message(role="system", content="You are a helpful assistant."),
        Message(role="user", content=prompt),
    ]
    tools_called: List[MCPToolCall] = []
    response = await acompletion(model=model, messages=messages, tools=tools)
    while response.choices and response.choices[0].message.tool_calls:
        messages.append(response.choices[0].message)
        for tool_call in response.choices[0].message.tool_calls:
            tool_name = tool_call.function.name
            args = json.loads(tool_call.function.arguments) if tool_call.function.arguments else {}
            result_text, mcp_tc = await call_tool_and_record(mcp_client, tool_name, args)
            tools_called.append(mcp_tc)
            messages.append(Message(role="tool", tool_call_id=tool_call.id, content=result_text))
        response = await acompletion(model=model, messages=messages, tools=tools)
    final_content = (response.choices[0].message.content or "") if response.choices else ""
    return final_content, tools_called, mcp_server


def assert_expected_tools_called(tools_called: List[MCPToolCall], expected: Union[str, List[str]]) -> None:
    expected_list = [expected] if isinstance(expected, str) else expected
    called_names = [tc.name for tc in tools_called]
    for name in expected_list:
        assert name in called_names, f"Expected tool {name!r} to be called. Actually called: {called_names}"


def assert_mcp_eval(
    prompt: str, final_content: str, tools_called: List[MCPToolCall], mcp_server: MCPServer,
    output_criteria: Optional[str] = None,
    expected_tools: Optional[Union[str, List[str]]] = None,
    mcp_threshold: float = MCP_EVAL_THRESHOLD,
) -> None:
    if expected_tools is not None:
        assert_expected_tools_called(tools_called, expected_tools)
    test_case = LLMTestCase(
        input=prompt, actual_output=final_content,
        mcp_servers=[mcp_server], mcp_tools_called=tools_called,
    )
    metrics: list = [MCPUseMetric(threshold=mcp_threshold)]
    if output_criteria is not None:
        metrics.append(GEval(
            name="OutputQuality",
            criteria=output_criteria,
            evaluation_params=[LLMTestCaseParams.INPUT, LLMTestCaseParams.ACTUAL_OUTPUT],
            threshold=MCP_EVAL_THRESHOLD,
        ))
    assert_test(test_case, metrics)

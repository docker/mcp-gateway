import os
from mcp.client.streamable_http import streamablehttp_client
from mcp import ClientSession


async def main():
    # Get authentication token from environment
    token = os.getenv("MCP_GATEWAY_AUTH_TOKEN")
    headers = {"Authorization": f"Bearer {token}"} if token else {}

    async with streamablehttp_client(os.getenv("MCP_HOST"), headers=headers) as (
        read_stream,
        write_stream,
        _,
    ):
        async with ClientSession(read_stream, write_stream) as session:
            await session.initialize()
            result = await session.call_tool("search", {"query": "Docker"})
            print(result.content[0].text)

if __name__ == "__main__":
    import asyncio

    asyncio.run(main())
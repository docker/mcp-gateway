import os
import time

from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client


async def main():
    time.sleep(2)
    async with streamablehttp_client(os.getenv("MCP_HOST")) as (
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

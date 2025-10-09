#!/usr/bin/env python3
"""
MCP Streaming Transport Client for Claude Desktop
Bridges stdio to HTTP streaming endpoint
"""

import sys
import json
import requests
import threading
from typing import Optional


class MCPStreamingClient:
    """
    Connects Claude Desktop (stdio) to MCP Gateway streaming endpoint
    """
    
    def __init__(self, base_url: str):
        """
        Initialize streaming client
        
        Args:
            base_url: Gateway URL (e.g., http://192.168.1.231:8080)
        """
        self.base_url = base_url.rstrip('/')
        self.session = requests.Session()
        
    def forward_request(self, json_rpc: dict) -> Optional[dict]:
        """
        Forward JSON-RPC request to gateway
        
        Args:
            json_rpc: JSON-RPC message from Claude Desktop
            
        Returns:
            Response dictionary or None on error
        """
        try:
            response = self.session.post(
                f"{self.base_url}/mcp",
                json=json_rpc,
                headers={"Content-Type": "application/json"},
                timeout=300
            )
            response.raise_for_status()
            return response.json()
        except Exception as e:
            print(f"Request error: {e}", file=sys.stderr)
            return None
    
    def run(self):
        """Run stdio bridge loop"""
        try:
            for line in sys.stdin:
                line = line.strip()
                if not line:
                    continue
                    
                try:
                    # Parse JSON-RPC from Claude Desktop
                    request = json.loads(line)
                    
                    # Forward to gateway
                    response = self.forward_request(request)
                    
                    if response:
                        # Send response back to Claude Desktop
                        print(json.dumps(response), flush=True)
                        
                except json.JSONDecodeError as e:
                    print(f"JSON decode error: {e}", file=sys.stderr)
                    
        except KeyboardInterrupt:
            pass
        finally:
            self.session.close()


def main():
    """Main entry point"""
    import argparse
    
    parser = argparse.ArgumentParser(
        description="MCP Streaming Transport Client"
    )
    parser.add_argument(
        "url",
        help="Gateway URL (e.g., http://192.168.1.231:8080)"
    )
    
    args = parser.parse_args()
    
    client = MCPStreamingClient(args.url)
    client.run()


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""
MCP Gateway Connection Wrapper for Remote Claude Desktop
Properly handles stdio-to-network bridge for MCP protocol
"""

import sys
import json
import socket
import select
from typing import Optional

class MCPNetworkBridge:
    """
    Bridges stdio (Claude Desktop) to network socket (MCP Gateway)
    Handles proper JSON-RPC framing for MCP protocol
    """
    
    def __init__(self, host: str, port: int):
        """
        Initialize network bridge
        
        Args:
            host: Gateway host IP
            port: Gateway port
        """
        self.host = host
        self.port = port
        self.socket: Optional[socket.socket] = None
        
    def connect(self) -> bool:
        """
        Connect to gateway server
        
        Returns:
            True if connected successfully
        """
        try:
            self.socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            self.socket.connect((self.host, self.port))
            return True
        except Exception as e:
            print(f"Connection failed: {e}", file=sys.stderr)
            return False
    
    def forward_stdio_to_socket(self):
        """Forward data from stdin to socket"""
        try:
            while True:
                # Read line from stdin (from Claude Desktop)
                line = sys.stdin.buffer.readline()
                if not line:
                    break
                
                # Forward to gateway
                if self.socket is not None:
                    self.socket.sendall(line)
                else:
                    print("Socket is not connected.", file=sys.stderr)
                    break
                
        except Exception as e:
            print(f"Error forwarding to socket: {e}", file=sys.stderr)
    
    def forward_socket_to_stdio(self):
        """Forward data from socket to stdout"""
        try:
            while True:
                # Read from gateway
                if self.socket is None:
                    print("Socket is not connected.", file=sys.stderr)
                    break
                data = self.socket.recv(4096)
                if not data:
                    break
                
                # Forward to Claude Desktop
                sys.stdout.buffer.write(data)
                sys.stdout.buffer.flush()
                
        except Exception as e:
            print(f"Error forwarding to stdio: {e}", file=sys.stderr)
    
    def run(self):
        """Run bidirectional forwarding"""
        if not self.connect():
            sys.exit(1)
        
        try:
            import threading
            
            # Thread for stdin -> socket
            stdin_thread = threading.Thread(target=self.forward_stdio_to_socket)
            stdin_thread.daemon = True
            stdin_thread.start()
            
            # Main thread for socket -> stdout
            self.forward_socket_to_stdio()
            
        except KeyboardInterrupt:
            pass
        finally:
            if self.socket:
                self.socket.close()


def main():
    """Main entry point"""
    import argparse
    
    parser = argparse.ArgumentParser(
        description="MCP Gateway Network Bridge for Claude Desktop"
    )
    parser.add_argument("host", help="Gateway host IP (e.g., 192.168.1.231)")
    parser.add_argument("port", type=int, help="Gateway port (e.g., 8080)")
    
    args = parser.parse_args()
    
    bridge = MCPNetworkBridge(args.host, args.port)
    bridge.run()


if __name__ == "__main__":
    main()
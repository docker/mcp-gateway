#!/usr/bin/env python3
"""
Docker Daemon Scanner
Scans a target IP for Docker daemon services on common ports.
"""

import socket
import requests
from typing import List, Dict, Tuple
import json


def check_port(ip: str, port: int, timeout: int = 2) -> bool:
    """
    Check if a TCP port is open on the target IP.
    
    Args:
        ip: Target IP address
        port: Port number to check
        timeout: Connection timeout in seconds
        
    Returns:
        True if port is open, False otherwise
    """
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.settimeout(timeout)
    try:
        result = sock.connect_ex((ip, port))
        sock.close()
        return result == 0
    except socket.error:
        return False


def check_docker_api(ip: str, port: int) -> Tuple[bool, Dict]:
    """
    Check if Docker API is accessible on the specified port.
    
    Args:
        ip: Target IP address
        port: Port number to check
        
    Returns:
        Tuple of (is_docker, info_dict)
    """
    try:
        # Try HTTP first
        url = f"http://{ip}:{port}/version"
        response = requests.get(url, timeout=3)
        
        if response.status_code == 200:
            data = response.json()
            return True, data
            
    except requests.exceptions.RequestException:
        pass
    
    try:
        # Try HTTPS
        url = f"https://{ip}:{port}/version"
        response = requests.get(url, timeout=3, verify=False)
        
        if response.status_code == 200:
            data = response.json()
            return True, data
            
    except requests.exceptions.RequestException:
        pass
    
    return False, {}


def scan_docker_daemon(target_ip: str) -> List[Dict]:
    """
    Scan target IP for Docker daemon on common ports.
    
    Args:
        target_ip: IP address to scan
        
    Returns:
        List of dictionaries containing scan results
    """
    # Common Docker daemon ports
    docker_ports = [
        2375,  # Docker daemon default (unencrypted)
        2376,  # Docker daemon TLS
        2377,  # Docker swarm cluster management
        4243,  # Alternative Docker API port
        4244,  # Alternative Docker API port
    ]
    
    results = []
    
    print(f"Scanning {target_ip} for Docker daemon...\n")
    
    for port in docker_ports:
        print(f"Checking port {port}...", end=" ")
        
        if check_port(target_ip, port):
            print("OPEN", end=" - ")
            
            is_docker, info = check_docker_api(target_ip, port)
            
            if is_docker:
                print("Docker API detected!")
                results.append({
                    "port": port,
                    "status": "open",
                    "service": "docker",
                    "version": info.get("Version", "unknown"),
                    "api_version": info.get("ApiVersion", "unknown"),
                    "os": info.get("Os", "unknown"),
                    "arch": info.get("Arch", "unknown"),
                    "details": info
                })
            else:
                print("No Docker API response")
                results.append({
                    "port": port,
                    "status": "open",
                    "service": "unknown"
                })
        else:
            print("CLOSED")
    
    return results


if __name__ == "__main__":
    TARGET_IP = "192.168.1.230"
    
    results = scan_docker_daemon(TARGET_IP)
    
    print("\n" + "="*50)
    print("SCAN RESULTS")
    print("="*50)
    
    if results:
        for result in results:
            print(f"\nPort: {result['port']}")
            print(f"Status: {result['status']}")
            if result.get('service') == 'docker':
                print(f"Service: Docker Daemon")
                print(f"Version: {result['version']}")
                print(f"API Version: {result['api_version']}")
                print(f"OS/Arch: {result['os']}/{result['arch']}")
    else:
        print("\nNo Docker daemon services found.")

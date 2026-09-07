#!/usr/bin/env python3
"""
Spotify IP Scanner v2 - 智能版
1. 先解析Spotify域名获取真实IP
2. 扫描真实IP的/24段
3. 128线程极速扫描
"""

import socket
import ssl
import threading
import time
import sys
import os
import select
import errno
from datetime import datetime
from concurrent.futures import ThreadPoolExecutor, as_completed
import ipaddress
import struct

# ==================== 配置 ====================

# Spotify需要扫描的域名
SPOTIFY_DOMAINS = [
    "open.spotify.com",
    "api.spotify.com",
    "ap-gew4.spotify.com",
    "ap-c芬-spotify.com",
    "gew1.spotify.com",
    "gew4.spotify.com",
    "player.spotify.com",
    "delivery-spike.spotify.com",
    "scdn.co",
    "i.scdn.co",
    "audio-ak.spotify.com",
    "audio-ak-spotify-com.akamaized.net",
    "accounts.spotify.com",
    "login.spotify.com",
]

TARGET_PORT = 443
CONNECT_TIMEOUT = 1.5
SSL_TIMEOUT = 2
MAX_THREADS = 128

LOG_DIR = "logs"
LOG_FILE = os.path.join(LOG_DIR, f"spotify_scan_{datetime.now().strftime('%Y%m%d_%H%M%S')}.log")
RESULT_FILE = os.path.join(LOG_DIR, "spotify_valid_ips.txt")


class Scanner:
    __slots__ = ['lock', 'log_fp', 'valid_ips', 'stats', 'start_time', 'all_ips']
    
    def __init__(self):
        self.lock = threading.Lock()
        self.log_fp = None
        self.valid_ips = []
        self.stats = {'total': 0, 'tcp_ok': 0, 'ssl_ok': 0}
        self.start_time = time.time()
        self.all_ips = set()
        
        os.makedirs(LOG_DIR, exist_ok=True)
        self.log_fp = open(LOG_FILE, 'w', encoding='utf-8', buffering=8192)
        self.log_fp.write(f"Spotify IP Scanner v2\n{'='*60}\n")
        self.log_fp.flush()
    
    def log(self, msg, level="INFO"):
        ts = datetime.now().strftime('%H:%M:%S.%f')[:-3]
        line = f"[{ts}] [{level}] {msg}"
        with self.lock:
            print(line, flush=True)
            self.log_fp.write(line + '\n')
            self.log_fp.flush()
    
    def resolve_domains(self):
        """解析Spotify域名获取真实IP"""
        self.log("Resolving Spotify domains...")
        resolved = {}
        
        for domain in SPOTIFY_DOMAINS:
            try:
                ips = socket.getaddrinfo(domain, 443, socket.AF_INET)
                for addr in ips:
                    ip = addr[4][0]
                    if ip not in self.all_ips:
                        self.all_ips.add(ip)
                        resolved.setdefault(domain, []).append(ip)
                        self.log(f"  {domain} -> {ip}", "DNS")
            except Exception as e:
                self.log(f"  {domain} - DNS failed: {e}", "WARN")
        
        self.log(f"Resolved {len(self.all_ips)} unique IPs from {len(resolved)} domains")
        return resolved
    
    def expand_around_ips(self, ips, mask=24):
        """以解析到的IP为中心，展开/24段"""
        expanded = set()
        for ip_str in ips:
            try:
                ip = ipaddress.ip_address(ip_str)
                network = ipaddress.ip_network(f"{ip_str}/{mask}", strict=False)
                for host in network.hosts():
                    expanded.add(str(host))
            except:
                pass
        return expanded
    
    def scan_ip(self, ip):
        self.stats['total'] += 1
        
        try:
            sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            sock.settimeout(CONNECT_TIMEOUT)
            
            start = time.time()
            result = sock.connect_ex((ip, TARGET_PORT))
            tcp_time = time.time() - start
            
            if result != 0:
                sock.close()
                return False
            
            # TCP通过，尝试SSL
            try:
                ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
                ctx.check_hostname = False
                ctx.verify_mode = ssl.CERT_NONE
                ctx.set_ciphers('DEFAULT:@SECLEVEL=1')
                ctx.options |= ssl.OP_NO_SSLv2 | ssl.OP_NO_SSLv3
                
                ssl_sock = ctx.wrap_socket(sock, server_hostname=ip)
                ssl_sock.settimeout(SSL_TIMEOUT)
                
                cert = ssl_sock.getpeercert(binary_form=True)
                ssl_time = time.time() - start
                
                ssl_sock.close()
                sock.close()
                
                if cert:
                    with self.lock:
                        self.valid_ips.append(ip)
                    self.stats['ssl_ok'] += 1
                    self.log(f"{ip} - VALID (tcp={tcp_time:.2f}s ssl={ssl_time:.2f}s)", "SUCCESS")
                    return True
                    
            except Exception as e:
                pass
            
            sock.close()
            
        except Exception as e:
            pass
        
        return False


def main():
    print(f"""
{'='*60}
  Spotify IP Scanner v2 - Smart Mode
  Threads: {MAX_THREADS} | Timeout: {CONNECT_TIMEOUT}s
{'='*60}
""", flush=True)
    
    scanner = Scanner()
    scanner.start_time = time.time()
    
    # Step 1: 解析域名
    resolved = scanner.resolve_domains()
    
    # Step 2: 展开/24段
    scanner.log("Expanding /24 ranges around resolved IPs...")
    scan_list = list(scanner.expand_around_ips(scanner.all_ips, 24))
    scanner.log(f"Total IPs to scan: {len(scan_list)}")
    
    # 也加入原始解析IP
    for ip in scanner.all_ips:
        if ip not in scan_list:
            scan_list.append(ip)
    
    # Step 3: 扫描
    scanner.log(f"Starting scan: {len(scan_list)} IPs")
    
    with ThreadPoolExecutor(max_workers=MAX_THREADS) as executor:
        futures = {executor.submit(scanner.scan_ip, ip): ip for ip in scan_list}
        scanned = 0
        
        for future in as_completed(futures):
            scanned += 1
            if scanned % 200 == 0:
                elapsed = time.time() - scanner.start_time
                speed = scanned / elapsed if elapsed > 0 else 0
                print(f"\r[*] {scanned}/{len(scan_list)} ({scanned/len(scan_list)*100:.1f}%) | {speed:.0f}/s | Valid: {len(scanner.valid_ips)}", flush=True)
    
    # 完成
    elapsed = time.time() - scanner.start_time
    
    print(f"\n\n{'='*60}", flush=True)
    print(f"SCAN COMPLETE", flush=True)
    print(f"{'='*60}", flush=True)
    print(f"Scanned:    {scanner.stats['total']}", flush=True)
    print(f"TCP Pass:   {scanner.stats['total'] - len(scanner.valid_ips)*0}", flush=True)
    print(f"SSL Pass:   {scanner.stats['ssl_ok']}", flush=True)
    print(f"Duration:   {elapsed:.2f}s", flush=True)
    print(f"Speed:      {scanner.stats['total']/elapsed:.0f} IPs/s", flush=True)
    print(f"Valid:      {len(scanner.valid_ips)}", flush=True)
    print(f"{'='*60}", flush=True)
    
    if scanner.valid_ips:
        valid_sorted = sorted(set(scanner.valid_ips))
        with open(RESULT_FILE, 'w') as f:
            f.write(f"Spotify Valid IPs\n")
            f.write(f"Scan: {datetime.now()}\n")
            f.write(f"Total: {len(valid_sorted)}\n")
            f.write(f"{'='*60}\n\n")
            for ip in valid_sorted:
                f.write(ip + '\n')
        
        print(f"\n[+] Valid IPs:", flush=True)
        for ip in valid_sorted[:50]:
            print(f"    {ip}", flush=True)
        print(f"\n[+] Saved to: {RESULT_FILE}", flush=True)
    else:
        print(f"\n[-] No valid IPs found!", flush=True)
    
    scanner.log_fp.close()
    return 0 if scanner.valid_ips else 1


if __name__ == "__main__":
    sys.exit(main())

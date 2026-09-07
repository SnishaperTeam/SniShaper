#!/usr/bin/env python3
"""
通用IP扫描器
用法: python scanner.py <域名:端口> <IP段(CIDR)> [最大线程数]

示例:
  python scanner.py open.spotify.com:443 23.2.16.0/24 128
  python scanner.py google.com:443 35.0.0.0/8 64
  python scanner.py example.com:80 1.0.0.0/16 32
"""

import socket
import ssl
import sys
import os
import time
import select
import errno
import ipaddress
from datetime import datetime
from concurrent.futures import ThreadPoolExecutor, as_completed
import threading


class Scanner:
    def __init__(self, domain, port, max_threads):
        self.domain = domain
        self.port = port
        self.max_threads = max_threads
        self.lock = threading.Lock()
        self.results = []  # (ip, tcp_time, total_time)
        self.stats = {'total': 0, 'tcp_ok': 0, 'ssl_ok': 0}
        self.start_time = time.time()
        
        # 日志
        os.makedirs("logs", exist_ok=True)
        ts = datetime.now().strftime('%Y%m%d_%H%M%S')
        safe_domain = domain.replace('.', '_')
        self.log_file = open(f"logs/scan_{safe_domain}_{ts}.log", 'w', encoding='utf-8', buffering=8192)
        self.result_file = f"logs/scan_{safe_domain}_{ts}_valid.txt"
        
        self.log(f"Scan: {domain}:{port} | Threads: {max_threads}")
    
    def log(self, msg, level="INFO"):
        ts = datetime.now().strftime('%H:%M:%S.%f')[:-3]
        line = f"[{ts}] [{level}] {msg}"
        with self.lock:
            print(line, flush=True)
            self.log_file.write(line + '\n')
            self.log_file.flush()
    
    def scan_ip(self, ip):
        self.stats['total'] += 1
        start = time.time()
        
        try:
            sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            sock.settimeout(1.5)
            
            tcp_start = time.time()
            result = sock.connect_ex((ip, self.port))
            tcp_time = time.time() - tcp_start
            
            if result != 0:
                sock.close()
                return
            
            # TCP通过，SSL验证
            try:
                ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
                ctx.check_hostname = False
                ctx.verify_mode = ssl.CERT_NONE
                ctx.set_ciphers('DEFAULT:@SECLEVEL=1')
                ctx.options |= ssl.OP_NO_SSLv2 | ssl.OP_NO_SSLv3
                
                ssl_sock = ctx.wrap_socket(sock, server_hostname=ip)
                ssl_sock.settimeout(2)
                cert = ssl_sock.getpeercert(binary_form=True)
                total_time = time.time() - start
                
                ssl_sock.close()
                sock.close()
                
                if cert:
                    with self.lock:
                        self.results.append((ip, tcp_time, total_time))
                        self.stats['ssl_ok'] += 1
                    self.log(f"{ip} - OK (tcp={tcp_time:.3f}s total={total_time:.3f}s)", "SUCCESS")
                    return
            except:
                pass
            
            sock.close()
            
        except:
            pass
        
        self.stats['tcp_ok'] += 1


def expand_cidr(cidr):
    try:
        net = ipaddress.ip_network(cidr, strict=False)
        total = net.num_addresses
        if total > 65536:
            step = total // 4096
            return [str(net.network_address + i) for i in range(0, total, step)][:4096]
        elif total > 4096:
            step = total // 1024
            return [str(net.network_address + i) for i in range(0, total, step)][:1024]
        else:
            return [str(ip) for ip in net.hosts()]
    except:
        return []


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        return 1
    
    # 解析参数
    target = sys.argv[1]  # 域名:端口
    ip_cidr = sys.argv[2]  # IP段
    max_threads = int(sys.argv[3]) if len(sys.argv) > 3 else 64
    
    if ':' in target:
        domain, port = target.rsplit(':', 1)
        port = int(port)
    else:
        domain = target
        port = 443
    
    print(f"""
{'='*60}
  IP Scanner
  Target: {domain}:{port}
  IP Range: {ip_cidr}
  Threads: {max_threads}
{'='*60}
""", flush=True)
    
    scanner = Scanner(domain, port, max_threads)
    
    # 展开IP
    ip_list = expand_cidr(ip_cidr)
    scanner.log(f"IPs to scan: {len(ip_list)}")
    
    # 扫描
    with ThreadPoolExecutor(max_workers=max_threads) as executor:
        futures = {executor.submit(scanner.scan_ip, ip): ip for ip in ip_list}
        scanned = 0
        
        for future in as_completed(futures):
            scanned += 1
            if scanned % 100 == 0:
                elapsed = time.time() - scanner.start_time
                speed = scanned / elapsed if elapsed > 0 else 0
                print(f"\r[*] {scanned}/{len(ip_list)} ({scanned/len(ip_list)*100:.1f}%) | {speed:.0f}/s | Valid: {len(scanner.results)}", flush=True)
    
    # 按响应速度排序
    scanner.results.sort(key=lambda x: x[2])
    
    elapsed = time.time() - scanner.start_time
    
    # 保存结果
    with open(scanner.result_file, 'w') as f:
        f.write(f"Target: {domain}:{port}\n")
        f.write(f"IP Range: {ip_cidr}\n")
        f.write(f"Scan Time: {datetime.now()}\n")
        f.write(f"Total: {len(scanner.results)}\n")
        f.write(f"{'='*60}\n\n")
        f.write(f"{'IP':<20} {'TCP(ms)':<12} {'Total(ms)':<12}\n")
        f.write(f"{'-'*44}\n")
        for ip, tcp_t, total_t in scanner.results:
            f.write(f"{ip:<20} {tcp_t*1000:<12.1f} {total_t*1000:<12.1f}\n")
    
    # 打印结果
    print(f"\n\n{'='*60}")
    print(f"COMPLETE - {len(scanner.results)} valid IPs (sorted by speed)")
    print(f"{'='*60}")
    print(f"Scanned: {scanner.stats['total']} | Duration: {elapsed:.2f}s | Speed: {scanner.stats['total']/elapsed:.0f}/s")
    print(f"\n{'IP':<20} {'TCP(ms)':<12} {'Total(ms)':<12}")
    print(f"{'-'*44}")
    
    for ip, tcp_t, total_t in scanner.results[:30]:
        print(f"{ip:<20} {tcp_t*1000:<12.1f} {total_t*1000:<12.1f}")
    
    if len(scanner.results) > 30:
        print(f"... and {len(scanner.results)-30} more")
    
    print(f"\nSaved to: {scanner.result_file}")
    print(f"{'='*60}")
    
    scanner.log_file.close()
    return 0 if scanner.results else 1


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
import socket, ssl, time, sys

domain = 'open.spotify.com'

# 先解析真实IP
print(f'Resolving {domain}...')
try:
    real_ip = socket.getaddrinfo(domain, 443, socket.AF_INET)[0][4][0]
    print(f'Real IP: {real_ip}')
except:
    real_ip = None
    print('DNS failed')

# 测试的IP列表
test_ips = ['35.186.224.206', '23.2.16.51', '23.202.35.81']
if real_ip:
    test_ips.insert(0, real_ip)

port = 443

for ip in test_ips:
    print()
    print(f'Testing {ip}:{port} for {domain}')
    print('=' * 50)

    # TCP
    try:
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(5)
        start = time.time()
        sock.connect((ip, port))
        tcp_time = time.time() - start
        print(f'TCP:    OK ({tcp_time:.3f}s)')
    except Exception as e:
        print(f'TCP:    FAIL - {e}')
        continue

    # SSL
    try:
        ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
        ctx.check_hostname = False
        ctx.verify_mode = ssl.CERT_NONE

        ssl_sock = ctx.wrap_socket(sock, server_hostname=domain)
        ssl_sock.settimeout(5)

        cert = ssl_sock.getpeercert()
        subject = dict(x[0] for x in cert.get('subject', []))
        issuer = dict(x[0] for x in cert.get('issuer', []))
        sans = cert.get('subjectAltName', [])

        cn = subject.get('commonName', 'N/A')
        org = issuer.get('organizationName', 'N/A')

        print(f'SSL:    OK')
        print(f'CN:     {cn}')
        print(f'Issuer: {org}')
        print(f'SANs:   {", ".join(s[1] for s in sans[:5])}')

        ssl_sock.close()
    except ssl.SSLCertVerificationError as e:
        print(f'SSL:    CertError - {e}')
    except Exception as e:
        print(f'SSL:    FAIL - {e}')
        sock.close()
        continue

    sock.close()

print()
print('Done!')

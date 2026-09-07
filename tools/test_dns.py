import socket, ssl

domain = 'open.spotify.com'
ip = socket.getaddrinfo(domain, 443, socket.AF_INET)[0][4][0]
print(f'Resolved: {domain} -> {ip}')

sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
sock.settimeout(10)
sock.connect((ip, 443))

ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
ctx.check_hostname = False
ctx.verify_mode = ssl.CERT_NONE

ssl_sock = ctx.wrap_socket(sock, server_hostname=domain)
cert = ssl_sock.getpeercert()
subject = dict(x[0] for x in cert.get('subject', []))
cn = subject.get('commonName', 'N/A')
print(f'SSL OK! CN={cn}')
ssl_sock.close()
sock.close()

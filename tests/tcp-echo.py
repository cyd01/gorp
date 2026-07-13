#!/usr/bin/env python3
import socketserver
import ssl
import sys


port = int(sys.argv[1])
prefix = sys.argv[2].encode()
cert = sys.argv[3] if len(sys.argv) > 3 else ""
key = sys.argv[4] if len(sys.argv) > 4 else ""


class EchoHandler(socketserver.BaseRequestHandler):
    def handle(self):
        self.request.sendall(prefix)
        while True:
            data = self.request.recv(4096)
            if not data:
                return
            self.request.sendall(data)


class EchoServer(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


server = EchoServer(("127.0.0.1", port), EchoHandler)
if cert and key:
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.load_cert_chain(cert, key)
    server.socket = context.wrap_socket(server.socket, server_side=True)
server.serve_forever()

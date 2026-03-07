import socket
import os
import platform

# Use a local file name; works on all OS
SOCKET_PATH = "backeye.sock" 

if platform.system() == "Windows":
    # On Windows, UDS paths often look like this to avoid collisions
    # but a simple filename in the current directory works too!
    SOCKET_PATH = r"\\.\pipe\myapp_socket" # Optional: Windows Named Pipe style
    # Actually, for modern AF_UNIX on Windows, a standard path is fine:
    SOCKET_PATH = os.path.join(os.getcwd(), "backeye.sock")

# 1. Clean up the old socket file if it exists
if os.path.exists(SOCKET_PATH):
    os.remove(SOCKET_PATH)

# 2. Create a Unix Domain Socket (AF_UNIX)
server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
server.bind(SOCKET_PATH)
server.listen(1)

print(f"Server listening on {SOCKET_PATH}...")

try:
    while True:
        conn, addr = server.accept()
        with conn:
            data = conn.recv(1024)
            if not data:
                break
            
            message = data.decode()
            print(f"Received from Go: {message}")
            
            # Send response
            conn.sendall(f"Hello Go! I got your message: {message}".encode())
finally:
    os.remove(SOCKET_PATH)
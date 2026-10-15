"""Small RESP client for disposable Redis integration fixtures, no dependencies."""
import socket
from urllib.parse import unquote, urlparse


class RedisFixture:
    def __init__(self, url):
        address = urlparse(url)
        self.socket = socket.create_connection((address.hostname, address.port or 6379), timeout=10)
        self.reader = self.socket.makefile("rb")
        if address.password:
            auth = [unquote(address.password)]
            if address.username:
                auth.insert(0, unquote(address.username))
            self.command("AUTH", *auth)
        self.command("SELECT", address.path.lstrip("/") or "15")

    def close(self):
        self.reader.close()
        self.socket.close()

    @staticmethod
    def encode(command):
        parts = [str(part).encode() for part in command]
        return b"*%d\r\n" % len(parts) + b"".join(b"$%d\r\n" % len(p) + p + b"\r\n" for p in parts)

    def receive(self):
        line = self.reader.readline()
        kind, value = line[:1], line[1:-2]
        if kind == b"-":
            raise RuntimeError(value.decode())
        if kind == b"+":
            return value.decode()
        if kind == b":":
            return int(value)
        if kind == b"$":
            size = int(value)
            if size < 0:
                return None
            return self.reader.read(size + 2)[:-2].decode()
        if kind == b"*":
            return [self.receive() for _ in range(int(value))]
        raise RuntimeError("Invalid Redis response: " + repr(line))

    def command(self, *command):
        self.socket.sendall(self.encode(command))
        return self.receive()

    def pipeline(self, commands):
        commands = list(commands)
        self.socket.sendall(b"".join(self.encode(command) for command in commands))
        return [self.receive() for _ in commands]

    def command_calls(self):
        stats = {}
        for line in self.command("INFO", "commandstats").splitlines():
            if line.startswith("cmdstat_"):
                command, fields = line.split(":", 1)
                stats[command.removeprefix("cmdstat_")] = int(fields.split(",")[0].split("=")[1])
        return stats

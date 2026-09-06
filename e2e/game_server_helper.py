import json
import socket
from typing import Any


# asks game server for the match config
def get_match_config(host: str, port: str) -> dict[str, Any]:
    with socket.socket(family=socket.AF_INET, type=socket.SOCK_DGRAM) as sock:
        sock.settimeout(3)

        _ = sock.sendto(b"PING", (host, parse_port(port)))

        data = sock.recvfrom(1024)[0]
        assert data[:4] == b"PONG"

        # returns match config
        config = json.loads(s=data[4:])
        assert isinstance(config, dict)
        return config


def send_match_stop(host: str, port: str) -> None:
    with socket.socket(family=socket.AF_INET, type=socket.SOCK_DGRAM) as sock:
        sock.settimeout(3)
        _ = sock.sendto(b"STOP", (host, parse_port(port)))


def parse_port(port: str) -> int:
    parts = port.split("/")

    if len(parts) > 1:
        assert len(parts) == 2
        assert parts[1] == "udp"

    return int(parts[0])
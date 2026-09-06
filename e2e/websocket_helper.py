import json
import threading
from collections.abc import Callable
from dataclasses import dataclass
from typing import Any

from websocket import WebSocketApp

from e2e.game_server_helper import get_match_config, send_match_stop
from e2e.globals import logger


def create_ws(
    token: str,
    on_message: Callable[[WebSocketApp, Any], None] | None = None,
    run_forever: bool = False,
) -> WebSocketApp:
    ws = WebSocketApp(
        url="ws://localhost:3000/api/connection",
        header={"cookie": f"session={token}"},
        on_message=on_message,
    )
    if run_forever:
        ws.run_forever()
    return ws


@dataclass
class WsPayload:
    login: str
    host: str
    port: str
    matchAuthToken: str


def parse_ws_payload(payload: Any) -> WsPayload:
    assert isinstance(payload, str)

    payload = json.loads(payload)

    return WsPayload(**payload)


def verify_match_config(match_config: Any, login: str, auth_token: str) -> None:
    assert "players" in match_config
    players = match_config["players"]
    assert isinstance(players, list)
    assert {
        "login": login,
        "matchAuthToken": auth_token,
    } in players


# Returns True if the user was successfully queued, False otherwise.
def queue_user(
    token: str, login: str, queue_timeout: float, stop_match: bool = False
) -> bool:
    msg_received = False
    msg: str | bytes = ""
    msg_lock = threading.Lock()

    def on_message(ws: WebSocketApp, message: Any) -> None:
        nonlocal msg_received, msg
        with msg_lock:
            msg_received = True
            msg = message
        ws.close()

    ws = create_ws(token)
    ws.on_message = on_message

    threading.Timer(interval=queue_timeout, function=ws.close).start()

    _ = ws.run_forever()

    with msg_lock:
        if not msg_received:
            return False
        payload = msg

    logger.info("user '%s' has been queued with payload '%s'", login, payload)

    parsed_ws = parse_ws_payload(payload)

    try:
        # config request also asserts that communication is fine
        match_config = get_match_config(parsed_ws.host, parsed_ws.port)
        verify_match_config(match_config, login, parsed_ws.matchAuthToken)

        if stop_match:
            send_match_stop(parsed_ws.host, parsed_ws.port)
    except TimeoutError:
        pass

    return True

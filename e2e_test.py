import json
import logging
import random
import shutil
import socket
import subprocess
import tempfile
import threading
import time
from collections.abc import Callable, Generator
from concurrent.futures import ThreadPoolExecutor, wait
from dataclasses import dataclass
from datetime import datetime
from typing import Any, Literal

import pytest
import requests
from playwright.sync_api import Page, expect
from websocket import WebSocketApp

GAME_SERVER_COUNT = 2
PLAYERS_PER_ROOM = 2
GAME_SERVER_IMAGE = "game-server:latest"
GAME_CLIENT_FILENAME = "game-client.zip"

logger = logging.getLogger(__name__)


def start_services(downloads_folder: str) -> None:
    _ = subprocess.run(
        ["make", "up", "UP_ARGS=-d"],
        check=True,
        env={"DOWNLOADS_FOLDER": downloads_folder},
    )


def stop_services(fail_on_game_server: bool = False) -> None:
    _ = subprocess.run(["make", "down", "DOWN_ARGS=-t 10 -v"], check=True)

    images = get_containers_by_image(GAME_SERVER_IMAGE)
    kill_containers(images)

    if len(images) != 0 and fail_on_game_server:
        pytest.fail("game-server containers were not killed")


# NOTE: by default it searches through all logs, but it can be filtered by `since` argument
def wait_for_log(
    service: str, target: str, timeout: int, since: datetime | None = None
) -> None:
    start = time.time()

    cmd = ["docker", "compose", "logs", service]
    if since is not None:
        cmd += ["--since", since.isoformat()]

    while time.time() - start < timeout:
        result = subprocess.run(cmd, capture_output=True, text=True, check=False)
        if target in result.stdout:
            return
        time.sleep(0.5)

    raise TimeoutError(f"Timed out waiting for '{target}' in {service} logs.")


def stream_logs() -> subprocess.Popen:
    return subprocess.Popen(["make", "logs", "LOG_ARGS=-f"])


def wait_for_server_manager(since: datetime | None = None) -> None:
    wait_for_log(
        service="server-manager", target='"msg":"started"', timeout=10, since=since
    )


def wait_for_api() -> None:
    wait_for_log(service="api", target='"msg":"api server listening"', timeout=30)


@pytest.fixture(scope="session")
def base_url() -> str:
    return "http://localhost:3000"


@pytest.fixture
def downloads_folder() -> Generator[str]:
    tmp = tempfile.mkdtemp()
    yield tmp
    shutil.rmtree(tmp)


@pytest.fixture(autouse=True)
def setup_services(downloads_folder: str) -> Generator:
    stop_services(fail_on_game_server=False)

    start_services(downloads_folder)

    wait_for_server_manager()
    log_proc = stream_logs()

    wait_for_api()

    yield

    log_proc.kill()
    log_proc.wait()

    stop_services(fail_on_game_server=True)


def restart_containers(containers: list[str]) -> None:
    kill_containers(containers)

    for c in containers:
        logger.info("restarting container %s", c)
        _ = subprocess.run(["docker", "start", c], check=True)


def kill_containers(containers: list[str]) -> None:
    for c in containers:
        logger.info("killing container %s", c)
        _ = subprocess.run(["docker", "kill", c], check=True)


def get_containers_by_image(image: str) -> list[str]:
    result = subprocess.run(
        ["docker", "ps", "-q", "--filter", f"ancestor={image}"],
        capture_output=True,
        text=True,
        check=True,
    )
    return result.stdout.strip().splitlines()


def register_user(login: str) -> str:
    response = requests.post(
        url="http://localhost:3000/api/register",
        json={"login": login, "password": "password"},
    )

    assert response.json() == {}
    token = response.cookies.get(name="session")
    assert token is not None
    assert len(token) > 0

    return token


# TODO: add type validation in some way
@dataclass
class MatchInfo:
    host: str
    port: str
    matchAuthToken: str


def get_user_match(token: str) -> MatchInfo | None:
    response = requests.get(
        url="http://localhost:3000/api/match",
        cookies={"session": token},
    )
    assert response.status_code == 200
    payload = response.json()
    assert "match" in payload

    match = payload["match"]

    if match is None:
        return match
    else:
        assert isinstance(match, dict)
        return MatchInfo(**match)


def is_in_match(token: str) -> bool:
    return get_user_match(token) is not None


@dataclass
class User:
    login: str
    token: str


# Returns a list of tokens and list of users
def create_users(number: int) -> list[User]:
    users: list[User] = []

    for i in range(number):
        login = "user" + str(i)
        token = register_user(login)
        users.append(User(login=login, token=token))

    return users


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


@dataclass
class MatchResult:
    id: int
    details: dict[str, Any]
    canceled: bool
    active: bool


def get_results(token: str) -> list[MatchResult]:
    resp = requests.get("http://localhost:3000/api/results", cookies={"session": token})
    assert resp.status_code == 200

    result = resp.json()
    assert isinstance(result, dict)
    assert len(result) == 1
    assert "matches" in result

    matches = result["matches"]
    assert isinstance(matches, list)
    for match in matches:
        assert isinstance(match, dict)

    return [MatchResult(**match) for match in matches]


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


@pytest.mark.parametrize(
    "n, queue_timeout", [(1, 10.0), (2, 10.0), (5, 20.0), (6, 20.0)]
)
def test_add_users(n: int, queue_timeout: float) -> None:
    users = create_users(n)

    with ThreadPoolExecutor() as executor:
        futures = [
            executor.submit(
                queue_user, user.token, user.login, queue_timeout, stop_match=True
            )
            for user in users
        ]
        wait(futures, timeout=queue_timeout + 10)

    matches = [future.result() for future in futures]

    assert sum(matches) == n - n % PLAYERS_PER_ROOM

    # wait for user matches to finish
    # TODO: use some lib for waiting
    start = time.time()
    timeout = 15
    for user in users:
        if not is_in_match(user.token):
            continue
        time.sleep(0.5)
        if time.time() - start > timeout:
            break

    # reading results
    for user, matched in zip(users, matches, strict=True):
        results = get_results(user.token)

        if matched:
            assert len(results) == 1, "matched but no results in api"
            match = results[0]
            assert match.active == False
            assert match.canceled == False
            assert match.details == {"success": True}
        else:
            assert len(results) == 0, "not matched but still has results in api"


def test_crash_server_manager() -> None:
    users = create_users(6)

    matchmakers = users[:-1]
    lazy_user = users[-1]

    with ThreadPoolExecutor() as executor:
        futures = [
            executor.submit(queue_user, user.token, user.login, 15.0)
            for user in matchmakers
        ]
        wait(futures, timeout=30.0)

    matches = [future.result() for future in futures]
    assert sum(matches) == 4

    restart = threading.Thread(
        target=restart_containers,
        args=(["multiplayer-asset-server-manager-1"],),
    )

    now = datetime.now()  # ruff: ignore[DTZ005]

    restart.start()
    restart.join(timeout=15.0)

    wait_for_server_manager(since=now)

    # matchmakers should not be in match
    # but those who were matched should have it canceled
    for user, matched in zip(matchmakers, matches):
        assert not is_in_match(user.token)

        results = get_results(user.token)

        if not matched:
            assert len(results) == 0
        else:
            assert len(results) == 1
            assert results[0].canceled == True
            assert results[0].active == False
            assert results[0].details == {}

    # lazy user should not be impacted
    assert not is_in_match(lazy_user.token)
    results = get_results(lazy_user.token)
    assert len(results) == 0

    # ensure no one is still matchmaking somehow
    res = queue_user(lazy_user.token, lazy_user.login, 10.0)
    assert res is False


def test_crash_game_server() -> None:
    users = create_users(4)

    with ThreadPoolExecutor() as executor:
        futures = [
            executor.submit(queue_user, user.token, user.login, 10.0) for user in users
        ]
        wait(futures, timeout=20)

    assert all(future.done() for future in futures)

    user_matches = [get_user_match(user.token) for user in users]
    assert all(match is not None for match in user_matches)

    containers = get_containers_by_image(GAME_SERVER_IMAGE)
    assert len(containers) == GAME_SERVER_COUNT

    kill = threading.Thread(target=kill_containers, args=([containers[0]],))
    kill.start()
    kill.join(timeout=15.0)

    def assert_match_removed() -> None:
        removed_matches = set()
        removed_users = 0

        for user in users:
            results = get_results(user.token)
            assert len(results) == 1

            if results[0].active:
                continue

            assert results[0].canceled
            assert results[0].details == {}

            removed_matches.add(results[0].id)
            removed_users += 1

        assert len(removed_matches) == 1
        assert removed_users == 2

    # TODO: get function for wait_for
    start = time.time()
    timeout = 30
    while time.time() - start < timeout:
        try:
            assert_match_removed()
            break
        except AssertionError:
            time.sleep(1)


def test_browser(page: Page, downloads_folder: str) -> None:
    resp = page.goto("/")
    assert resp is not None
    assert resp.status == 200

    # fills register or login form
    # it either submits it successfully or gets error and backs out
    # page will always end up on base_url
    def form(
        mode: Literal["login", "register"], login: str, password: str, accept: bool
    ) -> None:
        page.goto("/" + mode)

        error_text = page.locator("p")
        expect(error_text).to_be_hidden()

        loginBox = page.get_by_test_id("login-input")
        loginBox.fill(login)

        passBox = page.get_by_test_id("password-input")
        passBox.fill(password)

        page.get_by_test_id(f"{mode}-apply").click()

        if accept:
            page.wait_for_url("/")
            expect(page.get_by_test_id("logout")).to_be_visible()
            expect(page.get_by_test_id("title")).to_have_text(expected=login)
        else:
            expect(error_text).to_be_visible()
            page.get_by_text("Back").click()
            page.wait_for_url("/")

    form(mode="login", login="login", password="password", accept=False)
    form(mode="register", login="", password="", accept=False)
    form(mode="register", login="login", password="password", accept=True)

    btn = page.get_by_test_id("logout")
    expect(btn).to_be_visible()
    btn.click()
    expect(page.get_by_test_id("title")).to_have_text(expected="Log in")

    form(mode="login", login="login", password="password", accept=True)

    with open(downloads_folder + "/" + GAME_CLIENT_FILENAME, "w") as f:
        file_content = random.randbytes(8).hex()
        _ = f.write(file_content)

    with tempfile.NamedTemporaryFile() as tmp:
        with page.expect_download() as download_info:
            page.get_by_test_id("download-client").click()
            download = download_info.value
            download.save_as(tmp.name)

        tmp.seek(0)
        assert tmp.read().decode() == file_content

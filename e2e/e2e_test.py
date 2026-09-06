import random
import shutil
import tempfile
import threading
import time
from collections.abc import Generator
from concurrent.futures import ThreadPoolExecutor, wait
from datetime import datetime
from typing import Literal

import pytest
from playwright.sync_api import Page, expect

from api_helper import create_users, get_results, get_user_match, is_in_match
from docker_helper import (
    get_containers_by_image,
    kill_containers,
    restart_containers,
    start_services,
    stop_services,
    stream_logs,
    wait_for_api,
    wait_for_server_manager,
)
from globals import (
    GAME_CLIENT_FILENAME,
    GAME_SERVER_COUNT,
    GAME_SERVER_IMAGE,
    PLAYERS_PER_ROOM,
)
from websocket_helper import queue_user


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
        args=(["lobbylab-server-manager-1"],),
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

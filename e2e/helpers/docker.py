import os
import subprocess
import time
from datetime import datetime

import pytest

from e2e.helpers.globals import GAME_SERVER_IMAGE, PROJECT_FOLDER, logger


def start_services(downloads_folder: str) -> None:
    _ = subprocess.run(
        ["make", "up", "UP_ARGS=-d"],
        check=True,
        env={**os.environ, "DOWNLOADS_FOLDER": downloads_folder},
        cwd=PROJECT_FOLDER,
    )


def stop_services(fail_on_game_server: bool = False) -> None:
    _ = subprocess.run(
        ["make", "down", "DOWN_ARGS=-t 10 -v"],
        check=True,
        cwd=PROJECT_FOLDER,
    )

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
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            check=False,
            cwd=PROJECT_FOLDER,
        )
        if target in result.stdout:
            return
        time.sleep(0.5)

    raise TimeoutError(f"Timed out waiting for '{target}' in {service} logs.")


def stream_logs() -> subprocess.Popen:
    return subprocess.Popen(
        ["make", "logs", "LOG_ARGS=-f"],
        cwd=PROJECT_FOLDER,
    )


def wait_for_server_manager(since: datetime | None = None) -> None:
    wait_for_log(
        service="server-manager", target='"msg":"started"', timeout=10, since=since
    )


def wait_for_api() -> None:
    wait_for_log(service="api", target='"msg":"api server listening"', timeout=30)


def restart_containers(containers: list[str]) -> None:
    kill_containers(containers)

    for c in containers:
        logger.info("restarting container %s", c)
        _ = subprocess.run(
            ["docker", "start", c],
            check=True,
            cwd=PROJECT_FOLDER,
        )


def kill_containers(containers: list[str]) -> None:
    for c in containers:
        logger.info("killing container %s", c)
        _ = subprocess.run(
            ["docker", "kill", c],
            check=True,
            cwd=PROJECT_FOLDER,
        )


def get_containers_by_image(image: str) -> list[str]:
    result = subprocess.run(
        ["docker", "ps", "-q", "--filter", f"ancestor={image}"],
        capture_output=True,
        text=True,
        check=True,
        cwd=PROJECT_FOLDER,
    )
    return result.stdout.strip().splitlines()

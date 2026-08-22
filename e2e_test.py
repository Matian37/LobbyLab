import json
import random
import socket
import subprocess
import threading
import time
import zipfile
from collections.abc import Generator
from subprocess import CompletedProcess
from threading import Thread
from typing import Any, cast

import pytest
import requests
import websocket
from playwright.sync_api import Page, expect
from requests.models import Response
from websocket._app import WebSocketApp

#To run this test run the following command in main directory:
#pytest -s e2e_test.py (-s flag shows all logs)
#or
#pytest -s e2e_test.py::{name_of_specific_test}

#globals
PLAYERS_PER_ROOM = 2
CONTAINERS = 2

@pytest.fixture()
def setup_services() -> Generator[None, None, None]:
  _ = subprocess.run(['sudo', 'rm', '-rf', './data'], check=True)
  _ = subprocess.run(['make', 'up', 'UP_ARGS=-d'], check=True)
  def stream_logs() -> None:
    _ = subprocess.run(['docker', 'compose', 'logs', '-f'], check=True)
  log_thread: Thread = threading.Thread(target=stream_logs, daemon=True)
  log_thread.start()
  time.sleep(4)
  yield
  _ = subprocess.run(['make', 'down', 'DOWN_ARGS=-t 0 -v'], check=True) 

def log(args: list[object]) -> None:
    msg = ''
    for a in args:
        msg += str(a) + ' '
    print(f'\033[36m[E2E TEST LOGGER]: {msg}\033[0m')

def restart_container(containers: list[str], off_time: float) -> None:
    kill_container(containers)
    time.sleep(off_time)
    for c in containers:
        log(args=['restarting container', c])
        _ = subprocess.run(['docker', 'start', c], check=True)
def kill_container(containers: list[str]) -> None:
    for c in containers:
        log(args=['killing container', c])
        _ = subprocess.run(['docker', 'kill', c], check=True)

def get_containers_by_image(image: str) -> list[str]:
    result: CompletedProcess[str] = subprocess.run(
        ["docker", "ps", "-q", "--filter", f"ancestor={image}"],
        capture_output=True,
        text=True,
        check=True,
    )
    container_ids: str = result.stdout.strip()
    containers: list[str] = container_ids.splitlines()

    return containers

def register_user(username: str) -> str:
    response: Response = requests.post(
        url='http://localhost:3000/api/register',
        json={'login': username,
        'password': 'passawubasd!!'
    })

    assert response.json() == {}
    token = response.cookies.get(name='session')
    assert isinstance(token, str)
    return token

def ping_game_server(host: str, port: int) -> dict[str, Any]: 
    time.sleep(random.random())
    with socket.socket(family=socket.AF_INET, type=socket.SOCK_DGRAM) as sock:
        sock.settimeout(3)
            
        message = b"PING"
        _ = sock.sendto(
            message, 
            (host, port)
        )
            
        data: bytes = sock.recvfrom(1024)[0]
        assert data[:4] == b"PING"

        #returns match config
        result = json.loads(s=data[4:].decode(encoding='utf-8')) 
        assert isinstance(result, dict)
        return result 

def queue_user(token: str, username: str, ws_timeout: float) -> bool:
    payload = {}
    match_found = False
    def on_message(_ws: websocket.WebSocketApp, message: str) -> None:
        nonlocal payload, match_found
        payload = json.loads(s=message)  
        assert isinstance(payload, dict)
        if "host" in payload: 
            match_found = True

    ws: WebSocketApp = websocket.WebSocketApp(
        "ws://localhost:3000/api/connection",
        header={"cookie": f"session={token}"},
        on_message=on_message,
    )
    threading.Timer(interval=ws_timeout, function=ws.close).start() 
    _ = ws.run_forever() 
    if not match_found:
        return False
    
    log(args=[username, ": Queued user payload", payload])

    assert payload["login"] == username

    match_config = ping_game_server(
        host=cast(str, payload["host"]), 
        port=int(cast(int, payload["port"]))
    )
    players_in_match = match_config["players"] 
    match_tokens: list[str] = []
    for p in players_in_match: 
        match_tokens.append(p["matchAuthToken"]) 
    
    assert payload["matchAuthToken"] in match_tokens

    return True

def add_n_users(n: int):
    #registering
    tokens: list[str] = []
    usernames: list[str] = []
    for i in range(n):
        usernames.append('user' + str(i))
        tokens.append(register_user(usernames[-1]))
        assert len(tokens[-1]) > 0

    #should be no results yet
    results = requests.get( 
        'http://localhost:3000/api/results',
        cookies={'session': tokens[0]}
    ).json()
    assert isinstance(results, dict)

    assert "matches" in results
    assert len(cast(list[object], results["matches"])) == 0

    #adding to waitlist and matching players
    threads: list[threading.Thread] = []
    matched_players = 0
    def add(token: str, username: str) -> None:
        nonlocal matched_players
        #if some players have to wait for a free container then ws_timeout has to be longer
        ws_timeout = 5
        if n > PLAYERS_PER_ROOM * CONTAINERS:
            ws_timeout = 15 * CONTAINERS
        
        if queue_user(token, username, ws_timeout) == True:
            matched_players += 1

    for i in range(n):
        threads.append(threading.Thread(
            target=add, args=(
                tokens[i], 
                usernames[i],
            )
        ))
        threads[-1].start()

    for thr in threads:
        thr.join()
    
    #some players might not be matched
    assert matched_players == n - n % PLAYERS_PER_ROOM

    #reading results
    time.sleep(6)
    players_without_results = 0
    for i in range(n):
        results = requests.get( 
            'http://localhost:3000/api/results',
            cookies={'session': tokens[i]}
        ).json()
        assert isinstance(results, dict)
        results = cast(dict[str, Any], results) 
        assert "matches" in results

        if len(cast(list[object], results["matches"])) == 0:
            players_without_results += 1
            continue

        assert len(cast(list[object], results["matches"])) == 1
        match = results["matches"][0] 
        log(["match result for ", usernames[i], match])

        assert match["canceled"] == False
        assert match["details"] != None

    assert players_without_results == n % PLAYERS_PER_ROOM

@pytest.mark.usefixtures("setup_services")
@pytest.mark.parametrize(argnames="n", argvalues=[1, 2, 5, 6])
def test_add_users(n: int) -> None:
    add_n_users(n)

@pytest.mark.usefixtures("setup_services")
@pytest.mark.parametrize(argnames="kill_before_queue", argvalues=[True, False])
def test_crash_server_manager(kill_before_queue: bool) -> None:
    #registering
    tokens: list[str] = []
    usernames: list[str] = []
    for i in range(2):
        usernames.append('user' + str(i))
        tokens.append(register_user(username=usernames[-1]))

    threads: list[Thread] = []

    def add(token: str) -> None:
        ws: WebSocketApp = websocket.WebSocketApp(
        url="ws://localhost:3000/api/connection",
        header={"cookie": f"session={token}"},
        )

        _ = ws.run_forever() 

    #restarting container while matchmaking
    kill: Thread = threading.Thread(
        target=restart_container, 
        args=(
                ['multiplayer-asset-server-manager-1'], 5,
            )
        )  

    if kill_before_queue:
        kill.start()
        time.sleep(1)
    for i in range(2):
        threads.append(threading.Thread(target=add, args=(tokens[i],)))
        threads[-1].start()
    if not kill_before_queue:
        kill.start()

    for thr in threads:
        thr.join()
    kill.join()
    time.sleep(6)
    #match should be canceled
    for i in range(2):
        results = requests.get( 
            'http://localhost:3000/api/results',
            cookies={'session': tokens[i]}
        ).json()
        assert isinstance(results, dict)

        assert "matches" in results
        assert len(cast(list[object], results["matches"])) == 1
        match = cast(dict[str, object], results["matches"][0])
        assert match["canceled"] == True

@pytest.mark.usefixtures("setup_services")
def test_crash_game_server() -> None:
    #registering
    tokens: list[str] = []
    usernames: list[str] = []
    for i in range(2):
        usernames.append('user' + str(i))
        tokens.append(register_user(username=usernames[-1]))

    threads: list[Thread] = []

    #find all containers of image game-server
    containers: list[str] = get_containers_by_image(image='game-server')

    def add(token: str) -> None:
        ws: WebSocketApp = websocket.WebSocketApp(
        url="ws://localhost:3000/api/connection",
        header={"cookie": f"session={token}"},
        )

        _ = ws.run_forever() 

    #restart containers when match is running
    kill: Thread = threading.Thread(
        target=kill_container, 
        args=(containers,)
    )

    for i in range(2):
        threads.append(threading.Thread(
            target=add,
            args=(tokens[i],)
        ))
        threads[-1].start()

    kill.start()
    for thr in threads:
        thr.join()
    kill.join()

    #matches should be canceled
    time.sleep(17)
    for i in range(2):
        results = requests.get( 
            'http://localhost:3000/api/results',
            cookies={'session': tokens[i]}
        ).json()
        assert isinstance(results, dict)

        assert "matches" in results
        assert len(cast(list[object], results["matches"])) == 1 
        match = cast(dict[str, object], results["matches"][0])
        log(["wynik ", match])
        assert match["canceled"] == True

@pytest.mark.usefixtures("setup_services")
def test_browser(page: Page) -> None:
    _ = page.goto("http://localhost:3000")

    def form(form_name: str, input_value: str, fake: bool) -> None:
        #make sure we are in main page and go to form page
        expect(page.get_by_text('Log out')).to_be_visible()
        page.get_by_text(form_name).click()

        #make sure we are in form page and try to fill form
        expect(page.get_by_text('Password')).to_be_visible()
        error_text = page.locator('p')
        expect(error_text).to_be_hidden()
        for input in page.get_by_role('textbox').all():
            input.fill(input_value)
        page.get_by_text('Submit').click()
        if fake:
            expect(error_text).to_be_visible()
            page.get_by_text('Back').click()
        else:
            expect(page.get_by_text('Log out')).to_be_visible()
            expect(page.get_by_role("heading")).to_have_text(expected=input_value)

    form(form_name='Login', input_value='hej', fake=True)
    form(form_name='Register', input_value='hej', fake=True)
    form(form_name='Register', input_value='userpassword', fake=False)

    page.get_by_text('Log out').click()
    expect(page.get_by_role('heading')).to_have_text(expected='Log in')

    form(form_name='Login', input_value='userpassword', fake=False)

    _ = subprocess.run(["sudo", "chmod", "-R", "777", "./data"], check=False)

    zip_file_value = ''
    for _ in range(10):
        zip_file_value += str(random.randint(1, 10))
    with open("./data/downloads/game-client.txt", "w") as f:
        _ = f.write(zip_file_value)
    with zipfile.ZipFile("./data/downloads/game-client.zip", mode="w", compression=zipfile.ZIP_DEFLATED) as zip_file:
        zip_file.write("./data/downloads/game-client.txt", arcname="game-client.txt")

    with page.expect_download() as download_info:
        page.get_by_test_id('download-client').click()
    download = download_info.value

    download.save_as('./data/hej.zip')
    with zipfile.ZipFile('./data/hej.zip', 'r') as zip_ref:
        zip_ref.extractall('./data')
    with open("./data/game-client.txt", "r") as f:
        val = f.read()
        assert val == zip_file_value
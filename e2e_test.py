import subprocess
import requests
import json
import pytest
import socket
import time
import websocket
import threading
import random
import re
from playwright.sync_api import Page, expect

#To run this test run the following command in main directory:
#pytest -s e2e_test.py (-s flag shows all logs)
#or
#pytest -s e2e_test.py::{name_of_specific_test}

#globals
PLAYERS_PER_ROOM = 2
CONTAINERS = 2

@pytest.fixture()
def setup_services():
  subprocess.run(['sudo', 'rm', '-rf', './data'])
  subprocess.run(['make', 'up', 'UP_ARGS=-d'])
  def stream_logs():
    subprocess.run(['docker', 'compose', 'logs', '-f'])
  log_thread = threading.Thread(target=stream_logs, daemon=True)
  log_thread.start()
  time.sleep(4)
  yield
  subprocess.run(['make', 'down', 'DOWN_ARGS=-t 0 -v']) 

def log(args):
    msg = ''
    for a in args:
        msg += str(a) + ' '
    print(f'\033[36m[E2E TEST LOGGER]: {msg}\033[0m')

def restart_container(containers, off_time):
    kill_container(containers)
    time.sleep(off_time)
    for c in containers:
        log(['restarting container', c])
        subprocess.run(['docker', 'start', c])
def kill_container(containers):
    for c in containers:
        log(['killing container', c])
        subprocess.run(['docker', 'kill', c])

def get_containers_by_image(image):
    containers = []

    result = subprocess.run(
        ["docker", "ps", "-q", "--filter", f"ancestor={image}"],
        capture_output=True,
        text=True,
    )
    container_ids = result.stdout.strip()
    for c_id in container_ids.splitlines():
        containers.append(c_id)

    return containers

def register_user(username):
    response = requests.post(
        'http://localhost:3000/api/register',
        json={'login': username,
        'password': 'passawubasd!!'
    })

    assert response.json() == {}
    token = response.cookies.get('session')
    return token

def ping_game_server(host, port):
    time.sleep(random.random())
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as sock:
        sock.settimeout(3)
            
        message = b"PING"
        sock.sendto(
            message, 
            (host, port)
        )
            
        data, addr = sock.recvfrom(1024)
        assert data[:4] == b"PING"

        #returns match config
        return json.loads(data[4:].decode('utf-8'))

def queue_user(token, username, ws_timeout):
    payload = None
    match_found = False
    def on_message(ws, message):
        nonlocal payload, match_found
        payload = json.loads(message)
        if "host" in payload: 
            match_found = True

    ws = websocket.WebSocketApp(
        "ws://localhost:3000/api/connection",
        header={"cookie": f"session={token}"},
        on_message=on_message,
    )
    threading.Timer(ws_timeout, ws.close).start()
    ws.run_forever()
    if not match_found:
        return False, None
    
    log([username, ": Queued user payload", payload])

    assert payload["login"] == username

    match_config = ping_game_server(
        payload["host"], 
        int(payload["port"])
    )
    players_in_match = match_config["players"]
    match_tokens = []
    for p in players_in_match:
        match_tokens.append(p["matchAuthToken"])
    
    assert payload["matchAuthToken"] in match_tokens

    return True

def add_n_users(n):
    #registering
    tokens = []
    usernames = []
    for i in range(n):
        usernames.append('user' + str(i))
        tokens.append(register_user(usernames[-1]))
        assert len(tokens[-1]) > 0

    #should be no results yet
    results = requests.get(
        'http://localhost:3000/api/results',
        cookies={'session': tokens[0]}
    ).json()

    assert "matches" in results
    assert len(results["matches"]) == 0

    #adding to waitlist and matching players
    threads = []
    matched_players = 0
    def add(token, username):
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

        assert "matches" in results

        if len(results["matches"]) == 0:
            players_without_results += 1
            continue

        assert len(results["matches"]) == 1
        match = results["matches"][0]
        log(["match result for ", usernames[i], match])

        assert match["canceled"] == False
        assert match["details"] != None

    assert players_without_results == n % PLAYERS_PER_ROOM

@pytest.mark.parametrize("n", [1, 2, 5, 6])
def test_add_users(setup_services, n):
    add_n_users(n)

@pytest.mark.parametrize("kill_before_queue", [True, False])
def test_crash_server_manager(setup_services, kill_before_queue):
    #registering
    tokens = []
    usernames = []
    for i in range(2):
        usernames.append('user' + str(i))
        tokens.append(register_user(usernames[-1]))

    threads = []

    def add(token):
        ws = websocket.WebSocketApp(
        "ws://localhost:3000/api/connection",
        header={"cookie": f"session={token}"},
        )

        ws.run_forever()

    #restarting container while matchmaking
    kill = threading.Thread(
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
        results = requests.get('http://localhost:3000/api/results', cookies={'session': tokens[i]}).json()
        assert "matches" in results
        assert len(results["matches"]) == 1
        match = results["matches"][0]
        assert match["canceled"] == True

def test_crash_game_server(setup_services):
    #registering
    tokens = []
    usernames = []
    for i in range(2):
        usernames.append('user' + str(i))
        tokens.append(register_user(usernames[-1]))

    threads = []

    #find all containers of image game-server
    containers = get_containers_by_image('game-server')

    def add(token):
        ws = websocket.WebSocketApp(
        "ws://localhost:3000/api/connection",
        header={"cookie": f"session={token}"},
        )

        ws.run_forever()

    #restart containers when match is running
    kill = threading.Thread(
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

        assert "matches" in results
        assert len(results["matches"]) == 1
        match = results["matches"][0]
        log(["wynik ", match])
        assert match["canceled"] == True

def test_browser(setup_services, page: Page):
    page.goto("http://localhost:3000")

    #make sure we are in main page and go to login page
    expect(page.get_by_text('Log out')).to_be_visible()
    page.get_by_text('Login').click()

    #make sure we are in login page and try to log in
    expect(page.get_by_text('Password')).to_be_visible()
    error_text = page.locator('p')
    expect(error_text).to_be_hidden()
    for input in page.get_by_role('textbox').all():
        input.fill('hej')
    page.get_by_text('Submit').click()
    expect(error_text).to_be_visible()
    page.get_by_text('Back').click()

    #make sure we are in main page and go to register page
    expect(page.get_by_text('Log out')).to_be_visible()
    page.get_by_text('Register').click()

    #make sure we are in register page and register
    expect(page.get_by_text('Password')).to_be_visible()
    error_text = page.locator('p')
    expect(error_text).to_be_hidden()
    for input in page.get_by_role('textbox').all():
        input.fill('hej')
    page.get_by_text('Submit').click()
    expect(error_text).to_be_visible()
    for input in page.get_by_role('textbox').all():
        input.fill('userandpassword')
    page.get_by_text('Submit').click()

    #we should be in main page and logged in
    expect(page.get_by_text('Log out')).to_be_visible()
    expect(page.get_by_role("heading")).to_have_text('userandpassword')
    page.get_by_text('Log out').click()
    expect(page.get_by_role('heading')).to_have_text('Log in')
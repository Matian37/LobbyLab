import subprocess
import requests
import json
import pytest
import socket
import time
import websocket
import threading
import random

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

def register_user(username):
    response = requests.post('http://localhost:3000/api/register', json={'login': username, 'password': 'passawubasd!!'})
    assert response.json() == {}
    token = response.cookies.get('session')
    return token

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
        return False
        
    assert payload["login"] == username

    print("payload " + username + " " + str(payload))

    time.sleep(random.random())
    with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as sock:
        sock.settimeout(3)
            
        message = b"PING"
        sock.sendto(message, (payload["host"], int(payload["port"])))
            
        data, addr = sock.recvfrom(1024)
        assert data == b"PING"
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
    results = requests.get('http://localhost:3000/api/results', cookies={'session': tokens[0]}).json()
    assert "matches" in results
    assert len(results["matches"]) == 0

    #adding to waitlist and matching players
    threads = []
    matched_players = 0
    def add(token, username):
        nonlocal matched_players
        #if some players have to wait for a free worker then ws_timeout has to be longer
        ws_timeout = 5
        if n > PLAYERS_PER_ROOM * CONTAINERS:
            ws_timeout = 15 * CONTAINERS
        if queue_user(token, username, ws_timeout) == True:
            matched_players += 1
    for i in range(n):
        threads.append(threading.Thread(target=add, args=(tokens[i], usernames[i])))
        threads[-1].start()
    for thr in threads:
        thr.join()
    
    #some players might not be matched
    assert matched_players == n - n % PLAYERS_PER_ROOM

    #reading results
    time.sleep(6)
    players_without_results = 0
    for i in range(n):
        results = requests.get('http://localhost:3000/api/results', cookies={'session': tokens[i]}).json()
        assert "matches" in results
        if len(results["matches"]) == 0:
            players_without_results += 1
            continue

        assert len(results["matches"]) == 1
        match = results["matches"][0]
        assert match["canceled"] == False
        assert match["details"] != None

    assert players_without_results == n % PLAYERS_PER_ROOM
#no matches
def test_add_1_user(setup_services):
    add_n_users(1)
#one container filled
def test_add_2_users(setup_services):
    add_n_users(2)
#one container filled and one waiting user
def test_add_3_users(setup_services):
    add_n_users(3)
#both containers filled
def test_add_4_users(setup_services):
    add_n_users(4)
#both containers filled and one waiting user
def test_add_5_users(setup_services):
    add_n_users(5)
#both containers filled and one container need to be freed
def test_add_6_users(setup_services):
    add_n_users(6)
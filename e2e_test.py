import subprocess
import requests
import json
import pytest
import socket
import time
import websocket
import threading

@pytest.fixture()
def setup_services():
  subprocess.run(['make', 'up', 'UP_ARGS=-d'])
  yield
  subprocess.run(['make', 'down', 'DOWN_ARGS=-t 0 -v']) 

def add_user(username):
    response = requests.post('http://localhost:3000/api/register', json={'login': username, 'password': '123456'})
    assert response.json() == {}
    token = response.cookies.get('session')

    payload = None
    match_found = False
    def on_message(ws, message):
        nonlocal payload, match_found
        payload = json.dumps(message)
        if "host" in payload: 
            match_found = True
    def on_close(ws, close_status_code, close_msg):
        nonlocal match_found
        assert match_found == True

    ws = websocket.WebSocketApp(
        "ws://localhost:3000/api/connection",
        header={"cookie": f"session={token}"},
        on_message=on_message,
        on_close=on_close
    )
    ws.run_forever()
    return payload, token

def test_add2users(setup_services):
    results = {}
    def add(username):
        results[username] = add_user(username)
    t1 = threading.Thread(target=add, args=('user1',))
    t2 = threading.Thread(target=add, args=('user2',))
    t1.start()
    t2.start()
    t1.join()
    t2.join()
    
    payload1, token1 = results['user1']
    payload2, token2 = results['user2']
    assert payload1["username"] == 'user1'
    assert payload2["username"] == 'user2'
    assert payload1["host"] == payload2["host"]
    assert payload1["port"] == payload2["port"]
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.settimeout(2)
    sock.sendto('hello'.encode(), (payload1["host"], payload1["port"]))
    try:
        sock.recvfrom(1024)
    except socket.timeout:
        assert False
    print('waiting for match to end')
    time.sleep(15)
    print('saving results')
    results = requests.get('http://localhost:3000/api/results', cookies={'session': token1}).json()
    assert "matches" in results
    assert results.length == 1

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
    response = requests.post('http://localhost:3000/api/register', json={'login': username, 'password': 'yasbdhuabsdudbhsa123!!'})
    assert response.json() == {}
    token = response.cookies.get('session')
    return token

def queue_user(token):
    payload = None
    match_found = False
    def on_message(ws, message):
        nonlocal payload, match_found
        payload = json.loads(message)
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
    return payload

def test_add2users(setup_services):
    #registering
    token1 = register_user('user1')
    token2 = register_user('user2')
    assert len(token1) > 0 and len(token2) > 0
    print(token1 + ' ' + token2)

    #adding to waitlist
    payloads = {}
    def add(username):
        payloads[username] = queue_user(username)
    t1 = threading.Thread(target=add, args=(token1,))
    t2 = threading.Thread(target=add, args=(token2,))
    t1.start()
    t2.start()
    t1.join()
    t2.join()
    
    payload1 = payloads['user1']
    payload2 = payloads['user2']

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
    assert len(results) == 1

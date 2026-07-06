import subprocess
import requests
import json
import pytest
import socket
import time

@pytest.fixture()
def setup_services():
  subprocess.run(['docker', 'compose', 'up'])
  yield
  subprocess.run(['docker', 'compose', 'down', '-t', '0', '-v']) 

def add_user(username):
    response = requests.post('http://localhost:5173/api/register', json={'login': username, 'password': '123'})
    data = response.json()
    assert data["sukces"] == True
    token = data["msg"]

    response = requests.get(f'http://localhost:5173/api/connection?token={token}', stream=True)
    data = None
    for line in response.iter_lines():
        if line:
            message = line.decode('utf-8')[6:]
            if message == 'ping':
                continue
            data = json.loads(message)
    return data

def test_add2users(setup_services):
    data1 = add_user('user1')
    data2 = add_user('user2')

    assert data1["username"] == 'user1'
    assert data2["username"] == 'user2'
    assert data1["host"] == data2["host"]
    assert data1["port"] == data2["port"]
    
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.settimeout(2)
    sock.sendto('hello'.encode(), (data1["host"], data1["port"]))
    try:
        sock.recvfrom(1024)
    except socket.timeout:
        assert False

    time.sleep(15)

    query = requests.get(f'http://localhost:5173/api/results?login=user1')
    response = query.json()
    assert response == {'players': ['user1', 'user2'], 'winner': 'user1'}
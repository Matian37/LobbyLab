import subprocess
import requests
import json
import pytest

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

    
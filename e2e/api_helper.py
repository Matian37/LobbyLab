from dataclasses import dataclass
from typing import Any

import requests


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
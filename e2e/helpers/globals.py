import logging
import os

logger = logging.getLogger(__name__)
GAME_SERVER_COUNT = 2
PLAYERS_PER_ROOM = 2
GAME_SERVER_IMAGE = "game-server:latest"
GAME_CLIENT_FILENAME = "game-client.zip"
PROJECT_FOLDER = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

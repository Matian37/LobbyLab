import logging
import os
from pathlib import Path

logger = logging.getLogger(__name__)

GAME_SERVER_COUNT = 2
PLAYERS_PER_ROOM = 2
GAME_SERVER_IMAGE = "game-server:latest"
GAME_CLIENT_FILENAME = "game-client.zip"
PROJECT_FOLDER = str(Path(__file__).resolve().parents[2])
PROJECT_TMP_FOLDER = os.path.join(PROJECT_FOLDER, "tmp")

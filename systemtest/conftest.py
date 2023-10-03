from dynaconf import Dynaconf
from pytest import fixture
import os

current_directory = os.path.dirname(os.path.realpath(__file__))

@fixture
def settings():
    test_settings = Dynaconf(
        root_path = current_directory,
        dotenv_path = current_directory,
        settings_files = ["settings.toml",".secrets.toml"],
    )
    yield test_settings

from os import environ
import pytest

from dotenv import load_dotenv, dotenv_values

from pxknowledge_graph.pxgraphdb import PXGraphDB

class Settings:
    def __init__(self):
        load_dotenv(dotenv_path='test.env')
        self.settings = dotenv_values(dotenv_path='test.env')
        self.pxgraphdb = PXGraphDB(
            environ.get('DOCKER_HOST'), 
            environ.get('DOCKER_API_HOST'),
            exp_url=environ.get('KAESER_GRAPHDB_PROD_URL'), exp_user=environ.get('KAESER_GRAPHDB_PROD_USER'), exp_pass=environ.get('KAESER_GRAPHDB_PROD_PASS'),
            imp_url='http://localhost:7200')

@pytest.fixture
def settings():
    return Settings()

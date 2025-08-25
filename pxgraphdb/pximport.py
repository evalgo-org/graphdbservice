import requests
from requests.auth import HTTPBasicAuth
from os import environ,path
import hashlib
import pycurl
import certifi
import urllib
from urllib.parse import urlparse, urlencode
from io import BytesIO
import json
from jinja2 import Template

from pxgraphdb.pxdocker import PXDocker
from pxgraphdb import pxutils
from pxgraphdb.pxgraphdbinfra import PXGraphDBInfra

class PXImportGraphDB(PXGraphDBInfra):
    def graphdb_repo(self, prefix: str, name: str):
        return super(PXImportGraphDB, self).repo(name)
    
    def get_graph_names(self, repo: str = 'Maintenance-EG'):
        return super(PXImportGraphDB, self).graph_names(repo)
    
    def graphdb_repo_api(self, repo: str, data_file: str, conf_file: str = ''):
        return super(PXImportGraphDB, self).insert(repo, data_file, conf_file)

    def graphdb_graph(self, repo: str, graph_url: str):
        return super(PXImportGraphDB, self).graph_insert(repo, graph_url)
    
    def graphdb_repositories(self):
        return super(PXImportGraphDB, self).repositories()
    
    def graphdb_repository_create(self, name: str):
        return super(PXImportGraphDB, self).create(name)
    
    def graphdb_repository_delete(self, name: str):
        return super(PXImportGraphDB, self).delete(name)

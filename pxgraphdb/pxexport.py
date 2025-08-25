import requests
from requests.auth import HTTPBasicAuth
from os import path
from urllib.parse import urlparse, urlencode
import pycurl
import certifi
import urllib
from io import BytesIO
import json
from jinja2 import Template

from pxgraphdb.pxgraphdbinfra import PXGraphDBInfra

class PXExportGraphDB(PXGraphDBInfra):
    def graphdb_repo(self,  prefix: str, name: str):
        return super(PXExportGraphDB, self).repo(name)
    
    def get_graph_names(self, repo: str = 'Maintenance-EG'):
        return super(PXExportGraphDB, self).graph_names(repo)
    
    def graphdb_repo_api(self, repo: str, data_file: str, conf_file: str = ''):
        return super(PXExportGraphDB, self).insert(repo, data_file, conf_file)

    def graphdb_graph(self, repo: str, graph_url: str):
        return super(PXExportGraphDB, self).graph(repo, graph_url)
    
    def graphdb_repositories(self):
        return super(PXExportGraphDB, self).repositories()
    
    def graphdb_repository_create(self, name: str):
        return super(PXExportGraphDB, self).create(name)
    
    def graphdb_repository_delete(self, name: str):
        return super(PXExportGraphDB, self).delete(name)

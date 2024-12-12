from dotenv import load_dotenv
from os import environ
import sys
from prefect import flow
import time
import pycurl
import urllib
from urllib.parse import urlparse

from pxknowledge_graph.pxgraphdb import PXGraphDB

load_dotenv()

@flow(log_prints=True)
def export_import_repos_c5_ke1():
    pxgraphdb.export_import_repos_c5_ke1(['Vestas-demo'])

@flow(log_prints=True)
def local_setup():
    cnt = 'local-import-data'
    pxg = PXGraphDB(environ.get('DOCKER_HOST'), environ.get('DOCKER_API_HOST'), imp_url=environ.get('RST_SRV')+':'+environ.get('RST_SRV_PORT'))
    pxg.default_ports(cnt, {environ.get('RST_SRV_PORT'):environ.get('RST_SRV_PORT')})
    time.sleep(30)
    pxg.imp.graphdb_repository_create('ProductData-MDM-keys-EG')
    pxg.imp.graphdb_repository_create('ProductData-MDM-keys-US')

@flow(log_prints=True)
def local_import(repo: str, graph_url: str, graph_file: str):
    url = environ.get('RST_SRV')+':'+environ.get('RST_SRV_PORT')
    tgt_url = url+"/repositories/"+repo+'/rdf-graphs/service?'+urllib.parse.urlencode({'graph':graph_url})
    print('insert graph', graph_url, 'into repo', repo, 'from file', graph_file, 'into graphdb', tgt_url)
    c = pycurl.Curl()
    c.setopt(c.URL, tgt_url)
    c.setopt(pycurl.HTTPHEADER, ['Accept: application/json', 'Content-Type: application/rdf+xml'])
    c.setopt(c.UPLOAD, 1)
    file = open(graph_file, 'rb')
    c.setopt(c.READDATA, file)
    c.perform()
    status_code = c.getinfo(pycurl.HTTP_CODE)
    c.close()
    return status_code

@flow(log_prints=True)
def local_export_to_binary_rdf(repo: str, graph: str):
    url = environ.get('RST_SRV')+':'+environ.get('RST_SRV_PORT')
    resp_graph = None
    url_2_path = urlparse(graph)
    graph_file = url_2_path.path.replace('/','_') + '.brf'
    graph_url = urllib.parse.urlencode({'graph':graph})
    src_url = f"{url}/repositories/{repo}/rdf-graphs/service?{graph_url}"
    print(src_url)
    c = pycurl.Curl()
    c.setopt(c.VERBOSE, 0)
    c.setopt(c.URL, src_url)
    c.setopt(pycurl.HTTPHEADER, ['Accept: application/x-binary-rdf'])
    with open(graph_file, 'wb') as out:
        c.setopt(c.WRITEDATA, out)
        c.perform()
    print(c.getinfo(pycurl.HTTP_CODE))
    return {'repo':repo, 'file':graph_file, 'graph':graph}

def list_repos_c5():
    pxgraphdb.export_import_re

if __name__ == '__main__':
    args = sys.argv[1:]
    if len(args) > 0:
        if args[0] == 'export_import_repos_c5_ke1':
            export_import_repos_c5_ke1()
        elif args[0] == 'local-setup':
            local_setup()
        elif args[0] == 'local-import':
            local_import(args[1],args[2],args[3])
        elif args[0] == 'local-export-to-binary':
            local_export_to_binary_rdf(args[1],args[2])
    else:
        print("poetry run python app.py [option]")
        print("options:")
        print("  export_import_repos_c5_ke1")
        print("  local-setup")
        print("  local-import-graph <repo> <graph_url> <graph_file>")
        print("  local-export-to-binary <repo> <graph_url>")

# poetry run python app.py local-import ProductData-MDM-keys-EG 'https://data.kaeser.com/KKH/ARTICLES' ~/Downloads/statements.rdf
# poetry run python app.py local-export-to-binary ProductData-MDM-keys-EG 'https://data.kaeser.com/KKH/ARTICLES'

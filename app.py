from dotenv import load_dotenv
from os import environ
import sys
import time
import pycurl
import urllib
from urllib.parse import urlparse

from pxgraphdb.pxgraphdb import PXGraphDB

load_dotenv()

def local_setup():
    cnt = 'local-import-data'
    pxg = PXGraphDB(environ.get('DOCKER_HOST'), environ.get('DOCKER_API_HOST'), imp_url=environ.get('RST_SRV')+':'+environ.get('RST_SRV_PORT'))
    pxg.default_ports(cnt, {environ.get('RST_SRV_PORT'):environ.get('RST_SRV_PORT')})
    time.sleep(30)
    pxg.imp.graphdb_repository_create('ProductData-MDM-keys-EG')
    pxg.imp.graphdb_repository_create('ProductData-MDM-keys-US')

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

def migrate_graphdb(exp_url: str, imp_url: str):
    pxg = None
    if exp_url == "http://graphdb.buerkert.px":
        pxg = PXGraphDB(environ.get('DOCKER_HOST'), environ.get('DOCKER_API_HOST'), exp_url=exp_url, exp_user="admin", exp_pass="J3iDMalh7ak=", imp_url=imp_url)
    else:
        pxg = PXGraphDB(environ.get('DOCKER_HOST'), environ.get('DOCKER_API_HOST'), exp_url=exp_url, imp_url=imp_url)
    pxg.exp_repos = pxg.exp.graphdb_repositories()
    repos = []
    for repo in pxg.exp_repos["results"]["bindings"]:
        repos.append(repo["id"]["value"])
    # print(repos)
    for imp in pxg.export_import_repos(prefix="migrate-graphdb", repos=repos):
        print(imp)

if __name__ == '__main__':
    servers_map = [
        # {"exp_url": "http://build-001.graphdb.px:7200", "imp_url":"http://tmp-build-001.graphdb.px:7200"},
        {"exp_url": "http://build-002.graphdb.px:7200", "imp_url":"http://tmp-build-002.graphdb.px:7200"},
        # {"exp_url": "http://build-003.graphdb.px:7200", "imp_url":"http://tmp-build-003.graphdb.px:7200"},
        # {"exp_url": "http://build-004.graphdb.px:7200", "imp_url":"http://tmp-build-004.graphdb.px:7200"},
        # {"exp_url": "http://build-005.graphdb.px:7200", "imp_url":"http://tmp-build-005.graphdb.px:7200"},
        # {"exp_url": "http://build-006.graphdb.px:7200", "imp_url":"http://tmp-build-006.graphdb.px:7200"},
        # {"exp_url": "http://build-007.graphdb.px:7200", "imp_url":"http://tmp-build-007.graphdb.px:7200"},
        # {"exp_url": "http://build-008.graphdb.px:7200", "imp_url":"http://tmp-build-008.graphdb.px:7200"},
        # {"exp_url": "http://build-009.graphdb.px:7200", "imp_url":"http://tmp-build-009.graphdb.px:7200"},
        # {"exp_url": "http://build-010.graphdb.px:7200", "imp_url":"http://tmp-build-010.graphdb.px:7200"},
        # {"exp_url": "http://dev.graphdb.px:7200", "imp_url":"http://tmp-graphdb.buerkert.px:7200"},
        # {"exp_url": "http://demo.graphdb.px:7200", "imp_url":"http://tmp-graphdb.buerkert.px:7200"},
        # {"exp_url": "http://ke1.graphdb.px:7200", "imp_url":"http://tmp-graphdb.buerkert.px:7200"},
        # {"exp_url": "http://ke2.graphdb.px:7200", "imp_url":"http://tmp-graphdb.buerkert.px:7200"},
        # {"exp_url": "http://ke-ingest.graphdb.px:7200", "imp_url":"http://tmp-graphdb.buerkert.px:7200"},
        # {"exp_url": "http://ke-test.graphdb.px:7200", "imp_url":"http://tmp-graphdb.buerkert.px:7200"},
        # {"exp_url": "http://graphdb.buerkert.px", "imp_url":"http://tmp-graphdb.buerkert.px:7200"}
    ]
    for srv in servers_map:
        print(srv)
        migrate_graphdb(**srv)

    # args = sys.argv[1:]
    # if len(args) > 0:
    #     if args[0] == 'export_import_repos_c5_ke1':
    #         export_import_repos_c5_ke1()
    #     elif args[0] == 'local-setup':
    #         local_setup()
    #     elif args[0] == 'local-import':
    #         local_import(args[1],args[2],args[3])
    #     elif args[0] == 'local-export-to-binary':
    #         local_export_to_binary_rdf(args[1],args[2])
    # else:
    #     print("poetry run python app.py [option]")
    #     print("options:")
    #     print("  export_import_repos_c5_ke1")
    #     print("  local-setup")
    #     print("  local-import-graph <repo> <graph_url> <graph_file>")
    #     print("  local-export-to-binary <repo> <graph_url>")

# poetry run python app.py local-import ProductData-MDM-keys-EG 'https://data.kaeser.com/KKH/ARTICLES' ~/Downloads/statements.rdf
# poetry run python app.py local-export-to-binary ProductData-MDM-keys-EG 'https://data.kaeser.com/KKH/ARTICLES'

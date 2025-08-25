import urllib3
from urllib3.util import make_headers
import urllib
from uuid import uuid4
from os import path
from jinja2 import Template
from pathlib import Path
from urllib.parse import urlparse, urlencode

class PXGraphDBInfra:
    def __init__(self, url: str, user: str = "", passwd: str = ""):
        self.url = url
        self.user = user
        self.passwd = passwd
        self.http = urllib3.PoolManager(num_pools=10, maxsize=2)
        if user != "" and passwd != "":
            self.reload_headers()
        else:
            self.headers = {}
    def reload_headers(self):
        self.headers = make_headers(basic_auth=self.user+":"+self.passwd)
    def repositories(self):
        repos_url = f"{self.url}/repositories"
        self.headers.update({"Accept":"application/json"})
        resp = self.http.request("GET", repos_url, headers=self.headers)
        return resp.json()
    def create(self, name: str):
        create_url = f"{self.url}/rest/repositories"
        tmpl_content = None
        tmpl_file = f"{name}.ttl"
        with open(path.dirname(__file__)+path.sep+'config'+path.sep+'new_repository_config.ttl.jinja') as tmpl:
            tmpl_content = Template(tmpl.read())
        with open(tmpl_file, 'w') as wtmpl:
            wtmpl.write(tmpl_content.render(repository_id=name))
        res = self.http.request_encode_body("POST", create_url, headers=self.headers, fields={"config": (tmpl_file, open(tmpl_file).read(), "text/turtle")})
        Path(tmpl_file).unlink()
        return res
    def repo(self, name:str):
        config_url = f"{self.url}/rest/repositories/{name}/download-ttl"
        config_file = str(uuid4())+"-"+name+".conf.ttl"
        self.headers.update({"Accept":"text/turtle"})
        cresp = self.http.request("GET", config_url, headers=self.headers,preload_content=False)
        with open(config_file, "wb") as cf:
            for cchunk in cresp.stream(1024):
                cf.write(cchunk)
            cresp.release_conn()
        data_url = f"{self.url}/repositories/{name}/statements"
        data_file = str(uuid4())+"-"+name+".brf"
        self.headers.update({"Accept":"application/x-binary-rdf"})
        dresp = self.http.request("GET", data_url, headers=self.headers,preload_content=False)
        with open(data_file, "wb") as df:
            for dchunk in dresp.stream(4*1024*1024):
                df.write(dchunk)
            dresp.release_conn()
        return {"config":config_file, "data":data_file}
    def repo_size(self, name: str):
        size_url = f"{self.url}/repositories/"+name+"/size"
        self.headers.update({"Accept":"application/json"})
        resp = self.http.request("GET", size_url, headers=self.headers,preload_content=False)
        return resp.json()
    def delete(self, name: str):
        del_url = f"{self.url}/rest/repositories/{name}"
        return self.http.request("DELETE", del_url, headers=self.headers)
    def graphs(self, repo: str):
        graphs_url = f"{self.url}/repositories/{repo}/rdf-graphs"
        self.headers.update({"Accept":"application/json"})
        resp = self.http.request("GET", graphs_url, headers=self.headers)
        all_graphs = resp.json()
        key = all_graphs['head']['vars'][0]
        return list(map(lambda g: g[key]['value'], all_graphs['results']['bindings']))
    def create_from_turtle(self, repo:str, turtle: str):
        create_url = f"{self.url}/rest/repositories"
        with open(turtle, 'rb') as ttlf:
            ttl = ttlf.read()
            resp = self.http.request_encode_body("POST", create_url, headers=self.headers,fields={"config": (turtle, ttl, "text/turtle")})
            return resp
    def insert(self, repo:str, data: str, turtle: str = ''):
        created = None
        if turtle != '':
            created = self.create_from_turtle(repo, turtle)
        insert_url = f"{self.url}/repositories/{repo}/statements"
        self.headers.update({"Accept":"application/json"})
        self.headers.update({"Content-Type":"application/x-binary-rdf"})
        with open(data, 'rb') as brdff:
            brdf = brdff.read()
            resp = self.http.request("POST", insert_url, headers=self.headers, body=brdf, encode_multipart=False)
            return {"created": created, "inserted": resp}
    def repo_query(self, repo: str, query_file: str):
        repo_url = f"{self.url}/repositories/{repo}"
        resp = None
        self.headers.update({"Accept":"application/sparql-results+json"})
        self.headers.update({"Content-Type":"application/x-www-form-urlencoded"})
        with open(path.dirname(__file__)+path.sep+'config'+path.sep+query_file, 'r') as rq:
            query = rq.read()
            resp_raw = self.http.request("POST", repo_url, headers=self.headers, fields={"query":query}, encode_multipart=False)
        resp = resp_raw.json()
        resp_raw.release_conn()
        return list(map(lambda repo: repo['g']['value'], resp['results']['bindings']))
    def graph_names(self, repo: str):
        return self.repo_query(repo, 'get_graph_names.rq')
    def triple_count_by_graph(self, repo: str):
        return self.repo_query(repo, 'triples_count_by_graph.rq')
    def graph_exists(self, repo: str, graph: str):
        graphs = self.graphs(repo)
        return graph in graphs
    def graph(self, repo: str, graph: str):
        resp_graph = None
        url_2_path = urlparse(graph)
        graph_file = url_2_path.path.replace('/','_') + '.brf'
        graph_url = urllib.parse.urlencode({'graph':graph})
        g_url = f"{self.url}/repositories/{repo}/rdf-graphs/service?{graph_url}"
        self.headers.update({"Accept":"application/x-binary-rdf"})
        resp_raw = self.http.request("GET", g_url, headers=self.headers, preload_content=False)
        with open(graph_file, "wb") as gf:
            for gchunk in resp_raw.stream(4*1024*1024):
                gf.write(gchunk)
            resp_raw.release_conn()
        return {"repo":repo, "file":graph_file, "graph":graph}
    def graph_insert(self, repo: str, graph: str):
        resp_graph = None
        url_2_path = urlparse(graph)
        graph_file = url_2_path.path.replace('/','_') + '.brf'
        graph_url = urllib.parse.urlencode({'graph':graph})
        g_url = f"{self.url}/repositories/{repo}/rdf-graphs/service?{graph_url}"
        self.headers.update({"Content-Type":"application/x-binary-rdf"})
        with open(graph_file, 'rb') as fbrdf:
            graph_brdf = fbrdf.read()
            return self.http.request("POST", g_url, headers=self.headers, body=graph_brdf)

from typing import Any
from os import environ,path,sep
from pathlib import Path
import requests
import time
from requests.auth import HTTPBasicAuth
from jinja2 import Template

from pxgraphdb.pxdocker import PXDocker
from pxgraphdb.pxexport import PXExportGraphDB
from pxgraphdb.pximport import PXImportGraphDB
from pxgraphdb.pxinfisical import PXInfisical

PX_GRAPHDB_NETWORK='env-px'
PX_GRAPHDB_VOLUME='env-px-graphdb-data'
PX_GRAPHDB_IMAGE='ontotext/graphdb'
PX_GRAPHDB_VERSION='10.6.3'

class PXGraphDB:
    def __init__(self, docker_host: str = '', docker_api_host: str = '', exp_url: str = '', exp_user: str = '', exp_pass: str = '', imp_url: Any = None, imp_user: str = '', imp_pass: str = ''):
        if docker_host != '' and docker_api_host != '':
            self.pxd = PXDocker(docker_host, docker_api_host)
        self.exp = PXExportGraphDB(exp_url, exp_user, exp_pass)
        if isinstance(imp_url, list):
            self.imp = []
            for imp in imp_url:
                # that one needs to be fixed for multiple target credentials
                self.imp.append(PXImportGraphDB(imp, imp_user, imp_pass))
        else:
            self.imp = PXImportGraphDB(imp_url, imp_user, imp_pass)
    # fixing later for supporting multiple export targets
    def load_imp(self, url: Any , user: str = '', passwd: str = ''):
        if isinstance(url, list):
            self.imp = []
            self.imp_repos = []
            for i_target in url:
                # that one needs to be fixed for multiple target credentials
                new_i = PXImportGraphDB(i_target, user, passwd)
                self.imp_repos.append(new_i.graphdb_repositories())
                self.imp.append(new_i)
        else:
            self.imp.url = url
            self.imp.username = user
            self.imp.password = passwd
            self.imp.reload_headers()
            self.imp_repos = self.imp.graphdb_repositories()
    def load_exp(self, url: str , user: str = '', passwd: str = ''):
        self.exp.url = url
        self.exp.username = user
        self.exp.password = passwd
        self.exp.reload_headers()
        self.exp_repos = self.exp.graphdb_repositories()
    def default(self, name: str):    
        gdb_pull = self.pxd.image_pull(PX_GRAPHDB_IMAGE, PX_GRAPHDB_VERSION)
        # todo: check the pull result
        gdb_vol = self.pxd.volume_create(name+'-data', 'local')
        # todo: check pg_vol
        gdb_nw = self.pxd.network_create(PX_GRAPHDB_NETWORK, 'bridge')
        # todo: check pg_nw
        mounts = [self.pxd.mount_create(name+'-data','/opt/graphdb/home')]
        # todo: check mounts
        container = self.pxd.container_run(':'.join([PX_GRAPHDB_IMAGE, str(PX_GRAPHDB_VERSION)]), name, gdb_nw['name'], mounts, {}, ['-Dgraphdb.home=/opt/graphdb/home', '-Dgraphdb.workbench.maxUploadSize=1024000000'])
        # todo: check container
        return {
            'image': ':'.join([PX_GRAPHDB_IMAGE, PX_GRAPHDB_VERSION]),
            'network': PX_GRAPHDB_NETWORK,
            'volume': name+'-data',
            'container': container}
    def graphdb_repositories(self, imp: bool = True):
        results = None
        if imp:
            results = self.imp.graphdb_repositories()
        else:
            results = self.exp.graphdb_repositories()
        return list(map(lambda repo: repo['id']['value'] , results['results']['bindings']))
    def graphdb_repo_create(self, repo:str, inference: str = "empty", imp: bool = True):
        with open(str(Path(__file__).parent.absolute())+sep+'new_repository_config.ttl.jinja') as f:
            tmpl = Template(f.read())
            tmpFilePath = str(Path(__file__).parent.absolute())+sep+"tmp.create.repository.ttl"
            with open(tmpFilePath, "w") as tmplFile:
                tmplFile.write(tmpl.render(repository_id=repo, inference=inference))
        if imp:
            return self.imp.create_from_turtle(repo, tmpFilePath)
        else:
            return self.exp.create_from_turtle(repo, tmpFilePath)
    def default_ports(self, name: str, ports: dict, rebuild: bool = False):
        resp_dict = {
            'image': ':'.join([PX_GRAPHDB_IMAGE, PX_GRAPHDB_VERSION]),
            'network': PX_GRAPHDB_NETWORK,
            'volume': name+'-data',
            'ports': ports}
        found_cnt = self.pxd.container_by_name(name)
        if found_cnt is not None:
            if rebuild:
                resp_dict['rebuild'] = rebuild
                self.pxd.container_remove(name)
            else:
                resp_dict['rebuild'] = False
                resp_dict['container'] = found_cnt
                return resp_dict
        gdb_pull = self.pxd.image_pull(PX_GRAPHDB_IMAGE, PX_GRAPHDB_VERSION)
        # todo: check the pull result
        gdb_vol = self.pxd.volume_create(name+'-data', 'local')
        # todo: check pg_vol
        gdb_nw = self.pxd.network_create(PX_GRAPHDB_NETWORK, 'bridge')
        # todo: check pg_nw
        mounts = [self.pxd.mount_create(name+'-data','/opt/graphdb/home')]
        # todo: check mounts
        container = self.pxd.container_run_ports(':'.join([PX_GRAPHDB_IMAGE, str(PX_GRAPHDB_VERSION)]), name, gdb_nw['name'], mounts, ports, {}, ['-Dgraphdb.home=/opt/graphdb/home', '-Dgraphdb.workbench.maxUploadSize=1024000000'])
        # todo: check container
        resp_dict['container'] = container
        resp_dict['rebuild'] = True
        return resp_dict
    def default_remove(self, name: str):
        container = self.pxd.container_by_name(name)
        stopped = container['container'].stop()
        # todo: check stopped
        removed = container['container'].remove()
        # todo: check removed
        return removed
    def export_import_repos(self, prefix:str, repos: list[str]):
        resp_repos = list(map(lambda r: {'repo': r, 'files': self.exp.graphdb_repo(prefix=prefix, name=r)}, repos))
        # resp_repos = [{'repo': 'Liebherr-Demo', 'files': {'config': '1aecb984-1a18-4b2c-8e8a-0edac765c6f4-Liebherr-Demo.conf.ttl', 'data': 'f51b9773-42ae-48b3-b742-8ae14b45fe76-Liebherr-Demo.brf'}}, {'repo': 'Maintenance-001', 'files': {'config': '4a24f184-b3a8-4972-ae4c-8ce764e66ad4-Maintenance-001.conf.ttl', 'data': 'a67dedfa-ede5-4bcc-b4f4-abe849c7042c-Maintenance-001.brf'}}, {'repo': 'Consumption-Navigator-NL-Intern-All', 'files': {'config': '64c6cfd8-7556-4741-a7a1-57da79236dd1-Consumption-Navigator-NL-Intern-All.conf.ttl', 'data': '26a7ee8d-5ffd-4ffd-930a-1930ce77f5b6-Consumption-Navigator-NL-Intern-All.brf'}}, {'repo': 'Consumption-Navigator-DE-Intern-All', 'files': {'config': '5cbcbe87-afb9-4091-9e1a-7bd1a893e3bb-Consumption-Navigator-DE-Intern-All.conf.ttl', 'data': 'e979926f-9b60-4362-b172-adda8ca19c70-Consumption-Navigator-DE-Intern-All.brf'}}, {'repo': 'Transformation-Navigator-NL-Intern-All', 'files': {'config': '8e6e0fe5-65dd-4998-bf02-9583342dc996-Transformation-Navigator-NL-Intern-All.conf.ttl', 'data': 'be448c6b-0fca-4cd7-8c34-f37bd7829bc3-Transformation-Navigator-NL-Intern-All.brf'}}, {'repo': 'Transformation-Navigator-DE-Intern-All', 'files': {'config': 'a43f9f1d-7d01-4b99-b241-877fbf02135b-Transformation-Navigator-DE-Intern-All.conf.ttl', 'data': '31e9364f-96f1-46aa-b2c0-9fa595644206-Transformation-Navigator-DE-Intern-All.brf'}}]
        if isinstance(self.imp, list):
            responses = []
            for i_target in self.imp:
                responses.append(list(map(lambda r: i_target.graphdb_repo_api(r['repo'], r['files']['data'], r['files']['config']), resp_repos)))
            return responses
        else:
            # resp_created = list(map(lambda r: self.imp.create_from_turtle(r['repo'], r['files']['config']), resp_repos))
            resp_imported = list(map(lambda r: self.imp.graphdb_repo_api(r['repo'], r['files']['data'], r['files']['config']), resp_repos))
            # resp_import = list(map(lambda r: self.imp.graphdb_repo_api(r['repo'], r['files']['data'], r['files']['config']), resp_repos))
            # return {"created": resp_created, "inserted": resp_imported}
            return resp_imported
    def graph_import_with_check(self, repo: str, graph: str):
        imp_resp = self.imp.graphdb_graph(repo, graph)
        if not self.exp.graph_exists(repo, graph):
            print(f"reimport {repo} graph {graph}...")
            return self.imp.graphdb_graph(repo, graph)
    def export_import_repos_graphs(self, prefix: str, src_repo: str, graphs: list[str], tgt_repo: str):
        export_responses = list(map(lambda g: self.exp.graph(src_repo, g), graphs))
        list(map(lambda g: print("exported graph", g), export_responses))
        import_responses = list(map(lambda g: {'graph': g['graph'], 'response': self.graph_import_with_check(tgt_repo, g['graph'])}, export_responses))
        list(map(lambda r: print(r['graph'], r['response']), import_responses))
        return import_responses
    
    @staticmethod
    def get_repo_config_template() -> str:
        content = ""
        print()
        with open(str(Path(__file__).parent.absolute())+sep+'new_repository_config.ttl.jinja') as tmpl:
            content = tmpl.read()
        return content

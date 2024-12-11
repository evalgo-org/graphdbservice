from prefect import flow, task
from os import environ,path
import requests
import time
from requests.auth import HTTPBasicAuth
from jinja2 import Template

from pxinfra.pxdocker import PXDocker
from pxinfra.pxbackup import PXBackupGraphDB
from pxinfra.pxrestore import PXRestoreGraphDB
from pxinfra.pxexport import PXExportGraphDB
from pxinfra.pximport import PXImportGraphDB
from pxinfra.pxinfisical import PXInfisical

PX_GRAPHDB_NETWORK='env-px'
PX_GRAPHDB_VOLUME='env-px-graphdb-data'
PX_GRAPHDB_IMAGE='ontotext/graphdb'
PX_GRAPHDB_VERSION='10.6.3'

class PXGraphDB:
    def __init__(self, docker_host: str = '', docker_api_host: str = '', exp_url: str = '', exp_user: str = '', exp_pass: str = '', imp_url: str = '', imp_user: str = '', imp_pass: str = ''):
        if docker_host != '' and docker_api_host != '':
            self.pxd = PXDocker(docker_host, docker_api_host)
        self.exp = PXExportGraphDB(exp_url, exp_user, exp_pass)
        self.imp = PXImportGraphDB(imp_url, imp_user, imp_pass)
        self.bkp = PXBackupGraphDB(exp_url, exp_user, exp_pass)
        self.res = PXRestoreGraphDB(imp_url, imp_user, imp_pass)
    def load_imp(self, url: str , user: str = '', passwd: str = ''):
        self.imp.url = url
        self.imp.username = user
        self.imp.password = passwd
        self.imp_repos = self.imp.graphdb_repositories()
    def load_exp(self, url: str , user: str = '', passwd: str = ''):
        self.exp.url = url
        self.exp.username = user
        self.exp.password = passwd
        self.exp_repos = self.exp.graphdb_repositories()
    def load_res(self, url: str , user: str = '', passwd: str = ''):
        self.res.url = url
        self.res.username = user
        self.res.password = passwd
    def load_bkp(self, url: str , user: str = '', passwd: str = ''):
        self.bkp.url = url
        self.bkp.username = user
        self.bkp.password = passwd
    @task(log_prints=True,persist_result=False)
    def default(self, name: str):    
        gdb_pull = self.pxd.image_pull(PX_GRAPHDB_IMAGE, PX_GRAPHDB_VERSION)
        # todo: check the pull result
        gdb_vol = self.pxd.volume_create(name+'-data', 'local')
        # todo: check pg_vol
        gdb_nw = self.pxd.network_create(PX_GRAPHDB_NETWORK, 'bridge')
        # todo: check pg_nw
        mounts = [self.pxd.mount_create(name+'-data','/opt/graphdb/home')]
        # todo: check mounts
        container = self.pxd.container_run(':'.join([PX_GRAPHDB_IMAGE, str(PX_GRAPHDB_VERSION)]), name, gdb_nw['name'], mounts, {})
        # todo: check container
        return {
            'image': ':'.join([PX_GRAPHDB_IMAGE, PX_GRAPHDB_VERSION]),
            'network': PX_GRAPHDB_NETWORK,
            'volume': name+'-data',
            'container': container}
    @task(log_prints=True,persist_result=False)
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
        container = self.pxd.container_run_ports(':'.join([PX_GRAPHDB_IMAGE, str(PX_GRAPHDB_VERSION)]), name, gdb_nw['name'], mounts, ports, {})
        # todo: check container
        resp_dict['container'] = container
        resp_dict['rebuild'] = True
        return resp_dict
    @task(log_prints=True,persist_result=False)
    def default_remove(self, name: str):
        container = self.pxd.container_by_name(name)
        stopped = container['container'].stop()
        # todo: check stopped
        removed = container['container'].remove()
        # todo: check removed
        return removed
    @flow(log_prints=True,persist_result=False)
    def backup_all(self, prefix: str):
        repos = self.exp.graphdb_repositories()
        return self.bkp.graphdb(prefix, repos)
    @flow(log_prints=True,persist_result=False)
    def backup_restore(self, prefix: str, repos: list[str]):
        bkup = self.bkp.graphdb(prefix, repos)
        return self.res.graphdb(bkup)
    @flow(log_prints=True,persist_result=False)
    def export_import_repos(self, prefix:str, repos: list[str]):
        resp_repos = list(map(lambda r: {'repo': r, 'files': self.exp.graphdb_repo(prefix=prefix, repo=r)}, repos))
        resp_import = list(map(lambda r: self.imp.graphdb_repo_api(r['repo'], r['files']['data'], r['files']['conf']), resp_repos))
        return resp_import
    @task(log_prints=True,persist_result=False)
    def graph_import_with_check(self, repo: str, graph: str, graph_file:str):
        imp_resp = self.imp.graphdb_graph(repo, graph, graph_file)
        if not self.exp.graphdb_graph_exists(repo, graph):
            print(f"reimport {url}::{repo} graph {graph} from file {graph_file}...")
            return self.imp.graphdb_graph(repo, graph, graph_file)
    @flow(log_prints=True,persist_result=False)
    def export_import_repos_graphs(self, prefix: str, src_repo: str, graphs: list[str], tgt_repo: str):
        export_responses = list(map(lambda g: self.exp.graphdb_repo_graph(prefix, src_repo, g), graphs))
        list(map(lambda g: print("exported graph", g), export_responses))
        import_responses = list(map(lambda g: {'graph': g['graph'], 'response': self.graph_import_with_check(tgt_repo, g['graph'], g['file'])}, export_responses))
        list(map(lambda r: print(r['graph'], r['response']), import_responses))
        return import_responses

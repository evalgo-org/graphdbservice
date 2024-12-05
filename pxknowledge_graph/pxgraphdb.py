from prefect import flow, task
from os import environ,path
import requests
import time
from requests.auth import HTTPBasicAuth
from jinja2 import Template

from pxinfra.pxdocker import PXDocker
from pxinfra import pxbackup
from pxinfra import pxrestore
from pxinfra.pxexport import PXExportGraphDB
from pxinfra.pximport import PXImportGraphDB
from pxinfra.pxinfisical import PXInfisical

PX_GRAPHDB_NETWORK='env-px'
PX_GRAPHDB_VOLUME='env-px-graphdb-data'
PX_GRAPHDB_IMAGE='ontotext/graphdb'
PX_GRAPHDB_VERSION='10.6.3'

# environ.get('DOCKER_HOST'), environ.get('DOCKER_API_HOST')

class PXGraphDB:
    def __init__(self, docker_host: str = '', docker_api_host: str = '', exp_url: str = '', exp_user: str = '', exp_pass: str = '', imp_url: str = '', imp_user: str = '', imp_pass: str = ''):
        self.pxd = PXDocker(docker_host, docker_api_host)
        self.exp = PXExportGraphDB(exp_url, exp_user, exp_pass)
        self.imp = PXImportGraphDB(imp_url, imp_user, imp_pass)
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
                self.pxd.container_remove(name)
            else:
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
        return resp_dict
    @task(log_prints=True,persist_result=False)
    def default_remove(self, name: str):
        container = self.pxd.container_by_name(name)
        stopped = container['container'].stop()
        # todo: check stopped
        removed = container['container'].remove()
        # todo: check removed
        return removed
    @task(log_prints=True,persist_result=False)
    def create_repository(self, name: str):
        if not exists_repository(server, name, username, password):
            tmpl_content = None
            tmpl_file = name+'.ttl'
            with open(path.dirname(__file__)+path.sep+'new_repository_config.ttl.jinja') as tmpl:
                tmpl_content = Template(tmpl.read())
            with open(tmpl_file, 'w') as wtmpl:
                wtmpl.write(tmpl_content.render(repository_id=name))
            files = {'config':open(tmpl_file)}
            if username != '' and password != '':
                basic = HTTPBasicAuth(username, password)
                return requests.post(server, files=files, auth=basic)
            return requests.post(server+'/rest/repositories', files=files)
        return None
    @task(log_prints=True,persist_result=False)
    def delete_repository(self, server: str, name: str, username: str = '', password: str = ''):
        if exists_repository(server, name, username, password):
            return requests.delete(server+'/rest/repositories/'+name)
        return None
    @flow(log_prints=True,persist_result=False)
    def backup_restore(self, src_url: str, prefix: str, repos: list[str], src_user:str = '', src_passwd: str = '', tgt_url: str = '', tgt_user: str = '', tgt_passwd: str = ''):
        bkup = pxbackup.graphdb(src_url, prefix, repos, src_user, src_passwd)
        if tgt_user != '' and tgt_passwd != '':
            return pxrestore.graphdb(tgt_url, bkup, username=tgt_user, password=tgt_passwd)
        return pxrestore.graphdb(tgt_url, bkup)
    @flow(log_prints=True,persist_result=False)
    def export_import_repos(self, src_url: str, prefix: str, repos: list[str], src_user: str = '', src_passwd: str = '', tgt_url: str = '', tgt_user: str = '', tgt_passwd: str = ''):
        px_exp = PXExportGraphDB(src_url, src_user, src_passwd)
        resp_repos = list(map(lambda r: {'repo': r, 'files': px_exp.graphdb_repo(prefix=prefix, repo=r)}, repos))
        px_imp = PXImportGraphDB(tgt_url, tgt_user, tgt_passwd)
        resp_import = list(map(lambda r: px_imp.graphdb_repo_api(r['repo'], r['files']['data'], r['files']['conf']), resp_repos))
        return resp_import
    @flow(log_prints=True,persist_result=False)
    def delayed_graph_export(self, url: str, prefix: str, repo: str, graph: str, username: str, password:str):
        time.sleep(5)
        px_exp = PXExportGraphDB(url, username, password)
        return px_exp.graphdb_repo_graph(prefix, repo, graph)
    @flow(log_prints=True,persist_result=False)
    def delayed_graph_import(self, url: str, repo: str, graph: str, graph_file: str, username: str, password:str):
        time.sleep(5)
        px_imp = PXImportGraphDB(url, username, password)
        return px_imp.graphdb_graph(repo, graph, graph_file)
    def filter_error_response(resp: dict, export_responses: list[dict]):
        found_errors = list(filter(lambda graph: True if graph['graph'] == resp['graph'] else False, export_responses))
        if len(found_errors) > 0:
            return found_errors[0]
        return None
    @task(log_prints=True,persist_result=False)
    def graph_import_with_check(self, url: str, repo: str, graph: str, graph_file:str, username: str, passwd: str):
        time.sleep(2)
        px_imp = PXImportGraphDB(url, username, passwd)
        imp_resp = px_imp.graphdb_graph(repo, graph, graph_file)
        px_exp = PXExportGraphDB(url, username, passwd)
        if not px_exp.graphdb_graph_exists(repo, graph):
            print(f"reimport {url}::{repo} graph {graph} from file {graph_file}...")
            time.sleep(2)
            return px_imp.graphdb_graph(repo, graph, graph_file)
    @flow(log_prints=True,persist_result=False)
    def export_import_repos_graphs(self, src_url: str, prefix: str, src_repo: str, graphs: list[str], tgt_url: str, tgt_repo: str, src_user: str = '', src_passwd: str = '', tgt_user: str = '', tgt_passwd: str = ''):
        export_responses = list(map(lambda g: delayed_graph_export(src_url, prefix, src_repo, g, src_user, src_passwd), graphs))
        list(map(lambda g: print("exported graph", g), export_responses))
        import_responses = list(map(lambda g: {'graph': g['graph'], 'response': graph_import_with_check(tgt_url, tgt_repo, g['graph'], g['file'], tgt_user, tgt_passwd)}, export_responses))
        # error_responses = list(filter(lambda resp: True if resp['response'].status_code == 400 else False, import_responses))
        # if len(error_responses) > 0:
        #     retry_err_responses = list(filter(lambda resp: filter_error_response(resp, export_responses), ))
        list(map(lambda r: print(r['graph'], r['response']), import_responses))
        return import_responses

# env-ontotext-graphdb-ke-test @ mercur
# @flow(log_prints=True,persist_result=False)
# def backup_restore_c5_ke_test(repos: list[str]):
#     pxsec = PXInfisical()
#     secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'ke-test')
#     config = pxsec.bytes_to_dict(secrets_file)
#     backup_restore(config['BKP_SRV'], config['BKP_PREFIX'], repos, config['BKP_USER'], config['BKP_PASS'], config['RST_SRV'], config['RST_USER'], config['RST_PASS'])

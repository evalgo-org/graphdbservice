from prefect import flow, task
from os import environ,path
import requests
import time
from requests.auth import HTTPBasicAuth
from jinja2 import Template

from pxinfra.pxdocker import PXDocker
from pxinfra import pxbackup
from pxinfra import pxrestore
from pxinfra import pxexport
from pxinfra import pximport
from pxinfra.pxinfisical import PXInfisical

PX_GRAPHDB_NETWORK='env-px'
PX_GRAPHDB_VOLUME='env-px-graphdb-data'
PX_GRAPHDB_IMAGE='ontotext/graphdb'
PX_GRAPHDB_VERSION='10.6.3'

@task(log_prints=True)
def default(name: str):
    pxd = PXDocker(environ.get('DOCKER_HOST'), environ.get('DOCKER_API_HOST'))
    gdb_pull = pxd.image_pull(PX_GRAPHDB_IMAGE, PX_GRAPHDB_VERSION)
    # todo: check the pull result
    gdb_vol = pxd.volume_create(name+'-data', 'local')
    # todo: check pg_vol
    gdb_nw = pxd.network_create(PX_GRAPHDB_NETWORK, 'bridge')
    # todo: check pg_nw
    mounts = [pxd.mount_create(name+'-data','/opt/graphdb/home')]
    # todo: check mounts
    container = pxd.container_run(':'.join([PX_GRAPHDB_IMAGE, str(PX_GRAPHDB_VERSION)]), name, gdb_nw['name'], mounts, {})
    # todo: check container
    return {
        'image': ':'.join([PX_GRAPHDB_IMAGE, PX_GRAPHDB_VERSION]),
        'network': PX_GRAPHDB_NETWORK,
        'volume': name+'-data',
        'container': container}

@task(log_prints=True)
def default_ports(name: str, ports: dict):
    pxd = PXDocker(environ.get('DOCKER_HOST'), environ.get('DOCKER_API_HOST'))
    gdb_pull = pxd.image_pull(PX_GRAPHDB_IMAGE, PX_GRAPHDB_VERSION)
    # todo: check the pull result
    gdb_vol = pxd.volume_create(name+'-data', 'local')
    # todo: check pg_vol
    gdb_nw = pxd.network_create(PX_GRAPHDB_NETWORK, 'bridge')
    # todo: check pg_nw
    mounts = [pxd.mount_create(name+'-data','/opt/graphdb/home')]
    # todo: check mounts
    container = pxd.container_run_ports(':'.join([PX_GRAPHDB_IMAGE, str(PX_GRAPHDB_VERSION)]), name, gdb_nw['name'], mounts, ports, {})
    # todo: check container
    return {
        'image': ':'.join([PX_GRAPHDB_IMAGE, PX_GRAPHDB_VERSION]),
        'network': PX_GRAPHDB_NETWORK,
        'volume': name+'-data',
        'container': container,
        'ports': ports}

@task(log_prints=True)
def default_remove(name: str):
    pxd = PXDocker(environ.get('DOCKER_HOST'), environ.get('DOCKER_API_HOST'))
    container = pxd.container_by_name(name)
    stopped = container['container'].stop()
    # todo: check stopped
    removed = container['container'].remove()
    # todo: check removed
    return removed

@task(log_prints=True)
def exists_repository(server: str, name: str, username: str = '', password: str = ''):
    resp = None
    if username != '' and password != '':
        basic = HTTPBasicAuth(username, password)
        resp = requests.get(server+'/rest/repositories', auth=basic)
    else:
        resp = requests.get(server+'/rest/repositories')
    found = list(filter(lambda r: True if r['id'] == name else False, resp.json()))
    if len(found) > 0:
        return True
    return False

@task(log_prints=True)
def create_repository(server: str, name: str, username: str = '', password: str = ''):
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

@task(log_prints=True)
def delete_repository(server: str, name: str, username: str = '', password: str = ''):
    if exists_repository(server, name, username, password):
        return requests.delete(server+'/rest/repositories/'+name)
    return None

@flow(log_prints=True)
def backup_restore(src_url: str, prefix: str, repos: list[str], src_user:str = '', src_passwd: str = '', tgt_url: str = '', tgt_user: str = '', tgt_passwd: str = ''):
    bkup = pxbackup.graphdb(src_url, prefix, repos, src_user, src_passwd)
    if tgt_user != '' and tgt_passwd != '':
        return pxrestore.graphdb(tgt_url, bkup, username=tgt_user, password=tgt_passwd)
    return pxrestore.graphdb(tgt_url, bkup)

@flow(log_prints=True)
def export_import_repos(url: str, prefix: str, repos: list[str], container:str, volume:str, network: str, user:str = '', passwd: str = ''):
    pxd = PXDocker(environ.get('DOCKER_HOST'), environ.get('DOCKER_API_HOST'))
    cnt = pxd.container_by_name(container)
    resp_repos = list(map(lambda r: pxexport.graphdb_repo(url=url, prefix=prefix, repo=r, user=user, passwd=passwd), repos))
    cnt['container'].stop()
    resp_import = list(map(lambda r: pximport.graphdb_repo(volume, network, r['conf'], r['data']), resp_repos))
    cnt['container'].start()
    return resp_import

@flow(log_prints=True)
def delayed_graph_export(url: str, prefix: str, repo: str, graph: str, username: str, password:str):
    time.sleep(5)
    return pxexport.graphdb_repo_graph(url, prefix, repo, graph, username, password)

@flow(log_prints=True)
def delayed_graph_import(url: str, repo: str, graph: str, graph_file: str, username: str, password:str):
    time.sleep(5)
    return pximport.graphdb_graph(url, repo, graph, graph_file, username, password)

def filter_error_response(resp: dict, export_responses: list[dict]):
    found_errors = list(filter(lambda graph: True if graph['graph'] == resp['graph'] else False, export_responses))
    if len(found_errors) > 0:
        return found_errors[0]
    return None

@flow(log_prints=True)
def export_import_repos_graphs(src_url: str, prefix: str, src_repo: str, graphs: list[str], tgt_url: str, tgt_repo: str, src_user: str = '', src_passwd: str = '', tgt_user: str = '', tgt_passwd: str = ''):
    export_responses = list(map(lambda g: delayed_graph_export(src_url, prefix, src_repo, g, src_user, src_passwd), graphs))
    list(map(lambda g: print("exported graph", g), export_responses))
    import_responses = list(map(lambda g: {'graph': g['graph'], 'response': delayed_graph_import(tgt_url, tgt_repo, g['graph'], g['file'], tgt_user, tgt_passwd)}, export_responses))
    # error_responses = list(filter(lambda resp: True if resp['response'].status_code == 400 else False, import_responses))
    # if len(error_responses) > 0:
    #     retry_err_responses = list(filter(lambda resp: filter_error_response(resp, export_responses), ))
    list(map(lambda r: print(r['graph'], r['response'].content, r['response'].headers), import_responses))
    return import_responses

# env-ontotext-graphdb-ke-test @ mercur
@flow(log_prints=True)
def backup_restore_c5_ke_test(repos: list[str]):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'ke-test')
    config = pxsec.bytes_to_dict(secrets_file)
    backup_restore(config['BKP_SRV'], config['BKP_PREFIX'], repos, config['BKP_USER'], config['BKP_PASS'], config['RST_SRV'], config['RST_USER'], config['RST_PASS'])

@flow(log_prints=True)
def export_import_repos_c5_ke_test(repos: list[str]):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'ke-test')
    config = pxsec.bytes_to_dict(secrets_file)
    export_import_repos(config['BKP_SRV'], config['BKP_PREFIX'], repos, config['RST_CONTAINER'], config['RST_VOLUME'], config['RST_NETWORK'], config['BKP_USER'], config['BKP_PASS'])

@flow(log_prints=True)
def export_import_repos_ke1_ke_test(repos: list[str]):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'ke1-ke-test')
    config = pxsec.bytes_to_dict(secrets_file)
    export_import_repos(config['BKP_SRV'], config['BKP_PREFIX'], repos, config['RST_CONTAINER'], config['RST_VOLUME'], config['RST_NETWORK'], config['BKP_USER'], config['BKP_PASS'])

@flow(log_prints=True)
def export_import_repos_ke2_ke_test(repos: list[str]):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'ke2-ke-test')
    config = pxsec.bytes_to_dict(secrets_file)
    export_import_repos(config['BKP_SRV'], config['BKP_PREFIX'], repos, config['RST_CONTAINER'], config['RST_VOLUME'], config['RST_NETWORK'], config['BKP_USER'], config['BKP_PASS'])

@flow(log_prints=True)
def export_import_repos_graphs_c5_ke_test(src_repo: str, graphs: list[str], tgt_repo: str):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'ke-test')
    config = pxsec.bytes_to_dict(secrets_file)
    export_import_repos_graphs(config['BKP_SRV'], config['BKP_PREFIX'], src_repo, graphs, config['RST_SRV'], tgt_repo, config['BKP_USER'], config['BKP_PASS'], config['RST_USER'], config['RST_PASS'])

@flow(log_prints=True)
def export_import_repos_graphs_ke1_ke_test(src_repo: str, graphs: list[str], tgt_repo: str):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'ke1-ke-test')
    config = pxsec.bytes_to_dict(secrets_file)
    export_import_repos_graphs(config['BKP_SRV'], config['BKP_PREFIX'], src_repo, graphs, config['RST_SRV'], tgt_repo, config['BKP_USER'], config['BKP_PASS'], config['RST_USER'], config['RST_PASS'])

@flow(log_prints=True)
def export_import_repos_graphs_ke2_ke_test(src_repo: str, graphs: list[str], tgt_repo: str):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'ke2-ke-test')
    config = pxsec.bytes_to_dict(secrets_file)
    export_import_repos_graphs(config['BKP_SRV'], config['BKP_PREFIX'], src_repo, graphs, config['RST_SRV'], tgt_repo, config['BKP_USER'], config['BKP_PASS'], config['RST_USER'], config['RST_PASS'])

# env-ontotext-graphdb-ke-ingest @ mercur
@flow(log_prints=True)
def backup_restore_c5_ke_ingest(repos: list[str]):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'ke-ingest')
    config = pxsec.bytes_to_dict(secrets_file)
    backup_restore(config['BKP_SRV'], config['BKP_PREFIX'], repos, config['BKP_USER'], config['BKP_PASS'], config['RST_SRV'], config['RST_USER'], config['RST_PASS'])

@flow(log_prints=True)
def export_import_repos_c5_ke_ingest(repos: list[str]):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'ke-ingest')
    config = pxsec.bytes_to_dict(secrets_file)
    export_import_repos(config['BKP_SRV'], config['BKP_PREFIX'], repos, config['RST_CONTAINER'], config['RST_VOLUME'], config['RST_NETWORK'], config['BKP_USER'], config['BKP_PASS'])

@flow(log_prints=True)
def export_import_repos_graphs_c5_ke_ingest(src_repo: str, graphs: list[str], tgt_repo: str):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'ke-ingest')
    config = pxsec.bytes_to_dict(secrets_file)
    export_import_repos_graphs(config['BKP_SRV'], config['BKP_PREFIX'], src_repo, graphs, config['RST_SRV'], tgt_repo, config['BKP_USER'], config['BKP_PASS'], config['RST_USER'], config['RST_PASS'])

# env-ontotext-graphdb-demo @ mercur
@flow(log_prints=True)
def backup_restore_c5_demo(repos: list[str]):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'demo')
    config = pxsec.bytes_to_dict(secrets_file)
    backup_restore(config['BKP_SRV'], config['BKP_PREFIX'], repos, config['BKP_USER'], config['BKP_PASS'], config['RST_SRV'], config['RST_USER'], config['RST_PASS'])

@flow(log_prints=True)
def export_import_repos_c5_demo(repos: list[str]):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'demo')
    config = pxsec.bytes_to_dict(secrets_file)
    export_import_repos(config['BKP_SRV'], config['BKP_PREFIX'], repos, config['RST_CONTAINER'], config['RST_VOLUME'], config['RST_NETWORK'], config['BKP_USER'], config['BKP_PASS'])

@flow(log_prints=True)
def export_import_repos_graphs_c5_demo(src_repo: str, graphs: list[str], tgt_repo: str):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'demo')
    config = pxsec.bytes_to_dict(secrets_file)
    export_import_repos_graphs(config['BKP_SRV'], config['BKP_PREFIX'], src_repo, graphs, config['RST_SRV'], tgt_repo, config['BKP_USER'], config['BKP_PASS'], config['RST_USER'], config['RST_PASS'])

# env-ontotext-graphdb-dev @ build.px
@flow(log_prints=True)
def backup_restore_c5_dev(repos: list[str]):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'dev')
    config = pxsec.bytes_to_dict(secrets_file)
    backup_restore(config['BKP_SRV'], config['BKP_PREFIX'], repos, config['BKP_USER'], config['BKP_PASS'], config['RST_SRV'], config['RST_USER'], config['RST_PASS'])

@flow(log_prints=True)
def export_import_repos_c5_dev(repos: list[str]):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'dev')
    config = pxsec.bytes_to_dict(secrets_file)
    export_import_repos(config['BKP_SRV'], config['BKP_PREFIX'], repos, config['RST_CONTAINER'], config['RST_VOLUME'], config['RST_NETWORK'], config['BKP_USER'], config['BKP_PASS'])

@flow(log_prints=True)
def export_import_repos_graphs_c5_dev(src_repo: str, graphs: list[str], tgt_repo: str):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'dev')
    config = pxsec.bytes_to_dict(secrets_file)
    export_import_repos_graphs(config['BKP_SRV'], config['BKP_PREFIX'], src_repo, graphs, config['RST_SRV'], tgt_repo, config['BKP_USER'], config['BKP_PASS'], config['RST_USER'], config['RST_PASS'])

# env-ontotext-graphdb-ke1 @ build.px
@flow(log_prints=True)
def backup_restore_c5_ke1(repos: list[str]):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'ke1')
    config = pxsec.bytes_to_dict(secrets_file)
    backup_restore(config['BKP_SRV'], config['BKP_PREFIX'], repos, config['BKP_USER'], config['BKP_PASS'], config['RST_SRV'], config['RST_USER'], config['RST_PASS'])

@flow(log_prints=True)
def export_import_repos_c5_ke1(repos: list[str]):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'ke1', 'stringify')
    config = pxsec.bytes_to_dict(secrets_file)
    export_import_repos(config['BKP_SRV'], config['BKP_PREFIX'], repos, config['RST_CONTAINER'], config['RST_VOLUME'], config['RST_NETWORK'], config['BKP_USER'], config['BKP_PASS'])

@flow(log_prints=True)
def export_import_repos_graphs_c5_ke1(src_repo: str, graphs: list[str], tgt_repo: str):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'ke1')
    config = pxsec.bytes_to_dict(secrets_file)
    export_import_repos_graphs(config['BKP_SRV'], config['BKP_PREFIX'], src_repo, graphs, config['RST_SRV'], tgt_repo, config['BKP_USER'], config['BKP_PASS'], config['RST_USER'], config['RST_PASS'])

# env-ontotext-graphdb-ke2 @ build.px
@flow(log_prints=True)
def backup_restore_c5_ke2(repos: list[str]):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'ke2')
    config = pxsec.bytes_to_dict(secrets_file)
    backup_restore(config['BKP_SRV'], config['BKP_PREFIX'], repos, config['BKP_USER'], config['BKP_PASS'], config['RST_SRV'], config['RST_USER'], config['RST_PASS'])

@flow(log_prints=True)
def export_import_repos_c5_ke2(repos: list[str]):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'ke2')
    config = pxsec.bytes_to_dict(secrets_file)
    export_import_repos(config['BKP_SRV'], config['BKP_PREFIX'], repos, config['RST_CONTAINER'], config['RST_VOLUME'], config['RST_NETWORK'], config['BKP_USER'], config['BKP_PASS'])

@flow(log_prints=True)
def export_import_repos_graphs_c5_ke2(src_repo: str, graphs: list[str], tgt_repo: str):
    pxsec = PXInfisical()
    secrets_file = pxsec.get('eedaff89-6dbb-48c3-826e-0dd20fb1a02b', 'ke2')
    config = pxsec.bytes_to_dict(secrets_file)
    export_import_repos_graphs(config['BKP_SRV'], config['BKP_PREFIX'], src_repo, graphs, config['RST_SRV'], tgt_repo, config['BKP_USER'], config['BKP_PASS'], config['RST_USER'], config['RST_PASS'])

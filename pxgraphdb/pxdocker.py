import docker
from python_on_whales import docker as pdocker
from python_on_whales import Image as PImage
import docker.models
import docker.models.configs
from docker.types import LogConfig
import os
import time
import pycurl
import certifi
from urllib.parse import urlparse

from pxgraphdb.pxsingleton import PXSingleton

# https://docker-py.readthedocs.io/en/stable/containers.html#container-objects
# https://docker-py.readthedocs.io/en/stable/client.html
# container states:
# created
# running
# paused
# restarting
# exited
# dead


class PXDocker(metaclass=PXSingleton):
    def __init__(self, client_socket: str = 'unix:///var/run/docker.sock', api_client_socket: str = 'unix://var/run/docker.sock'):
        self.client = docker.DockerClient(base_url=client_socket)
        self.apiClient = docker.APIClient(base_url=api_client_socket)
    def has_name(self, container: docker.models.containers.Container, cname: str):
        if container.name == cname:
            return True
        return False
    def cnt_info(self, container: docker.models.containers.Container):
        return {
            'id':container.short_id, 
            'name':container.name
        }
    
    def info(self, container: docker.models.containers.Container):
        return self.cnt_info(container)
    def cnt_restart(self, container_name: str):
        cnt = list(filter(lambda c: self.has_name(c, container_name), self.client.containers.list(all=True)))
        if len(cnt) > 0:
            cnt[0].restart()
            return {'status','OK'}
        return {'status','container with name:'+container_name+' not found!'}
    
    def container_restart(self, container_name: str):
        return self.cnt_restart(container_name)
    def cnt_start(self, container_name: str):
        cnt = list(filter(lambda c: self.has_name(c, container_name), self.client.containers.list(all=True)))
        if len(cnt) > 0:
            cnt[0].start()
            return {'status','OK'}
        return {'status','container with name:'+container_name+' not found!'}
    
    def container_start(self, container_name: str):
        return self.cnt_start(container_name)
    def cnts_list(self):
        return list(map(lambda c: self.cnt_info(c), self.client.containers.list(all=True)))
    
    def containers_list(self):
        return self.cnts_list()
    def cnt_by_name(self, name: str):
        cnt = list(filter(lambda c: self.has_name(c, name), self.client.containers.list(all=True)))
        if len(cnt) > 0:
            return {'name': cnt[0].name, 'container':cnt[0]}
        return None
    
    def container_by_name(self, name: str):
        return self.cnt_by_name(name)
    
    def container_by_name_from_list(self, name: str, containers: list[docker.models.containers.Container]):
        cnt = list(filter(lambda c: self.has_name(c, name), containers))
        if len(cnt) > 0:
            return {'name': cnt[0].name, 'container':cnt[0]}
        return None
    def cnt_stop(self, name: str):
        container = self.cnt_by_name(name)
        if container != None:
            stopped = container['container'].stop()
            # todo: check stopped
            return True
        return False
    
    def container_stop(self, name: str):
        return self.cnt_stop(name)
    def cnt_remove(self, name: str):
        removed = None
        container = self.cnt_by_name(name)
        if container != None:
            stopped = self.cnt_stop(name)
            # todo: check stopped
            removed = container['container'].remove()
            # todo: check removed
        return removed
    
    def container_remove(self, name: str):
        return self.cnt_remove(name)
    
    def volumes_list(self):
        return list(map(lambda v: {'name': v.name, 'volume':v}, self.client.volumes.list()))
    def vol_exists(self, name:str):
        vol = list(filter(lambda v: True if v['name'] == name else False, self.volumes_list()))
        if len(vol) > 0:
            return vol[0]
        return None
    
    def volume_exists(self, name:str):
        return self.vol_exists(name)
    def vol_create(self, name: str, driver: str):
        vol = self.volume_exists(name)
        if vol is None:
            return {'name': name, 'volume': self.client.api.create_volume(name=name, driver=driver)}
        return vol
    
    def volume_create(self, name: str, driver: str):
        return self.vol_create(name, driver)
    def vol_remove(self, name: str,persist_result=False):
        vol = self.volume_exists(name)
        if vol is None:
            return True
        self.client.api.remove_volume(name, True)
        return True
    def vol_remove_wait(self, name: str,persist_result=False):
        vol = self.volume_exists(name)
        if vol is None:
            return True
        for i in range(10):
            try:
                self.client.api.remove_volume(name, True)
                return True
            except Exception as err:
                print(err)
        return False
    
    def volume_remove(self, name: str,persist_result=False):
        return self.vol_remove(name)
    
    def volume_remove_wait(self, name: str,persist_result=False):
        return self.vol_remove_wait(name)
    def is_volume(self, mount: dict):
        if 'Type' not in mount:
            return False
        if mount['Type'] == 'volume':
            return True
        return False
    def cnt_image(self, name: str):
        cnt_info = self.apiClient.inspect_container(name)
        return cnt_info['Config']['Image']
    
    def container_image(self, name: str):
        return self.cnt_image(name)
    def cnt_ports(self, name: str):
        cnt_info = self.apiClient.inspect_container(name)
        return cnt_info['Config']['ExposedPorts']
    
    def container_ports(self, name: str):
        return self.cnt_ports(name)
    def container_cmd(self, name: str):
        cnt_info = self.apiClient.inspect_container(name)
        return cnt_info['Config']['Cmd']
    def container_environment(self, name: str):
        cnt_info = self.apiClient.inspect_container(name)
        return cnt_info['Config']['Env']
    
    def container_volumes(self, name: str):
        cnt_info = self.apiClient.inspect_container(name)
        return list(filter(lambda m: self.is_volume(m), cnt_info['Mounts']))
    
    def networks_list(self):
        return list(map(lambda n: {'name':n.name, 'network':n}, self.client.networks.list()))
    
    def network_exists(self, name: str):
        nw = list(filter(lambda n: True if n['name'] == name else False, self.networks_list()))
        if len(nw) > 0:
            return nw[0]
        return None
    
    def network_create(self, name: str, driver: str):
        nw = self.network_exists(name)
        if nw is None:
            return {'name':name, 'network': self.client.networks.create(name=name, driver=driver)}
        return nw
    
    def network_by_name(self, name: str):
        nw = list(filter(lambda n: True if n['name'] == name else False, self.networks_list()))
        if len(nw) > 0:
            return nw[0]
        return None
    
    def container_connect_to_network(self, name: str, container_name: str):
        nw = self.network_create(name, "bridge")
        return nw['network'].connect(container_name)
    
    def container_disconnect_from_network(self, name: str, container_name: str):
        nw = self.network_by_name(name)
        if nw is not None:
            return nw['network'].disconnect(container_name)
        return None
    def cnt_networks(self, name: str):
        cnt_info = self.apiClient.inspect_container(name)
        return list(map(lambda n: self.network_by_name(n), cnt_info['NetworkSettings']['Networks']))
    
    def container_networks(self, name: str):
        return self.cnt_networks(name)
    def cnt_ips(self, name: str):
        cnt_info = self.apiClient.inspect_container(name)
        nwks = cnt_info['NetworkSettings']['Networks'].keys()
        return list(map(lambda n: cnt_info['NetworkSettings']['Networks'][n]['IPAddress'], nwks))
    
    def container_ips(self, name: str):
        return self.cnt_ips(name)
    
    def mount_create(self, src: str, tgt: str):
        return docker.types.Mount(tgt, src)
    def cnt_exec(self, name: str, command: list[str], env: dict = {}):
        cnt = self.container_by_name(name)
        if cnt is not None:
            return cnt['container'].exec_run(cmd=command, environment=env, stdout=True, stderr=True)
        return None
    
    def container_exec(self, name: str, command: list[str], env: dict = {}):
        print(command)
        return self.cnt_exec(name, command, env)
    def cnt_run(
        self, 
        image:str, 
        name: str, 
        network: str = 'bridge', 
        mounts: list[docker.types.Mount] = [], 
        env: dict = {}, 
        command: list[str] = [],
        entrypoint: str = None, 
        run_once: bool = False, 
        auto_remove: bool = False):

        print(command)
        restart_policy = {'Name':'unless-stopped'}
        if run_once:
            restart_policy = {"Name": "on-failure", "MaximumRetryCount": 1}
        if auto_remove:
            restart_policy = {}
        if entrypoint is not None:
            return self.client.containers.run(
                remove=False,
                detach=True,
                restart_policy=restart_policy,
                name=name,
                network=network,
                mounts=mounts,
                image=image,
                environment=env,
                entrypoint=entrypoint,
                command=command,
                auto_remove=auto_remove)
        return self.client.containers.run(
                remove=False,
                detach=True,
                restart_policy=restart_policy,
                name=name,
                network=network,
                mounts=mounts,
                image=image,
                environment=env,
                command=command,
                auto_remove=auto_remove)
    
    def container_run(self, image:str, name: str, network: str, mounts: list[docker.types.Mount], env: dict = {}, command: list[str] = [], entrypoint: str = None, run_once: bool = False, auto_remove: bool = False):
        return self.cnt_run(image, name=name, network=network, mounts=mounts, env=env, command=command, entrypoint=entrypoint, run_once=run_once, auto_remove=auto_remove)
    def cnt_state(self, name: str):
        cnt_info = self.apiClient.inspect_container(name)
        return cnt_info['State']['Status']
    
    def container_state(self, name: str):
        return self.cnt_state(name)
    def cnt_run_wait(self, image:str, name: str, network: str = 'bridge', mounts: list[docker.types.Mount] = [], env: dict = {}, command: list[str] = [], entrypoint: str = None, run_once: bool = False, auto_remove: bool = False):
        cnt = self.cnt_run(image, name=name, network=network, mounts=mounts, env=env, command=command, entrypoint=entrypoint, run_once=run_once, auto_remove=auto_remove)
        return cnt.wait()
    
    def container_run_wait(self, image:str, name: str, network: str, mounts: list[docker.types.Mount], env: dict = {}, command: list[str] = [], entrypoint: str = None, run_once: bool = False, auto_remove: bool = False):
        return self.cnt_run_wait(image, name=name, network=network, mounts=mounts, env=env, command=command, entrypoint=entrypoint, run_once=run_once, auto_remove=auto_remove)
    def cnt_run_wait_bash(
        self, 
        image:str, 
        name: str, 
        network: str = 'bridge', 
        mounts: list[docker.types.Mount] = [], 
        env: dict = {}, 
        command: list[str] = [], 
        check: dict = {'cmd': 'whoami','loop':10, 'wait_interval':1},
        entrypoint: str = None,
        run_once: bool = False, 
        auto_remove: bool = False):

        cnt = self.cnt_run(image, name=name, network=network, mounts=mounts, env=env, command=command, entrypoint=entrypoint, run_once=run_once, auto_remove=auto_remove)
        for i in range(check['loop']):
            time.sleep(check['wait_interval'])
            try:
                resp = cnt.exec_run(check['cmd'], environment=env, demux=True)
                if resp.exit_code == 0:
                    return cnt
            except Exception as err:
                print(err)
                pass
        # cleanup container before sending None back
        self.cnt_remove(name)
        return None
    
    def container_run_wait_bash(
        self, 
        image:str, 
        name: str, 
        network: str = 'bridge', 
        mounts: list[docker.types.Mount] = [], 
        env: dict = {}, 
        command: list[str] = [], 
        entrypoint: str = None,
        check: dict = {'cmd': 'whoami','loop':10, 'wait_interval':1},
        run_once: bool = False, 
        auto_remove: bool = False):

        return self.cnt_run_wait_bash(image, name=name, network=network, mounts=mounts, env=env, command=command, check=check, entrypoint=entrypoint, run_once=run_once, auto_remove=auto_remove)
    
    def container_config(self, name:str, driver: str, opts: dict):
        log_type = LogConfig.types.JSON
        if driver == 'fluentd':
            log_type = LogConfig.types.FLUENTD
        lc = LogConfig(type=log_type, config=opts)
        # nc = self.apiClient.create_networking_config({'fluent': self.apiClient.create_endpoint_config(ipv4_address='172.20.0.5',links={name:name})})
        return self.apiClient.create_host_config(log_config=lc)
    
    def container_config_ports(self, name:str, driver: str, opts: dict, ports: dict):
        log_type = LogConfig.types.JSON
        if driver == 'fluentd':
            log_type = LogConfig.types.FLUENTD
        lc = LogConfig(type=log_type, config=opts)
        # nc = self.apiClient.create_networking_config({'fluent': self.apiClient.create_endpoint_config(ipv4_address='172.20.0.5',links={name:name})})
        return self.apiClient.create_host_config(log_config=lc, port_bindings=ports)
    
    def container_create_logging(self, image:str, name: str, network: str, mounts: list[docker.types.Mount], host_config: docker.models.configs.Config, env: dict = {}, command: list[str] = [], run_once: bool = False, auto_remove: bool = False):
        restart_policy = {'Name':'unless-stopped'}
        if run_once:
            restart_policy = {"Name": "on-failure", "MaximumRetryCount": 1}
        if auto_remove:
            restart_policy = {}
        self.apiClient.create_container(
            host_config=host_config,
            detach=True,
            name=name,
            image=image,
            environment=env,
            command=command)
        self.container_connect_to_network('fluent', name)
        return name
    
    def container_create_logging_ports(self, image:str, name: str, network: str, mounts: list[docker.types.Mount], ports: list[int], host_config: docker.models.configs.Config, env: dict = {}, command: list[str] = [], run_once: bool = False, auto_remove: bool = False):
        restart_policy = {'Name':'unless-stopped'}
        if run_once:
            restart_policy = {"Name": "on-failure", "MaximumRetryCount": 1}
        if auto_remove:
            restart_policy = {}
        self.apiClient.create_container(
            host_config=host_config,
            ports=ports,
            detach=True,
            name=name,
            image=image,
            environment=env,
            command=command)
        self.container_connect_to_network('fluent', name)
        return name
    def cnt_run_ports(self, image:str, name: str, network: str = 'bridge', mounts: list[docker.types.Mount] = [], ports: dict = {8080:80}, env: dict = {}, command: list[str] = [], run_once: bool = False, auto_remove: bool = False):
        print(command)
        swapped_ports = {v: k for k, v in ports.items()}
        restart_policy = {'Name':'unless-stopped'}
        if run_once:
            restart_policy = {"Name": "on-failure", "MaximumRetryCount": 1}
        if auto_remove:
            restart_policy = {}
        return self.client.containers.run(
            ports=swapped_ports,
            remove=False,
            detach=True,
            restart_policy=restart_policy,
            name=name,
            network=network,
            mounts=mounts,
            image=image,
            environment=env,
            command=command,
            auto_remove=auto_remove)
    def cnt_run_wait_for_rest_service(
        self, 
        image:str, 
        name: str, 
        network: str = 'bridge', 
        mounts: list[docker.types.Mount] = [], 
        env: dict = {}, 
        command: list[str] = [], 
        check: dict = {'status_code': 200,'loop':10,'wait_interval':1, 'scheme':'http', 'host': 'localhost:8000', 'path':'/health'},
        run_once: bool = False, 
        auto_remove: bool = False):

        resp_cnt = self.cnt_run(image, name=name, network=network, mounts=mounts, env=env, command=command, run_once=run_once, auto_remove=auto_remove)
        for i in range(check['loop']):
            time.sleep(check['wait_interval'])
            ips = self.cnt_ips(check['host'])
            src_url = check['scheme']+'://'+ips[0]+check['path']
            c = pycurl.Curl()
            c.setopt(c.VERBOSE, 0)
            c.setopt(c.URL, src_url)
            p_uri = urlparse(src_url)
            if p_uri.scheme == 'https':
                c.setopt(c.CAINFO, certifi.where())
            c.perform()
            if c.getinfo(pycurl.HTTP_CODE) == check['status_code']:
                return True
        # cleanup container before sending False back
        self.cnt_remove(name)
        return False
    
    def container_run_wait_for_rest_service(
        self, 
        image:str, 
        name: str, 
        network: str = 'bridge', 
        mounts: list[docker.types.Mount] = [], 
        env: dict = {}, 
        command: list[str] = [], 
        check: dict = {'status_code': 200,'loop':10,'wait_interval':1, 'scheme':'http', 'host': 'localhost:8000', 'path':'/health'},
        run_once: bool = False, 
        auto_remove: bool = False):

        return self.cnt_run_wait_for_rest_service(image, name=name, network=network, mounts=mounts, env=env, command=command, check=check, run_once=run_once, auto_remove=auto_remove)
    def cnt_run_ports_wait_for_rest_service(
        self, 
        image:str, 
        name: str, 
        network: str = 'bridge', 
        mounts: list[docker.types.Mount] = [], 
        ports: dict = {8080:80}, 
        env: dict = {}, 
        command: list[str] = [], 
        check: dict = {'path':'/','status_code': 200,'loop':10,'wait_interval':1, 'scheme':'http'},
        run_once: bool = False, 
        auto_remove: bool = False):

        resp_cnt = self.cnt_run_ports(image, name, network, mounts, ports, env, command, run_once, auto_remove)
        for i in range(check['loop']):
            time.sleep(check['wait_interval'])
            src_url = check['scheme']+'://localhost:'+str(next(iter(ports)))+check['path']
            c = pycurl.Curl()
            c.setopt(c.VERBOSE, 0)
            c.setopt(c.URL, src_url)
            p_uri = urlparse(src_url)
            if p_uri.scheme == 'https':
                c.setopt(c.CAINFO, certifi.where())
            c.perform()
            if c.getinfo(pycurl.HTTP_CODE) == check['status_code']:
                return True
        # cleanup container before sending False back
        self.cnt_remove(name)
        return False
    
    def container_run_ports_wait_for_rest_service(
        self, 
        image:str, 
        name: str, 
        network: str = 'bridge', 
        mounts: list[docker.types.Mount] = [], 
        ports: dict = {8080:80}, 
        env: dict = {}, 
        command: list[str] = [], 
        check: dict = {'path':'/','status_code': 200,'loop':10,'wait_interval':1, 'scheme':'http'},
        run_once: bool = False, 
        auto_remove: bool = False):

        return cnt_run_ports_wait_for_rest_service(image, name, network, mounts, ports, env, command, check, run_once, auto_remove)
    
    def container_run_ports(self, image:str, name: str, network: str, mounts: list[docker.types.Mount], ports: dict, env: dict = {}, command: list[str] = [], run_once: bool = False, auto_remove: bool = False):
        return self.cnt_run_ports(image, name, network, mounts, ports, env, command, run_once, auto_remove)
    def cnt_run_root(self, image:str, name: str, network: str, mounts: list[docker.types.Mount], env: dict = {}, command: list[str] = [], run_once: bool = False, auto_remove: bool = False):
        print(command)
        restart_policy = {'Name':'unless-stopped'}
        if run_once:
            restart_policy = {"Name": "on-failure", "MaximumRetryCount": 1}
        if auto_remove:
            restart_policy = {}
        return self.client.containers.run(
            remove=False,
            detach=True,
            restart_policy=restart_policy,
            user='root',
            name=name,
            network=network,
            mounts=mounts,
            image=image,
            environment=env,
            command=command,
            auto_remove=auto_remove)
    
    def container_run_root(self, image:str, name: str, network: str, mounts: list[docker.types.Mount], env: dict = {}, command: list[str] = [], run_once: bool = False, auto_remove: bool = False):
        return self.cnt_run_root(image, name=name, network=network, mounts=mounts, env=env, command=command, run_once=run_once, auto_remove=auto_remove)
    
    def container_run_debug(self, image:str, name: str, network: str, mounts: list[docker.types.Mount], env: dict = {}, command: list[str] = [], run_once: bool = False, auto_remove: bool = False):
        print(command)
        restart_policy = {'Name':'unless-stopped'}
        if run_once:
            restart_policy = {"Name": "on-failure", "MaximumRetryCount": 1}
        if auto_remove:
            restart_policy = {}
        return self.client.containers.run(
            remove=False,
            detach=False,
            restart_policy=restart_policy,
            name=name,
            network=network,
            mounts=mounts,
            image=image,
            environment=env,
            command=command,
            auto_remove=auto_remove)
    def cnt_run_shell(self, image:str, name: str, network: str, mounts: list[docker.types.Mount], env: dict = {}, command: list[str] = [], run_once: bool = False, auto_remove: bool = False):
        print(command)
        restart_policy = {'Name':'unless-stopped'}
        if run_once:
            restart_policy = {"Name": "on-failure", "MaximumRetryCount": 1}
        if auto_remove:
            restart_policy = {}
        return self.client.containers.run(
            remove=False,
            detach=True,
            restart_policy=restart_policy,
            name=name,
            network=network,
            mounts=mounts,
            image=image,
            environment=env,
            command=command,
            auto_remove=auto_remove,
            entrypoint='sh')
    
    def container_run_shell(self, image:str, name: str, network: str, mounts: list[docker.types.Mount], env: dict = {}, command: list[str] = [], run_once: bool = False, auto_remove: bool = False):
        return self.cnt_run_shell(image, name, network, mounts, env, command, run, auto_remove)
    
    def container_run_root_shell(self, image:str, name: str, network: str, mounts: list[docker.types.Mount], env: dict = {}, command: list[str] = [], run_once: bool = False, auto_remove: bool = False):
        print(command)
        restart_policy = {'Name':'unless-stopped'}
        if run_once:
            restart_policy = {"Name": "on-failure", "MaximumRetryCount": 1}
        if auto_remove:
            restart_policy = {}
        return self.client.containers.run(
            remove=False,
            detach=True,
            restart_policy=restart_policy,
            user='root',
            name=name,
            network=network,
            mounts=mounts,
            image=image,
            environment=env,
            command=command,
            auto_remove=auto_remove,
            entrypoint='sh')
    
    def container_run_shell_debug(self, image:str, name: str, network: str, mounts: list[docker.types.Mount], env: dict = {}, command: list[str] = [], run_once: bool = False, auto_remove: bool = False):
        print(command)
        restart_policy = {'Name':'unless-stopped'}
        if run_once:
            restart_policy = {"Name": "on-failure", "MaximumRetryCount": 1}
        if auto_remove:
            restart_policy = {}
        return self.client.containers.run(
            remove=False,
            detach=False,
            restart_policy=restart_policy,
            name=name,
            network=network,
            mounts=mounts,
            image=image,
            environment=env,
            command=command,
            auto_remove=auto_remove,
            entrypoint='sh')
    
    def container_run_custom_shell_debug(self, image:str, name: str, network: str, mounts: list[docker.types.Mount], env: dict = {}, command: list[str] = [], run_once: bool = False, auto_remove: bool = False, custom_entry: str = 'sh'):
        print(command)
        print(mounts)
        print(network)
        restart_policy = {'Name':'unless-stopped'}
        if run_once:
            restart_policy = {"Name": "on-failure", "MaximumRetryCount": 1}
        if auto_remove:
            restart_policy = {}
        return self.client.containers.run(
            remove=False,
            detach=False,
            restart_policy=restart_policy,
            name=name,
            network=network,
            mounts=mounts,
            image=image,
            environment=env,
            command=command,
            auto_remove=auto_remove,
            entrypoint=custom_entry)
    
    def container_run_root_shell_debug(self, image:str, name: str, network: str, mounts: list[docker.types.Mount], env: dict = {}, command: list[str] = [], run_once: bool = False, auto_remove: bool = False):
        print(command)
        restart_policy = {'Name':'unless-stopped'}
        if run_once:
            restart_policy = {"Name": "on-failure", "MaximumRetryCount": 1}
        if auto_remove:
            restart_policy = {}
        return self.client.containers.run(
            user='root',
            remove=False,
            detach=False,
            restart_policy=restart_policy,
            name=name,
            network=network,
            mounts=mounts,
            image=image,
            environment=env,
            command=command,
            auto_remove=auto_remove,
            entrypoint='sh')
    def img_build(self, src: str, tag: str, docker_file_path: str = 'Dockerfile', platform: str = 'linux/amd64', secrets: list[str] = None):
        print("build image for platform ", platform, " from source:", src, "with Dockerfile:", docker_file_path, "into tag:", tag)
        # shutil.copy(docker_file_path,'tmp_repo/Dockerfile')
        return pdocker.buildx.build(context_path=src, tags=[tag], cache=False, file=docker_file_path, load=True, pull=True, platforms=[platform], secrets=secrets)
    
    def image_build(self, src: str, tag: str, docker_file_path: str = 'Dockerfile', platform: str = 'linux/amd64', secrets: list[str] = None):
        return self.img_build(src, tag, docker_file_path, platform, secrets=secrets)
    
    def image_build_with_args(self, src: str, tag: str, docker_file_path: str = 'Dockerfile', args: dict = {}, platform: str = 'linux/amd64', secrets: list[str] = None):
        print("build image for platform ", platform, " from source:", src, "with Dockerfile:", docker_file_path, "into tag:", tag)
        return pdocker.buildx.build(context_path=src, tags=[tag], build_args=args, cache=False, file=docker_file_path, load=True, pull=True, platforms=[platform], secrets=secrets)
    
    def tag_from_url(self, url: str):
        sp_url = url.split(':')
        return sp_url[(len(sp_url)-1)]
    def img_pull(self, registry: str, tag: str):
        print("pull", registry, "with tag", tag)
        return pdocker.image.pull(registry+':'+tag)
    
    def image_pull(self, registry: str, tag: str):
        return self.img_pull(registry, tag)
    
    def pimage(self, name: str):
        return pdocker.image.inspect(name)
    
    def image_push(self, img: PImage, registry: str):
        print("push", img, "to", registry)
        pdocker.image.tag(img, registry)
        pdocker.image.push(registry)
        return self.client.images.push(registry)
    
    def image_save(self, image: str, file_path: str):
        img = self.apiClient.get_image(image)
        f = open(file_path, 'wb')
        for chunk in img:
            f.write(chunk)
        f.close()
        return file_path
    
    def image_load(self, file_path: str):
        with open(file_path, 'rb') as f:
            self.apiClient.load_image(f)
        f.close()
    
    def login(self, registry: str, user: str, password: str):
        return pdocker.login(
            username=user,
            password=password,
            server=registry
        )

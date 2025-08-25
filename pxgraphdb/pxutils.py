import shutil
from os import mkdir , path, remove, environ
import base64
from uuid import uuid4
from git import Repo
from urllib.parse import urlparse
from os import environ

from pxgraphdb.pxdocker import PXDocker

PX_UTILS_IMAGE='bitnami/minideb'
PX_UTILS_IMAGE_VERSION='latest'
PX_UTILS_NETWORK='bridge'
PX_UTILS_CONTAINER='tmp-pxutils-container'
PX_UTILS_REPO='tmp_repo'

def check_required_env_vars(vars: list[str]):
    not_found = list(filter(lambda v: True if environ.get(v) != '' else False, vars))
    if len(not_found) == 0:
        return True
    list(map(lambda v: print("ENV VAR NOT FOUND",v), not_found))
    return False

def git_clone(src: str, branch: str):
    repo_info = urlparse(src)
    tld = repo_info.netloc.split('@')
    if len(tld) > 1:
        print('from', tld[1],'clone repository',repo_info.path, 'branch', branch)
    else:
        print('from', repo_info.netloc,'clone repository',repo_info.path, 'branch', branch)
    if path.isdir(PX_UTILS_REPO):
        shutil.rmtree(PX_UTILS_REPO)
    return Repo.clone_from(src, PX_UTILS_REPO, branch=branch)

def git_clone_pkg(src: str, branch: str):
    repo_info = urlparse(src)
    tld = repo_info.netloc.split('@')
    if len(tld) > 1:
        print('from', tld[1],'clone repository',repo_info.path, 'branch', branch)
    else:
        print('from', repo_info.netloc,'clone repository',repo_info.path, 'branch', branch)
    if path.isdir(PX_UTILS_REPO):
        shutil.rmtree(PX_UTILS_REPO)
    Repo.clone_from(src, PX_UTILS_REPO+path.sep+path.basename(src), branch=branch)
    return PX_UTILS_REPO+path.sep+path.basename(src)

def copy_file_to_volume(file_path: str, volume: str, volume_path: str = ''):
    if path.isdir('tmp_dir'):
        shutil.rmtree('tmp_dir')
    mkdir('tmp_dir')
    pxd = PXDocker(environ.get('DOCKER_HOST'), environ.get('DOCKER_API_HOST'))
    pxd.network_create(PX_UTILS_NETWORK,'bridge')
    shutil.copyfile(file_path, 'tmp_dir'+path.sep+path.basename(file_path))
    shutil.make_archive('tmp','gztar','tmp_dir')
    vol = pxd.volume_create(volume,'local')
    mount = pxd.mount_create(vol['name'], '/data')
    cnt = PX_UTILS_CONTAINER+'-'+str(uuid4())
    container = pxd.container_run_wait_bash(
        image=':'.join([PX_UTILS_IMAGE, PX_UTILS_IMAGE_VERSION]),
        name=cnt,
        network=PX_UTILS_NETWORK,
        mounts=[mount],
        command=['sleep','infinity']
    )
    if container is None:
        raise Exception('pxutils:copy_file_to_volume could not create tmp container<'+cnt+'>!')
    tar_file = None
    with open('tmp.tar.gz','rb') as ftar:
        tar_file = ftar.read()
        ftar.close()
    target_path = '/data'
    if volume_path != '':
        target_path = target_path + '/' + volume_path
    print("target_path ::",target_path)
    ret = container.put_archive(target_path, tar_file)
    pxd.container_remove(cnt)
    return ret 

def copy_tar_to_volume(tar_path: str, volume: str, volume_path: str = ''):
    pxd = PXDocker(environ.get('DOCKER_HOST'), environ.get('DOCKER_API_HOST'))
    vol = pxd.volume_create(volume,'local')
    mount = pxd.mount_create(vol['name'], '/data')
    pxd.network_create(PX_UTILS_NETWORK,'bridge')
    cnt = PX_UTILS_CONTAINER+'-'+str(uuid4())
    container = pxd.container_run(
        image=':'.join([PX_UTILS_IMAGE, PX_UTILS_IMAGE_VERSION]),
        name=cnt,
        network=PX_UTILS_NETWORK,
        mounts=[mount],
        command=['sleep','infinity']
    )
    tar_file = None
    with open(tar_path,'rb') as ftar:
        tar_file = ftar.read()
        ftar.close()
    target_path = '/data'
    if volume_path != '':
        target_path = target_path + '/' + volume_path
    print(target_path, tar_path)
    ret = container.put_archive(target_path, tar_file)
    pxd.container_remove(cnt)
    return ret

def copy_file_to_container(cname: str, file_path: str, target_file_path: str = ''):
    tar_file = None
    tmp_dir = 'tmp_'+str(uuid4())
    tmp_tar = 'tmp_'+str(uuid4())
    mkdir(tmp_dir)
    print('copy file',file_path,"to container",cname,"into",target_file_path,"to filepath",tmp_dir+path.sep+path.basename(file_path))
    shutil.copyfile(file_path, tmp_dir+path.sep+path.basename(file_path))
    shutil.make_archive(tmp_tar,'gztar',tmp_dir)
    with open(tmp_tar+'.tar.gz','rb') as ftar:
        tar_file = ftar.read()
        ftar.close()
    remove(tmp_tar+'.tar.gz')
    shutil.rmtree(tmp_dir)
    pxd = PXDocker(environ.get('DOCKER_HOST'), environ.get('DOCKER_API_HOST'))
    container = pxd.container_by_name(cname)
    print('move file', file_path, 'to container', cname, 'path', target_file_path)
    if container is None:
        raise Exception("can not copy file to not existing container <"+cname+">")
    return container['container'].put_archive(target_file_path, tar_file)

def rename_copy_file_to_container(cname: str, file_path: str, new_name: str, target_file_path: str = ''):
    tar_file = None
    tmp_dir = 'tmp_'+str(uuid4())
    tmp_tar = 'tmp_'+str(uuid4())
    mkdir(tmp_dir)
    print('copy file',file_path,"to container",cname,"into",target_file_path,"to filepath",tmp_dir+path.sep+new_name)
    shutil.copyfile(file_path, tmp_dir+path.sep+new_name)
    shutil.make_archive(tmp_tar,'gztar',tmp_dir)
    with open(tmp_tar+'.tar.gz','rb') as ftar:
        tar_file = ftar.read()
        ftar.close()
    remove(tmp_tar+'.tar.gz')
    shutil.rmtree(tmp_dir)
    pxd = PXDocker(environ.get('DOCKER_HOST'), environ.get('DOCKER_API_HOST'))
    container = pxd.container_by_name(cname)
    print('move file', file_path, 'to container', cname, 'path', target_file_path)
    if container is None:
        raise Exception("can not copy file to not existing container <"+cname+">")
    return container['container'].put_archive(target_file_path, tar_file)

def container_get_file_tar(name: str, file_path: str):
    pxd = PXDocker(environ.get('DOCKER_HOST'), environ.get('DOCKER_API_HOST'))
    container = pxd.container_by_name(name)
    # todo: check container
    f = open('./tmp.tar', 'wb')
    bits,stat = container['container'].get_archive(file_path)
    for chunk in bits:
        f.write(chunk)
    f.close()
    return './tmp.tar'

def container_get_bytes(name: str, file_path: str):
    pxd = PXDocker(environ.get('DOCKER_HOST'), environ.get('DOCKER_API_HOST'))
    container = pxd.container_by_name(name)
    # todo: check container
    f = open('./tmp.tar', 'wb')
    bits,stat = container['container'].get_archive(file_path)
    for chunk in bits:
        f.write(chunk)
    f.close()
    shutil.unpack_archive('./tmp.tar', 'tmp_dir', 'gztar')
    remove('tmp.tar')
    with open('./tmp_dir/'+path.basename(file_path), 'rb') as rf:
        return rf.read()

def container_get_file(name: str, file_path: str):
    pxd = PXDocker(environ.get('DOCKER_HOST'), environ.get('DOCKER_API_HOST'))
    container = pxd.container_by_name(name)
    # todo: check container
    f = open('./tmp.tar', 'wb')
    bits,stat = container['container'].get_archive(file_path)
    for chunk in bits:
        f.write(chunk)
    f.close()
    shutil.unpack_archive('./tmp.tar', 'tmp_dir', 'gztar')
    remove('tmp.tar')
    return './tmp_dir/'+path.basename(file_path)

def container_get_file_utf8(name: str, file_path: str):
    return container_get_bytes(name, file_path).decode('utf-8')

def container_get_file_base64(name: str, file_path: str):
    return base64.b64encode(container_get_bytes(name, file_path))

def tar_gz(src: str, tgt: str):
    shutil.make_archive(tgt, 'gztar', src)
    if path.isfile(tgt+'.tar.gz'):
        return tgt+'.tar.gz'
    return None

def volume_list_files(vol: str, file_path: str = '/'):
    pxd = PXDocker(environ.get('DOCKER_HOST'), environ.get('DOCKER_API_HOST'))
    cnt = 'tmp-pxutils-'+uuid4().hex
    mount = pxd.mount_create(vol, '/data')
    container = pxd.container_run_wait_bash(
        image=':'.join([PX_UTILS_IMAGE, PX_UTILS_IMAGE_VERSION]),
        name=cnt,
        network=PX_UTILS_NETWORK,
        mounts=[mount],
        command=['sleep','infinity']
    )
    if container is None:
        raise Exception('pxutils:copy_file_to_volume could not create tmp container<'+cnt+'>!')
    ret = pxd.container_exec(cnt, ['ls', '/data'+file_path])
    pxd.container_remove(cnt)
    return ret.output

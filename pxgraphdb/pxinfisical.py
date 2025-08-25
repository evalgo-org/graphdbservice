from dotenv import set_key
from os import environ, remove
from os.path import isfile
from dotenv import dotenv_values
import base64
import configparser
from lxml import etree
from yaml import dump
import pycurl
import certifi
from io import BytesIO
from urllib.parse import urlparse, urlencode
import json
import time

from pxgraphdb.pxdocker import PXDocker

PX_INFISICAL_NETWORK='env-px'
PX_INFISICAL_IMAGE='infisical/infisical'
PX_INFISICAL_VERSION='v0.117.1-postgres'
PX_INFISICAL_ENV = {
    'DB_CONNECTION_URI':'postgres://postgres:postgres@env-px-postgres:5432/postgres',
    'ENCRYPTION_KEY':'501f5ab5393a0eb441ae5473dc4870d6',
    'AUTH_SECRET':'FaaPDvAtHWx0DyJJQDRV55w5yzqu8s67kzx/eDQtWdY=',
    'REDIS_URL':'redis://env-px-redis:6379'
}

class PXInfisical:
    def __init__(self, site_url: str = '', client_id: str = '', client_secret: str = ''):
        self.token = None
        if site_url != '':
            self.site_url = site_url
        else:
            self.site_url = environ.get("SECRETS_SITE_URL")
        if client_id != '':
            self.client_id = client_id
        else:
            self.client_id = environ.get("SECRETS_CLIENT_ID")
        if client_secret != '':
            self.client_secret = client_secret
        else:
            self.client_secret = environ.get("SECRETS_CLIENT_SECRET")
        self.token_url = self.site_url + "/api/v1/auth/universal-auth/login"
        self.secrets_url = self.site_url + "/api/v3/secrets/raw"
        self.secret_update_url = self.site_url + "/api/v3/secrets/raw/"
        self.client = pycurl.Curl()
        self.client.setopt(self.client.VERBOSE, 0)
        self.buffer = BytesIO()
    def reset_client(self):
        self.client = pycurl.Curl()
        self.client.setopt(self.client.VERBOSE, 0)
        self.buffer = BytesIO()
    
    def secret(self, project_id: str, env_slug: str, secret_name: str):
        return self.client.getSecret(options=GetSecretOptions(
            environment=env_slug,
            project_id=project_id,
            secret_name=secret_name
        ))
    
    def secrets(self, project_id: str, env_slug: str, to_dict: bool = False):
        self.reset_client()
        time.sleep(1)
        self.client.setopt(self.client.URL, self.token_url)
        p_uri = urlparse(self.site_url)
        if p_uri.scheme == 'https':
            self.client.setopt(self.client.CAINFO, certifi.where())
        self.client.setopt(pycurl.POST, 1)
        self.client.setopt(pycurl.HTTPHEADER, ['Content-Type: application/json'])
        self.client.setopt(self.client.READDATA, BytesIO(json.dumps({"clientId":self.client_id,"clientSecret":self.client_secret}).encode("UTF-8")))
        self.client.setopt(self.client.WRITEDATA, self.buffer)
        self.client.perform()
        if self.client.getinfo(pycurl.HTTP_CODE) == 200:
            self.token = json.loads(self.buffer.getvalue())
        else:
            return self.client.getinfo(pycurl.HTTP_CODE)
        self.reset_client()
        time.sleep(1)
        self.client.setopt(pycurl.HTTPHEADER, ['Authorization: Bearer ' + self.token["accessToken"]])
        self.client.setopt(self.client.URL, self.secrets_url + "?workspaceId=" + project_id+ "&environment=" + env_slug + "&expandSecretReferences=true")
        self.client.setopt(self.client.WRITEDATA, self.buffer)
        self.client.perform()
        if self.client.getinfo(pycurl.HTTP_CODE) == 200:
            return json.loads(self.buffer.getvalue())
        return self.client.getinfo(pycurl.HTTP_CODE)
    
    def write_env(self, project_id: str, env_slug:str , stringify: bool = False):
        tmpFile = "tmp-"+env_slug+".env"
        quote_mode = "never"
        if isfile(tmpFile):
            remove(tmpFile)
        if stringify:
            quote_mode = "always"
        secrets = self.secrets(project_id=project_id, env_slug=env_slug)
        for secret in secrets["secrets"]:
            set_key(dotenv_path=tmpFile, key_to_set=secret["secretKey"], value_to_set=secret["secretValue"], quote_mode=quote_mode)
        return tmpFile
    
    def write_netrc(self, project_id: str, env_slug:str):
        tmpFile = "tmp-"+env_slug+".env"
        if isfile(tmpFile):
            remove(tmpFile)
        content = "machine "
        secrets = self.secrets(project_id=project_id, env_slug=env_slug, to_dict=True)
        machine = ""
        login = ""
        password = ""
        for secret in secrets["secrets"]:
            if secret["secretKey"] == "MACHINE":
                machine = "machine " + secret["secretValue"] + '\n'
            if secret["secretKey"] == "LOGIN":
                login = "login " + secret["secretValue"] + '\n'
            if secret["secretKey"] == "PASSWORD":
                password = "password " + secret["secretValue"] + '\n'
        content = machine
        content = content + login
        content = content + password
        with open(tmpFile, 'w') as netrc:
            netrc.write(content)
        return tmpFile
    
    def write_pip_conf(self, project_id: str, env_slug:str):
        tmpFile = "tmp-"+env_slug+".env"
        if isfile(tmpFile):
            remove(tmpFile)
        secrets = self.secrets(project_id=project_id, env_slug=env_slug, to_dict=True)
        config = configparser.ConfigParser()
        config["global"] = {}
        for secret in secrets["secrets"]:
            if secret["secretKey"] == "EXTRA-INDEX-URL":
                config["global"]["extra-index-url"] = secret["secretValue"]
        with open(tmpFile, 'w') as pip_config:
            config.write(pip_config)
        return tmpFile
    
    def write_pypirc(self, project_id: str, env_slug:str):
        tmpFile = "tmp-"+env_slug+".env"
        if isfile(tmpFile):
            remove(tmpFile)
        secrets = self.secrets(project_id=project_id, env_slug=env_slug, to_dict=True)
        config = configparser.ConfigParser()
        config["distutils"] = {}
        config["distutils"]["index-servers"] = "\npypi\npantopix\n"
        config["pypi"] = {}
        config["pantopix"] = {}
        for secret in secrets["secrets"]:
            if secret["secretKey"] == "PYPIRC_URL":
                config["pantopix"]["repository"] = secret["secretValue"]
            if secret["secretKey"] == "LOGIN":
                config["pantopix"]["username"] = secret["secretValue"]
            if secret["secretKey"] == "PASSWORD":
                config["pantopix"]["password"] = secret["secretValue"]
        with open(tmpFile, 'w') as pypirc_config:
            config.write(pypirc_config)
        return tmpFile
    
    def write_xml(self, project_id: str, env_slug:str):
            tmpFile = "tmp-"+env_slug+".settings.xml"
            if isfile(tmpFile):
                remove(tmpFile)
            root = etree.Element("settings")
            servers = etree.SubElement(root, "servers")
            server = etree.SubElement(servers, "server")
            secrets = self.secrets(project_id=project_id, env_slug=env_slug)
            for secret in secrets["secrets"]:
                secretElement = etree.SubElement(server, secret["secretKey"].lower())
                secretElement.text = secret["secretValue"]
            etree.ElementTree(root).write(tmpFile, pretty_print=True)
            return tmpFile
    
    def write_yaml(self, project_id: str, env_slug:str):
        tmpFile = "tmp-"+env_slug+".lakectl.yaml"
        if isfile(tmpFile):
            remove(tmpFile)
        data = {
            "credentials": {
                "access_key_id": None,
                "secret_access_key": None,
            },
            "experimental": {
                "local": {
                    "posix_permissions": {
                        "enabled": False
                    }
                }
            },
            "metastore":{
                "glue": {
                    "catalog_id": ""
                },
                "hive": {
                    "db_location_uri": "file:/user/hive/warehouse/",
                    "uri":""
                }
            },
            "server": {
                "endpoint_url":"https://lakefs.hz.pantopix.net",
                "retries": {
                    "enabled": True,
                    "max_attempts": 4,
                    "max_wait_interval": "30s",
                    "min_wait_interval": "200ms"
                }
            }

        }
        secrets = self.secrets(project_id=project_id, env_slug=env_slug)
        for secret in secrets["secrets"]:
            if secret["secretKey"] == "LAKEFS_ACCESS_KEY":
                data["credentials"]["access_key_id"] = secret["secretValue"]
            if secret["secretKey"] == "LAKEFS_SECRET_KEY":
                data["credentials"]["secret_access_key"] = secret["secretValue"]
        dump(data, open(tmpFile,"w"))
        return tmpFile
    
    def bytes_to_file(self, fname: str, content: bytes):
        with open(fname, 'wb') as f:
            f.write(content)
        return fname
    
    def bytes_to_dict(self, content: bytes):
        tmp_secrets_file = 'secrets.env'
        with open(tmp_secrets_file, 'wb') as sf:
            sf.write(content)
            sf.close()
        conf = dotenv_values(tmp_secrets_file)
        remove(tmp_secrets_file)
        return conf
    
    def str_to_file(self, fname: str, content: str):
        with open(fname, "w") as f:
            f.write(content)
        return fname
    
    def create_google_creds(self, secrets: dict):
        if "GOOGLE_APPLICATION_CREDENTIALS_BASE64" in secrets:
            gcreds = base64.b64decode(secrets["GOOGLE_APPLICATION_CREDENTIALS_BASE64"])
            return self.bytes_to_file(secrets["GOOGLE_APPLICATION_CREDENTIALS"], gcreds)
        return ""
    
    def update(self, project_id: str, env_slug: str, key: str, value: str):
        self.reset_client()
        time.sleep(1)
        self.client.setopt(self.client.URL, self.token_url)
        p_uri = urlparse(self.site_url)
        if p_uri.scheme == 'https':
            self.client.setopt(self.client.CAINFO, certifi.where())
        self.client.setopt(pycurl.POST, 1)
        self.client.setopt(pycurl.HTTPHEADER, ['Content-Type: application/json'])
        self.client.setopt(self.client.READDATA, BytesIO(json.dumps({"clientId":self.client_id,"clientSecret":self.client_secret}).encode("UTF-8")))
        self.client.setopt(self.client.WRITEDATA, self.buffer)
        self.client.perform()
        if self.client.getinfo(pycurl.HTTP_CODE) == 200:
            self.token = json.loads(self.buffer.getvalue())
        else:
            return self.client.getinfo(pycurl.HTTP_CODE)
        self.reset_client()
        time.sleep(1)
        self.client.setopt(self.client.URL, self.secret_update_url + key)
        p_uri = urlparse(self.site_url)
        if p_uri.scheme == 'https':
            self.client.setopt(self.client.CAINFO, certifi.where())
        self.client.setopt(pycurl.CUSTOMREQUEST, "PATCH")
        self.client.setopt(pycurl.HTTPHEADER, ['Content-Type: application/json', 'Authorization: Bearer ' + self.token["accessToken"]])
        self.client.setopt(pycurl.POSTFIELDS, json.dumps({"workspaceId":project_id,"environment":env_slug,"secretPath":"/","secretValue":value}))
        self.client.setopt(self.client.WRITEDATA, self.buffer)
        self.client.perform()
        if self.client.getinfo(pycurl.HTTP_CODE) == 200:
            return self.buffer.getvalue()
        return self.client.getinfo(pycurl.HTTP_CODE)
    
    def get(self, project_id: str, env_slug: str, stringify: bool = False):
        tmpFile = self.write_env(project_id=project_id, env_slug=env_slug, stringify=stringify)
        with open(tmpFile, 'rb') as tmp:
            data = tmp.read()
            tmp.close()
        return data
    
    def get_pypirc(self, project_id: str, env_slug: str):
        tmpFile = self.write_pypirc(project_id=project_id, env_slug=env_slug)
        with open(tmpFile, 'rb') as tmp:
            data = tmp.read()
            tmp.close()
        return data
    
    def get_pip_conf(self, project_id: str, env_slug: str):
        tmpFile = self.write_pip_conf(project_id=project_id, env_slug=env_slug)
        with open(tmpFile, 'rb') as tmp:
            data = tmp.read()
            tmp.close()
        return data
    
    def get_pypi_netrc(self, project_id: str, env_slug: str):
        tmpFile = self.write_netrc(project_id=project_id, env_slug=env_slug)
        with open(tmpFile, 'rb') as tmp:
            data = tmp.read()
            tmp.close()
        return data
    
    def get_yaml(self, project_id: str, env_slug: str):
        tmpFile = self.write_yaml(project_id=project_id, env_slug=env_slug)
        with open(tmpFile, 'rb') as tmp:
            data = tmp.read()
            tmp.close()
        return data
    
    def get_xml(self, project_id: str, env_slug: str):
        tmpFile = self.write_xml(project_id=project_id, env_slug=env_slug)
        with open(tmpFile, 'rb') as tmp:
            data = tmp.read()
            tmp.close()
        return data

    def get_from_infisical(self, project_id: str, env_slug: str, mime_type: str = 'dot/env', stringify: bool = False):
        if mime_type == "maven/settings":
            return self.get_xml(project_id=project_id, env_slug=env_slug)
        elif mime_type == "lakefs/yaml":
            return self.get_yaml(project_id=project_id, env_slug=env_slug)
        elif mime_type == "pypi/netrc":
            return self.get_pypi_netrc(project_id=project_id, env_slug=env_slug)
        elif mime_type == "pip/conf":
            return self.get_pip_conf(project_id=project_id, env_slug=env_slug)
        elif mime_type == "pypi/rc":
            return self.get_pypirc(project_id=project_id, env_slug=env_slug)
        # return dot/env
        return self.write_env(project_id=project_id, env_slug=env_slug, stringify=stringify)

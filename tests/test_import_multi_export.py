import pytest
import time

from pxgraphdb.pxgraphdb import PXGraphDB

def test_backup_restore(settings):
    db = PXGraphDB(
        "unix:///var/run/docker.sock", 
        "unix:///var/run/docker.sock",
        exp_url='http://ke1.graphdb.px:7200',
        imp_url=['http://localhost:7202','http://localhost:7203'])
    print(db.export_import_repos("test-multi-exp-imp", ["CantoRepo","dataCatalog"]))

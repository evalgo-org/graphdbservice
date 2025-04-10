import pytest
import time

from pxknowledge_graph.pxgraphdb import PXGraphDB

def test_backup_restore(settings):
    db = PXGraphDB(
        "tcp://localhost:2375", 
        "tcp://localhost:2375",
        exp_url='http://ke1.graphdb.px:7200',
        imp_url=['http://localhost:7202','http://localhost:7203'])
    db.export_import_repos("test-multi-exp-imp", ["CantoRepo","dataCatalog"])

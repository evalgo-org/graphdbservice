import pytest
from os import environ

def test_backup_all(settings):
    settings.pxgraphdb.load_exp(environ.get('BKP_SRV'),environ.get('BKP_USER'),environ.get('BKP_PASS'))
    settings.pxgraphdb.load_bkp(environ.get('BKP_SRV'),environ.get('BKP_USER'),environ.get('BKP_PASS'))
    settings.pxgraphdb.backup_all('backup_c5')

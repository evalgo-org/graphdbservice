import pytest
import time

def test_backup_restore(settings):
    cnt = 'test-local-backup-restore'
    created = settings.pxgraphdb.default_ports(cnt, {settings.settings['RST_SRV_PORT']:settings.settings['RST_SRV_PORT']})
    if created['rebuild']:
        time.sleep(30)
    settings.pxgraphdb.load_bkp(
        settings.settings['BKP_SRV'],
        settings.settings['BKP_USER'],
        settings.settings['BKP_PASS']
    )
    settings.pxgraphdb.load_res(
        settings.settings['RST_SRV']+':'+settings.settings['RST_SRV_PORT']
    )
    repos = [
        'ProductData-KKH-EG-US',
        'ProductData-KKH-keys-EG',
        'ProductData-KKH-keys-US',
        'ProductData-MDM-keys-EG',
        'ProductData-MDM-keys-US',
        'ProductData-KKH-keys-EG',
        'ProductData-MDM-EG-US']
    settings.pxgraphdb.backup_restore("c5-to-local", repos)
    settings.pxgraphdb.default_remove(cnt)

def test_export_import_repos(settings):
    cnt = 'test-local-import-export'
    created = settings.pxgraphdb.default_ports(cnt, {settings.settings['RST_SRV_PORT']:settings.settings['RST_SRV_PORT']})
    if created['rebuild']:
        time.sleep(30)
    settings.pxgraphdb.load_exp(
        settings.settings['BKP_SRV'],
        settings.settings['BKP_USER'],
        settings.settings['BKP_PASS']
    )
    settings.pxgraphdb.load_imp(
        settings.settings['RST_SRV']+':'+settings.settings['RST_SRV_PORT']
    )
    repos = ['Chatbot-Demo', 'dataCatalog']
    settings.pxgraphdb.export_import_repos("c5-to-local-import", repos)
    settings.pxgraphdb.default_remove(cnt)

def test_export_import_repos_graphs(settings):
    cnt = 'test-local-import-export-graphs'
    created = settings.pxgraphdb.default_ports(cnt, {settings.settings['RST_SRV_PORT']:settings.settings['RST_SRV_PORT']})
    if created['rebuild']:
        time.sleep(30)
    settings.pxgraphdb.load_exp(
        settings.settings['BKP_SRV'],
        settings.settings['BKP_USER'],
        settings.settings['BKP_PASS']
    )
    settings.pxgraphdb.load_imp(
        settings.settings['RST_SRV']+':'+settings.settings['RST_SRV_PORT']
    )
    settings.pxgraphdb.export_import_repos_graphs(
        "c5-to-local-import-graphs", 
        'Chatbot-Demo', 
        ['https://data.kaeser.com/KKH/MDM','https://data.kaeser.com/KKH/MDM/PH'],
        'Chatbot-Demo')
    settings.pxgraphdb.default_remove(cnt)

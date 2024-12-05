import pytest
import time

def test_create_repository_on_prod(settings):
    settings.pxgraphdb.load_exp(
        settings.settings['KAESER_GRAPHDB_PROD_URL'],
        settings.settings['KAESER_GRAPHDB_PROD_USER'],
        settings.settings['KAESER_GRAPHDB_PROD_PASS'])
    start_repos = settings.pxgraphdb.exp_repos
    settings.pxgraphdb.exp.graphdb_repository_create('test-repo')
    settings.pxgraphdb.load_exp(
        settings.settings['KAESER_GRAPHDB_PROD_URL'],
        settings.settings['KAESER_GRAPHDB_PROD_USER'],
        settings.settings['KAESER_GRAPHDB_PROD_PASS'])
    one_more = settings.pxgraphdb.exp_repos
    assert len(one_more) > len(start_repos)
    settings.pxgraphdb.exp.graphdb_repository_delete('test-repo')
    settings.pxgraphdb.load_exp(
        settings.settings['KAESER_GRAPHDB_PROD_URL'],
        settings.settings['KAESER_GRAPHDB_PROD_USER'],
        settings.settings['KAESER_GRAPHDB_PROD_PASS'])
    end_repos = settings.pxgraphdb.exp_repos
    assert len(start_repos) == len(end_repos)

def test_export_import_c5_prod(settings):
    settings.pxgraphdb.load_exp(
        settings.settings['BKP_SRV'],
        settings.settings['BKP_USER'],
        settings.settings['BKP_PASS']
    )
    exp_info = settings.pxgraphdb.exp.graphdb_repo('c5','ProductData-KKH-keys-EG')
    settings.pxgraphdb.load_imp(
        settings.settings['KAESER_GRAPHDB_PROD_URL'],
        settings.settings['KAESER_GRAPHDB_PROD_USER'],
        settings.settings['KAESER_GRAPHDB_PROD_PASS']
    )
    repos = [
        'ProductData-KKH-EG-US',
        'ProductData-KKH-keys-EG',
        'ProductData-KKH-keys-US',
        'ProductData-MDM-keys-EG',
        'ProductData-MDM-keys-US',
        'ProductData-KKH-keys-EG',
        'ProductData-MDM-EG-US']
    settings.pxgraphdb.imp.graphdb_repo_api('ProductData-KKH-keys-EG',exp_info['data'], exp_info['conf'])

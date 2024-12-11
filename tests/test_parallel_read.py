import pytest
import concurrent.futures


def reload_get_repositories(gdbi, url):
    gdbi.load_exp(url)
    return gdbi.exp.graphdb_repositories()

def test_read_all_repos(settings):
    all_graphdb_instances = [
        'http://ke-ingest.graphdb.px:7200',
        'http://ke1.graphdb.px:7200',
        'http://ke2.graphdb.px:7200',
        'http://dev.graphdb.px:7200',
        'http://ke-test.graphdb.px:7200'
        ]
    try:
        with concurrent.futures.ThreadPoolExecutor() as executor:
            futures = list(map(lambda url: executor.submit(reload_get_repositories, settings.pxgraphdb, url), all_graphdb_instances))
            all_repositories_results = list(map(lambda future: future.result(), concurrent.futures.as_completed(futures)))
            print(all_repositories_results)
            print(len(all_repositories_results))
    except Exception as e:
        print(str(e))

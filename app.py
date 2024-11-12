from dotenv import load_dotenv
from os import environ
import sys
from prefect import flow

from knowledge_graph import pxgraphdb

load_dotenv()

@flow(log_prints=True)
def export_import_repos_c5_ke1():
    pxgraphdb.export_import_repos_c5_ke1(['Vestas-demo'])

if __name__ == '__main__':
    pxgraphdb.create_repository('http://ke1.graphdb.px:7200', 'test-from-francisc-2')
    # args = sys.argv[1:]
    # if len(args) > 0:
    #     if args[0] == 'export_import_repos_c5_ke1':
    #         export_import_repos_c5_ke1()
    # else:
    #     print("poetry run python app.py [option]")
    #     print("options:")
    #     print("  export_import_repos_c5_ke1")

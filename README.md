# pxknowledge graph

## requirements on linux / macos bash

### install uv 
https://docs.astral.sh/uv/getting-started/installation/

### run the command
````
uv sync
````

## run different options
````
uv run python app.py [option]
````
- export_import_repos_c5_ke1


## deployments
````
uv run prefect deploy --all
````

## use this package in other projects import
````
uv add "pxknowledge-graph==0.0.35"

````

## use this package in othere projects in code
````
from pxknowledge_graph import pxgraphdb

pxgraphdb.create_repository('http://{xxx.graphdb.px}:7200', 'my-test-repository')

````

# publications
the publications folder contains all code needed to build and run on SAP BTP as a publications consumer.

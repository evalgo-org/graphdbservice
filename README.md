# pxknowledge graph

## requirements on linux / macos bash

### install uv 
https://docs.astral.sh/uv/getting-started/installation/

### .netrc file in your home folder on linux /home/<user>, macos /Users/<user> or windows /<Users|Benutzer>/<user>
````
machine cache.hz.pantopix.net
login <username>
password <s3cr3t>
````
### run the command
````
uv sync --index https://cache.hz.pantopix.net/repository/pypi-private/simple/
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
uv add "pxknowledge-graph==0.0.7"

````

## use this package in othere projects in code
````
from pxknowledge_graph import pxgraphdb

pxgraphdb.create_repository('http://{xxx.graphdb.px}:7200', 'my-test-repository')

````

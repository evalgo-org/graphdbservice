# pxknowledge graph

## requirements on linux / macos bash
````
export PREFECT_API_URL=http://pipelines.px/api
poetry env use python3.12
# get the NEXUS_USER from LOGIN and NEXUS_SECRET from password of the infisical project located here:
# https://secrets.mars.pantopix.net/project/1ee100bd-80a0-4a84-9747-b6ead2bf3dd8/secrets/overview
poetry config http-basic.pantopix ${NEXUS_USER} ${NEXUS_SECRET}
poetry install
````

## requirements on windows powershell
````
$env:PREFECT_API_URL="http://pipelines.px/api"
pip install poetry
poetry env use python
# get the NEXUS_USER from LOGIN and NEXUS_SECRET from password of the infisical project located here:
# https://secrets.mars.pantopix.net/project/1ee100bd-80a0-4a84-9747-b6ead2bf3dd8/secrets/overview
poetry config http-basic.pantopix ${NEXUS_USER} ${NEXUS_SECRET}
poetry install
````

## run different options
````
poetry run python app.py [option]
````
- export_import_repos_c5_ke1


## deployments
````
poetry run prefect deploy --all
````

## use this package in other projects import
````
poetry source add --priority=supplemental pantopix https://cache.hz.pantopix.net/repository/pypi-private/simple/
# get the NEXUS_USER from LOGIN and NEXUS_SECRET from password of the infisical project located here:
# https://secrets.mars.pantopix.net/project/1ee100bd-80a0-4a84-9747-b6ead2bf3dd8/secrets/overview
poetry config http-basic.pantopix ${NEXUS_USER} ${NEXUS_SECRET}
poetry add "pxknowledge-graph==0.0.7"

````

## use this package in othere projects in code
````
from pxknowledge_graph import pxgraphdb

pxgraphdb.create_repository('http://{xxx.graphdb.px}:7200', 'my-test-repository')

````

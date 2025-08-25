# kaeser-publications

## build docker image
- create .netrc file
- create pip.conf file
- copy publications.consumer.json to local directory
- run `task`

## run docker image
- crate .env file
- run `task run`

## .netrc
````
machine cache.hz.pantopix.net
login ${USER}
password ${PASSWORD}
````

## pip.conf
````
[global]
extra-index-url = https://cache.hz.pantopix.net/repository/pypi-private/simple
````

## run
````
poetry env use python3.12
poetry config http-basic.pantopix ${USER} ${SECRET}
poetry install
# edit .env
poetry run 
# copy an json message to the rabbit mq ui message queue for the consumer
````

## .env
````
DOCKER_HOST=
DOCKER_API_HOST=

GITEA_PANTOPIX_TOKEN=
GITEA_PANTOPIX_URL=
GITEA_PANTOPIX_USERNAME=

SECRETS_CLIENT_ID=
SECRETS_CLIENT_SECRET=
SECRETS_SITE_URL=

RABBITMQ_CONN_URL=
RABBITMQ_QUEUE=
RABBITMQ_MAX_CONSUMERS=1

TARGET_GRAPH_INSTANCE=
COMPUTE_GRAPH_SERVER=

DBUSER=
DBPW=
METASELECT_API_KEY=
````

## message
````
{
    "version":"v1",
    "kaeser-publications": [{
        "selectionTemplate": "https://data.kaeser.com/KKH/SelectionTemplateEG",
        "language_tag": ["en-GB", "de-DE"],
        "include_prices": "true",
        "include_characteristics": "true",
        "include_options": "true",
        "include_associatedArticles": "true",
        "salesOrg": "0002",
        "region": "EG",
        "country": "DE",
        "repoConsumption": "Consumption-Navigator-DE-Intern-All",
        "repoTransformation": "Transformation-Navigator-DE-Intern"
    }]
}
````

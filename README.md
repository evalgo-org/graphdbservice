# pxgraphdb service

## contract
````
{
    "version": "v0.0.1",
    "tasks": [
        {
            "action": "repo-migration",
            "src": {
                "url":"",
                "username": "",
                "password": "",
                "repo": ""
            },
            "tgt": {
                "url":"",
                "username": "",
                "password": "",
                "repo" :""
            }
        },
        {
            "action": "graph-migration",
            "src": {
                "url":"",
                "username": "",
                "password": "",
                "repo": "",
                "graph": ""
            },
            "tgt": {
                "url":"",
                "username": "",
                "password": "",
                "repo" :"",
                "graph": ""
            }
        },
        {
            "action": "repo-delete",
            "tgt": {
                "url":"",
                "username": "",
                "password": "",
                "repo" :""
            }
        },
        {
            "action": "graph-delete",
            "tgt": {
                "url":"",
                "username": "",
                "password": "",
                "repo" :"",
                "graph": ""
            }
        },
        {
            "action": "repo-create",
            "tgt": {
                "url":"",
                "username": "",
                "password": "",
                "repo" :""
            }
        },
        {
            "action": "graph-import",
            "tgt": {
                "url":"",
                "username": "",
                "password": "",
                "repo" :"",
                "graph": ""
            }
        },
        {
            "action": "repo-rename",
            "tgt": {
                "url":"",
                "username": "",
                "password": "",
                "repo_old": "",
                "repo_new": ""
            }
        },
        {
            "action": "graph-rename",
            "tgt": {
                "url":"",
                "username": "",
                "password": "",
                "repo" :"",
                "graph_old": "",
                "graph_new": ""
            }
        }
    ]
}
````

## requests
````
API_KEY=1234567890
API_URL=http://service.pxgraphdb.px:8080

SRC_GRAPHDB=http://dev.graphdb.px:7200
TGT_GRAPHDB=http://build-003.graphdb.px:7200
REPO=CantoRepo
GRAPH=https://data.kaeser.com/KKH/DCAT/CANTO

# repo-migration
curl -X POST \
    -H "X-API-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "{\"version\": \"v0.0.1\",\"tasks\": [{\"action\": \"repo-migration\",\"src\": {\"url\":\"${SRC_GRAPHDB}\",\"username\": \"\",\"password\": \"\",\"repo\": \"${REPO}\"},\"tgt\": {\"url\":\"${TGT_GRAPHDB}\",\"username\": \"\",\"password\": \"\",\"repo\":\"${REPO}\"}}]}" \
    ${API_URL}/v1/api/action

# repo-rename
curl -X POST \
    -H "X-API-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "{\"version\": \"v0.0.1\",\"tasks\": [{\"action\": \"repo-rename\",\"tgt\": {\"url\":\"${TGT_GRAPHDB}\",\"username\":\"\",\"password\":\"\",\"repo_old\":\"${REPO}\",\"repo_new\":\"${REPO}Renamed\"}}]}" \
    ${API_URL}/v1/api/action

# repo-delete
curl -X POST \
    -H "X-API-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "{\"version\": \"v0.0.1\",\"tasks\": [{\"action\": \"repo-delete\",\"tgt\": {\"url\":\"${TGT_GRAPHDB}\",\"username\": \"\",\"password\": \"\",\"repo\":\"${REPO}Renamed\"}}]}" \
    ${API_URL}/v1/api/action

# # repo-create
curl -X POST \
    -H "x-api-key: ${API_KEY}" \
    -F "request={\"version\":\"v0.0.1\",\"tasks\":[{\"action\":\"repo-create\",\"tgt\":{\"url\":\"${TGT_GRAPHDB}\",\"username\":\"\",\"password\":\"\",\"repo\":\"autocreated\"}}]}" \
    -F "task_0_config=@repo-config.ttl" \
    ${API_URL}/v1/api/action

# # repo-import
curl -X POST \
    -H "X-API-Key: ${API_KEY}" \
    -F "request={\"version\":\"v0.0.1\",\"tasks\":[{\"action\":\"repo-import\",\"tgt\":{\"url\":\"${TGT_GRAPHDB}\",\"username\":\"\",\"password\":\"\",\"repo\":\"autocreated\"}}]}" \
    -F "task_0_files=@autocreated.brf" \
    ${API_URL}/v1/api/action

# # graph-import
curl -X POST \
    -H "X-API-Key: ${API_KEY}" \
    -F "task_0_files=@6d81da45b5d4e483fdbece2ba950eb6a.brf" \
    -F "request={\"version\": \"v0.0.1\",\"tasks\": [{\"action\": \"graph-import\",\"tgt\": {\"url\":\"${TGT_GRAPHDB}\",\"username\": \"\",\"password\": \"\",\"repo\":\"autocreated\", \"graph\":\"${GRAPH}\"}}]}" \
    ${API_URL}/v1/api/action

# # graph-delete
curl -X POST \
    -H "X-API-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "{\"version\": \"v0.0.1\",\"tasks\": [{\"action\": \"graph-delete\",\"tgt\": {\"url\":\"${TGT_GRAPHDB}\",\"username\": \"\",\"password\": \"\",\"repo\":\"autocreated\", \"graph\":\"${GRAPH}\"}}]}" \
    ${API_URL}/v1/api/action

# # graph-migration
curl -X POST \
    -H "X-API-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "{\"version\": \"v0.0.1\",\"tasks\": [{\"action\": \"graph-migration\",\"src\": {\"url\":\"${SRC_GRAPHDB}\",\"username\": \"\",\"password\": \"\",\"repo\": \"${REPO}\", \"graph\":\"${GRAPH}\"},\"tgt\": {\"url\":\"${TGT_GRAPHDB}\",\"username\": \"\",\"password\": \"\",\"repo\":\"autocreated\", \"graph\":\"${GRAPH}\"}}]}" \
    ${API_URL}/v1/api/action

# # graph-rename
curl -X POST \
    -H "X-API-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "{\"version\": \"v0.0.1\",\"tasks\": [{\"action\": \"graph-rename\",\"tgt\": {\"url\": \"${TGT_GRAPHDB}\",\"username\": \"\",\"password\": \"\",\"repo\": \"autocreated\",\"graph_old\":\"${GRAPH}\",\"graph_new\": \"${GRAPH}_RENAMED\"}}]}" \
    ${API_URL}/v1/api/action

````

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

curl -X POST \
    -H "X-API-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"version": "v0.0.1","tasks": [{"action": "repo-migration","src": {"url":"http://dev.graphdb.px:7200","username": "","password": "","repo": "CantoRepo"},"tgt": {"url":"http://build-003.graphdb.px:7200","username": "","password": "","repo" :"CantoRepo"}}]}' \
    ${API_URL}/v1/api/action

curl -X POST \
    -H "X-API-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"version": "v0.0.1","tasks": [{"action": "repo-rename","tgt": {"url": "http://build-003.graphdb.px:7200","username": "","password": "","repo_old": "CantoRepo","repo_new": "CantoRepoRenamed"}}]}' \
    ${API_URL}/v1/api/action

curl -X POST \
    -H "X-API-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"version": "v0.0.1","tasks": [{"action": "repo-delete","tgt": {"url":"http://build-003.graphdb.px:7200","username": "","password": "","repo" :"CantoRepoRenamed"}}]}' \
    ${API_URL}/v1/api/action

curl -X POST \
  -H "x-api-key: ${API_KEY}" \
  -F 'request={"version":"v0.0.1","tasks":[{"action":"repo-create","tgt":{"url":"http://build-003.graphdb.px:7200","username":"","password":"","repo":"autocreated"}}]}' \
  -F "task_0_config=@repo-config.ttl" \
  ${API_URL}/v1/api/action

curl -v -X POST \
    -H "X-API-Key: ${API_KEY}" \
    -F "task_0_files=@6d81da45b5d4e483fdbece2ba950eb6a.brf" \
    -F 'request={"version": "v0.0.1","tasks": [{"action": "graph-import","tgt": {"url":"http://build-003.graphdb.px:7200","username": "","password": "","repo" :"autocreated", "graph":"https://data.kaeser.com/KKH/DCAT/CANTO"}}]}' \
    ${API_URL}/v1/api/action

curl -X POST \
    -H "X-API-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"version": "v0.0.1","tasks": [{"action": "graph-delete","tgt": {"url":"http://build-003.graphdb.px:7200","username": "","password": "","repo" :"CantoRepo", "graph":"https://data.kaeser.com/KKH/DCAT/CANTO"}}]}' \
    ${API_URL}/v1/api/action

curl -X POST \
    -H "X-API-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"version": "v0.0.1","tasks": [{"action": "graph-migration","src": {"url":"http://dev.graphdb.px:7200","username": "","password": "","repo": "CantoRepo", "graph":"https://data.kaeser.com/KKH/DCAT/CANTO"},"tgt": {"url":"http://build-003.graphdb.px:7200","username": "","password": "","repo" :"CantoRepo", "graph":"https://data.kaeser.com/KKH/DCAT/CANTO"}}]}' \
    ${API_URL}/v1/api/action

curl -X POST \
    -H "X-API-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"version": "v0.0.1","tasks": [{"action": "graph-rename","tgt": {"url": "http://build-003.graphdb.px:7200","username": "","password": "","repo": "CantoRepo","graph_old":"https://data.kaeser.com/KKH/DCAT/CANTO","graph_new": "https://data.kaeser.com/KKH/DCAT/CANTO_RENAMED"}}]}' \
    ${API_URL}/v1/api/action

````

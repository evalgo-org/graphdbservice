curl -X POST \
    -H "X-API-Key: 1234567890" \
    -H "Content-Type: application/json" \
    -d '{"version": "v0.0.1","tasks": [{"action": "graph-rename","tgt": {"url": "http://build-003.graphdb.px:7200","username": "","password": "","repo": "CantoRepo","graph_old":"https://data.kaeser.com/KKH/DCAT/CANTO","graph_new": "https://data.kaeser.com/KKH/DCAT/CANTO_RENAMED"}}]}' \
    http://localhost:8081/v1/api/action

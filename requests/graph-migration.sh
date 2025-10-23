curl -X POST \
    -H "X-API-Key: 1234567890" \
    -H "Content-Type: application/json" \
    -d '{"version": "v0.0.1","tasks": [{"action": "graph-migration","src": {"url":"http://dev.graphdb.px:7200","username": "","password": "","repo": "CantoRepo", "graph":"https://data.kaeser.com/KKH/DCAT/CANTO"},"tgt": {"url":"http://build-003.graphdb.px:7200","username": "","password": "","repo" :"CantoRepo", "graph":"https://data.kaeser.com/KKH/DCAT/CANTO"}}]}' \
    http://localhost:8081/v1/api/action

API_KEY=1234567890
API_URL=http://service.pxgraphdb.px:8080

curl -X POST \
    -H "X-API-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"version": "v0.0.1","tasks": [{"action": "graph-migration","src": {"url":"http://dev.graphdb.px:7200","username": "","password": "","repo": "CantoRepo", "graph":"https://data.kaeser.com/KKH/DCAT/CANTO"},"tgt": {"url":"http://build-003.graphdb.px:7200","username": "","password": "","repo" :"CantoRepo", "graph":"https://data.kaeser.com/KKH/DCAT/CANTO"}}]}' \
    ${API_URL}/v1/api/action

API_KEY=1234567890
API_URL=http://service.pxgraphdb.px:8080
API_URL=http://localhost:8081

curl -v -X POST \
    -H "X-API-Key: ${API_KEY}" \
    -F "task_0_files=@6d81da45b5d4e483fdbece2ba950eb6a.brf" \
    -F 'request={"version": "v0.0.1","tasks": [{"action": "graph-import","tgt": {"url":"http://build-003.graphdb.px:7200","username": "","password": "","repo" :"autocreated", "graph":"https://data.kaeser.com/KKH/DCAT/CANTO"}}]}' \
    ${API_URL}/v1/api/action

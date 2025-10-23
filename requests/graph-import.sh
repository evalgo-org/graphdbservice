curl -X POST \
    -H "X-API-Key: 1234567890" \
    -F "task_0_files=@6d81da45b5d4e483fdbece2ba950eb6a.brf" \
    -F 'request={"version": "v0.0.1","tasks": [{"action": "graph-import","tgt": {"url":"http://build-003.graphdb.px:7200","username": "","password": "","repo" :"CantoRepo", "graph":"https://data.kaeser.com/KKH/DCAT/CANTO"}}]}' \
    http://localhost:8081/v1/api/action

API_KEY=1234567890
API_URL=http://localhost:8081

curl -X POST \
    -H "X-API-Key: ${API_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"version": "v0.0.1","tasks": [{"action": "repo-delete","tgt": {"url":"http://build-003.graphdb.px:7200","username": "","password": "","repo" :"CantoRepo"}}]}' \
    ${API_URL}/v1/api/action

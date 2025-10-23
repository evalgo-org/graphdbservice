API_KEY=1234567890
API_URL=http://service.pxgraphdb.px:8080

curl -X POST \
  -H "x-api-key: ${API_KEY}" \
  -F 'request={"version":"v0.0.1","tasks":[{"action":"repo-create","tgt":{"url":"http://build-003.graphdb.px:7200","username":"","password":"","repo":"autocreated"}}]}' \
  -F "task_0_config=@repo-config.ttl" \
  ${API_URL}/v1/api/action

curl -X POST \
  -H "x-api-key: 1234567890" \
  -F 'request={"version":"v0.0.1","tasks":[{"action":"repo-create","tgt":{"url":"http://build-003.graphdb.px:7200","username":"","password":"","repo":"autocreated"}}]}' \
  -F "task_0_config=@repo-config.ttl" \
  http://localhost:8081/v1/api/action

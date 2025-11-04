# Repository Creation with Turtle Configuration

This example demonstrates how to create a new GraphDB repository using a Turtle configuration file with the semantic JSON-LD API.

## Overview

Creating a repository requires:
1. A JSON-LD `CreateAction` describing the repository
2. A Turtle (`.ttl`) configuration file with repository settings
3. Multipart/form-data request to upload both

## Files

- **07-repo-create-with-config.json**: JSON-LD CreateAction definition
- **repo-config-example.ttl**: Sample Turtle repository configuration

## Method 1: Using curl with Multipart Upload

The semantic API supports multipart/form-data to upload the Turtle config file along with the JSON-LD action.

### Request Format

```bash
curl -X POST http://localhost:8080/v1/api/semantic/action \
  -H "x-api-key: your-api-key-here" \
  -F "action=@examples/workflows/07-repo-create-with-config.json;type=application/json" \
  -F "config=@examples/workflows/repo-config-example.ttl"
```

### Explanation

- **`-F "action=@file.json"`**: Uploads the JSON-LD CreateAction
  - The field name must be `action`
  - Content-Type is automatically set to `application/json`

- **`-F "config=@file.ttl"`**: Uploads the Turtle configuration
  - The field name must be `config`
  - Can be any Turtle/RDF file

### Complete Example

```bash
# Set your API key
export API_KEY="your-api-key-here"

# Create repository with config file
curl -X POST http://localhost:8080/v1/api/semantic/action \
  -H "x-api-key: ${API_KEY}" \
  -F "action=@examples/workflows/07-repo-create-with-config.json;type=application/json" \
  -F "config=@examples/workflows/repo-config-example.ttl"
```

### Expected Response

```json
{
  "@context": "https://schema.org",
  "@type": "CreateAction",
  "identifier": "create-new-repository",
  "actionStatus": "CompletedActionStatus",
  "result": {
    "action": "repo-create",
    "status": "completed",
    "message": "Repository created successfully",
    "repo": "my-new-repository",
    "config_file": "repo-config-example.ttl"
  }
}
```

## Method 2: Using Legacy API Endpoint

The legacy endpoint `/v1/api/action` also supports multipart uploads:

```bash
curl -X POST http://localhost:8080/v1/api/action \
  -H "x-api-key: ${API_KEY}" \
  -F 'request={"version":"v0.0.1","tasks":[{"action":"repo-create","tgt":{"url":"http://localhost:7200","username":"admin","password":"admin","repo":"my-new-repository"}}]}' \
  -F "task_0_config=@examples/workflows/repo-config-example.ttl"
```

## Customizing the Turtle Configuration

Edit `repo-config-example.ttl` to customize:

### Repository ID
```turtle
rep:repositoryID "my-new-repository" ;
```

### Ruleset (Reasoning Level)
```turtle
graphdb:ruleset "rdfsplus-optimized" ;
```

Available rulesets:
- `empty` - No inference
- `rdfs` - RDFS inference
- `rdfsplus-optimized` - RDFS with optimization
- `owl-horst` - OWL Horst profile
- `owl-max` - Maximum OWL reasoning

### Storage Settings
```turtle
graphdb:entity-index-size "10000000" ;
graphdb:storage-folder "storage" ;
```

### Full-Text Search
```turtle
graphdb:enable-fts-index "true" ;
```

## Workflow Integration

To use in an ItemList workflow:

```json
{
  "@context": "https://schema.org",
  "@type": "ItemList",
  "identifier": "setup-new-environment",
  "name": "Setup New GraphDB Environment",
  "parallel": false,
  "itemListElement": [
    {
      "@type": "ListItem",
      "position": 1,
      "item": {
        "@type": "CreateAction",
        "identifier": "create-repo",
        "name": "Create Repository",
        "result": {
          "@type": "SoftwareSourceCode",
          "identifier": "my-new-repository",
          "additionalProperty": {
            "serverUrl": "http://localhost:7200",
            "username": "admin",
            "password": "admin"
          }
        }
      }
    }
  ]
}
```

Note: File uploads in workflows are sent as separate form fields.

## Programmatic Usage (Go)

If you're using the EVE semantic library in your own Go code:

```go
package main

import (
    "bytes"
    "mime/multipart"
    "net/http"

    "eve.evalgo.org/semantic"
)

func createRepositoryWithConfig() error {
    // Create multipart form
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)

    // Add JSON-LD action
    actionJSON := `{
        "@context": "https://schema.org",
        "@type": "CreateAction",
        "identifier": "create-repo",
        "result": {
            "@type": "SoftwareSourceCode",
            "identifier": "my-repo",
            "additionalProperty": {
                "serverUrl": "http://localhost:7200",
                "username": "admin",
                "password": "admin"
            }
        }
    }`
    writer.WriteField("action", actionJSON)

    // Add config file
    configFile, _ := writer.CreateFormFile("config", "repo-config.ttl")
    configFile.Write([]byte("@prefix rep: ... ")) // Your Turtle content

    writer.Close()

    // Send request
    req, _ := http.NewRequest("POST", "http://localhost:8080/v1/api/semantic/action", body)
    req.Header.Set("Content-Type", writer.FormDataContentType())
    req.Header.Set("x-api-key", "your-api-key")

    client := &http.Client{}
    resp, err := client.Do(req)
    // Handle response...

    return err
}
```

## Troubleshooting

### Error: "Missing 'action' field"
- Ensure the form field is named exactly `action`
- Verify JSON-LD is valid

### Error: "Failed to create repository"
- Check repository doesn't already exist
- Verify GraphDB server URL is accessible
- Check credentials are correct

### Error: "Invalid Turtle configuration"
- Validate Turtle syntax
- Ensure `rep:repositoryID` matches the identifier in JSON

### Repository created but empty
- Turtle config was applied
- Use graph-import to add data

## Related Examples

- **06-repo-create-example.json**: CreateAction without file (shows expected structure)
- **05-repo-migration-ontotext.json**: Full repository migration
- **02-multi-graph-migration.json**: Graph-level operations

## References

- [GraphDB Configuration Documentation](http://graphdb.ontotext.com/documentation/)
- [Schema.org CreateAction](https://schema.org/CreateAction)
- [EVE Semantic Library](https://github.com/evalgo/eve)

# Graph Data Import with RDF Files

This example demonstrates how to import RDF data files into a GraphDB repository using the semantic JSON-LD API with multipart/form-data upload.

## Overview

Importing data requires:
1. A JSON-LD `UploadAction` describing the target graph and repository
2. An RDF data file (Turtle, RDF/XML, N-Triples, N-Quads, TriG, JSON-LD, or Binary RDF)
3. Multipart/form-data request to upload both

## Files

- **08-graph-import-with-data.json**: JSON-LD UploadAction definition
- **sample-data.ttl**: Example Turtle data file with RDF triples
- **import-data-example.sh**: Helper script for easy execution

## Supported RDF Formats

GraphDB supports importing the following RDF formats:

| Format | File Extension | MIME Type | Description |
|--------|---------------|-----------|-------------|
| Turtle | `.ttl` | `text/turtle` | Human-readable, compact |
| RDF/XML | `.rdf`, `.xml` | `application/rdf+xml` | XML-based format |
| N-Triples | `.nt` | `application/n-triples` | Line-based, simple |
| N-Quads | `.nq` | `application/n-quads` | N-Triples with graph context |
| TriG | `.trig` | `application/trig` | Turtle with named graphs |
| JSON-LD | `.jsonld` | `application/ld+json` | JSON-based RDF |
| Binary RDF | `.brf` | `application/x-binary-rdf` | GraphDB proprietary, fastest |

## Method 1: Using curl with Multipart Upload

### Basic Import

```bash
curl -X POST http://localhost:8080/v1/api/semantic/action \
  -H "x-api-key: your-api-key-here" \
  -F "action=@08-graph-import-with-data.json" \
  -F "data=@sample-data.ttl"
```

### Import Multiple Files

You can import multiple data files in a single request:

```bash
curl -X POST http://localhost:8080/v1/api/semantic/action \
  -H "x-api-key: your-api-key-here" \
  -F "action=@08-graph-import-with-data.json" \
  -F "data=@file1.ttl" \
  -F "data=@file2.rdf" \
  -F "data=@file3.nt"
```

### Import Binary RDF (BRF)

Binary RDF format is GraphDB's proprietary format for fastest import:

```bash
# First, export from source repository as BRF
curl -X GET "http://source-graphdb:7200/repositories/source-repo/statements?infer=false" \
  -H "Accept: application/x-binary-rdf" \
  -o export.brf

# Then import to target repository
curl -X POST http://localhost:8080/v1/api/semantic/action \
  -H "x-api-key: your-api-key" \
  -F "action=@08-graph-import-with-data.json" \
  -F "data=@export.brf"
```

## Method 2: Using the Shell Script

The provided script makes it easy to import data:

```bash
# Basic usage with default settings
./import-data-example.sh

# Custom data file
DATA_FILE="my-data.rdf" ./import-data-example.sh

# Custom graph URI and repository
GRAPH_URI="http://example.org/my-graph" \
REPO_NAME="production-repo" \
DATA_FILE="production-data.ttl" \
./import-data-example.sh

# All configurable options
API_KEY="your-key" \
API_URL="http://localhost:8080" \
GRAPHDB_URL="http://graphdb:7200" \
REPO_NAME="my-repo" \
GRAPH_URI="http://example.org/graph/data" \
DATA_FILE="data.ttl" \
./import-data-example.sh
```

## JSON-LD UploadAction Structure

The `08-graph-import-with-data.json` file contains:

```json
{
  "@context": "https://schema.org",
  "@type": "UploadAction",
  "identifier": "import-graph-data",
  "object": {
    "@type": "Dataset",
    "identifier": "http://example.org/graph/imported-data",
    "encodingFormat": "text/turtle"
  },
  "target": {
    "@type": "DataCatalog",
    "identifier": "repository-name",
    "url": "http://graphdb-server:7200",
    "additionalProperty": {
      "serverUrl": "http://graphdb-server:7200",
      "username": "admin",
      "password": "password"
    }
  }
}
```

### Key Fields

- **object.identifier**: The named graph URI where data will be imported
- **object.encodingFormat**: RDF format of the data file
- **target.identifier**: Target repository name
- **target.additionalProperty**: GraphDB server credentials

## Sample Data File

The included `sample-data.ttl` contains:

- **10 entities** across different types (Person, Organization, Project, etc.)
- **~50 RDF triples** demonstrating various relationships
- **Multiple namespaces** (FOAF, Dublin Core, custom)
- **Typed literals** (dates, integers, strings)

### Example Triples

```turtle
ex:person1 a foaf:Person ;
    foaf:name "Alice Johnson" ;
    foaf:email "alice.johnson@example.org" ;
    foaf:knows ex:person2 , ex:person3 .

ex:project1 a ex:Project ;
    dc:title "GraphDB Migration Tool" ;
    dc:creator ex:person1 ;
    ex:organization ex:org1 .
```

## Querying Imported Data

After successful import, query the data using SPARQL:

### Get All Triples from Imported Graph

```sparql
SELECT *
FROM <http://example.org/graph/imported-data>
WHERE {
  ?subject ?predicate ?object
}
LIMIT 100
```

### Count Triples by Type

```sparql
PREFIX rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#>
PREFIX foaf: <http://xmlns.com/foaf/0.1/>

SELECT ?type (COUNT(?instance) AS ?count)
FROM <http://example.org/graph/imported-data>
WHERE {
  ?instance rdf:type ?type .
}
GROUP BY ?type
ORDER BY DESC(?count)
```

### Find All People and Their Connections

```sparql
PREFIX foaf: <http://xmlns.com/foaf/0.1/>

SELECT ?person ?name ?knows ?knowsName
FROM <http://example.org/graph/imported-data>
WHERE {
  ?person a foaf:Person ;
          foaf:name ?name ;
          foaf:knows ?knows .
  ?knows foaf:name ?knowsName .
}
```

## Repository Import (Entire Repository)

To import an entire repository (not just a graph), use a slightly different action:

```json
{
  "@context": "https://schema.org",
  "@type": "UploadAction",
  "identifier": "import-repo-data",
  "target": {
    "@type": "SoftwareSourceCode",
    "identifier": "target-repo",
    "additionalProperty": {
      "serverUrl": "http://graphdb:7200",
      "username": "admin",
      "password": "password"
    }
  }
}
```

Then upload a BRF file:

```bash
curl -X POST http://localhost:8080/v1/api/semantic/action \
  -H "x-api-key: your-api-key" \
  -F "action=@repo-import.json" \
  -F "data=@repository-export.brf"
```

## Performance Tips

### For Large Files

1. **Use Binary RDF (BRF)** format - 5-10x faster than Turtle
2. **Split into multiple files** - Import in parallel using ItemList
3. **Disable inference** during import - Re-enable after
4. **Use N-Quads** for multi-graph imports

### Parallel Import Example

```json
{
  "@context": "https://schema.org",
  "@type": "ItemList",
  "parallel": true,
  "concurrency": 3,
  "itemListElement": [
    {
      "@type": "ListItem",
      "position": 1,
      "item": {
        "@type": "UploadAction",
        "object": { "identifier": "http://example.org/graph/part1" },
        "target": { "identifier": "my-repo" }
      }
    },
    {
      "@type": "ListItem",
      "position": 2,
      "item": {
        "@type": "UploadAction",
        "object": { "identifier": "http://example.org/graph/part2" },
        "target": { "identifier": "my-repo" }
      }
    }
  ]
}
```

Then upload:
```bash
curl -X POST http://localhost:8080/v1/api/semantic/action \
  -H "x-api-key: your-api-key" \
  -F "action=@parallel-import.json" \
  -F "data=@part1.ttl" \
  -F "data=@part2.ttl"
```

## Troubleshooting

### Error: "Failed to import data"
- Check repository exists: `curl http://graphdb:7200/repositories/my-repo/size`
- Verify credentials are correct
- Ensure GraphDB server is accessible

### Error: "Invalid RDF syntax"
- Validate your RDF file with a tool like [RDF Validator](http://www.w3.org/RDF/Validator/)
- Check file encoding is UTF-8
- Verify namespace prefixes are declared

### Error: "Graph already exists"
- By default, import adds to existing graph
- To replace graph, first delete it:
  ```bash
  curl -X DELETE http://graphdb:7200/repositories/my-repo/statements \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "context=<http://example.org/graph/imported-data>"
  ```

### Data not visible in queries
- Check if data was imported to correct graph:
  ```sparql
  SELECT DISTINCT ?g WHERE { GRAPH ?g { ?s ?p ?o } }
  ```
- Verify graph URI matches exactly (including trailing slashes)

## Workflow Integration

### Automated Data Ingestion

Combine with ScheduledAction for automated data imports:

```json
{
  "@context": "https://schema.org",
  "@type": "ScheduledAction",
  "identifier": "daily-data-import",
  "schedule": {
    "@type": "Schedule",
    "repeatFrequency": "P1D",
    "startTime": "03:00:00"
  },
  "additionalProperty": {
    "url": "http://localhost:8080/v1/api/semantic/action",
    "httpMethod": "POST",
    "body": {
      "@type": "UploadAction",
      "object": {
        "@type": "Dataset",
        "identifier": "http://example.org/graph/daily-updates"
      },
      "target": {
        "@type": "DataCatalog",
        "identifier": "production-repo"
      }
    }
  }
}
```

## Related Examples

- **07-repo-create-with-config.json**: Create repository before importing
- **05-repo-migration-ontotext.json**: Migrate entire repository
- **02-multi-graph-migration.json**: Migrate multiple graphs

## References

- [GraphDB Import Documentation](http://graphdb.ontotext.com/documentation/standard/loading-data.html)
- [Schema.org UploadAction](https://schema.org/UploadAction)
- [RDF 1.1 Formats](https://www.w3.org/TR/rdf11-primer/)
- [SPARQL 1.1 Query Language](https://www.w3.org/TR/sparql11-query/)

# GraphDB Semantic Workflow Examples

This directory contains example workflows for orchestrating GraphDB operations using **When** (semantic task orchestrator) and **pxgraphservice** (GraphDB migration service).

## Architecture

```
When Orchestrator (Scheduler)
    ↓ (HTTP POST with JSON-LD)
pxgraphservice (/v1/api/semantic/action)
    ↓ (Parse Schema.org actions)
GraphDB Operations (Migrations, Backups, etc.)
```

## Key Features

✅ **Semantic-First**: Full Schema.org JSON-LD compliance
✅ **Schedulable**: Run on any schedule (hourly, daily, weekly)
✅ **Parallel Execution**: Run multiple migrations concurrently
✅ **Dependency Management**: Chain tasks with `dependsOn`
✅ **Machine-Readable**: AI agents can understand and generate workflows
✅ **Introspectable**: Query workflows by semantic type

## Workflow Files

### 01-nightly-backup.json
**Purpose**: Daily backup of production GraphDB repository

**Action Type**: `TransferAction` (repository migration)

**Schedule**: Every day at 2:00 AM

**Usage with When**:
```bash
# Submit to When orchestrator
curl -X POST http://localhost:3000/api/workflows/create \
  -H "Content-Type: application/json" \
  -d @01-nightly-backup.json
```

**What it does**:
- Connects to production GraphDB instance
- Exports entire repository as BRF (Binary RDF)
- Transfers to backup GraphDB instance
- Creates/overwrites archive repository

### 02-multi-graph-migration.json
**Purpose**: Migrate multiple named graphs in parallel

**Action Type**: `ItemList` containing multiple `TransferAction`s

**Parallelism**: 3 concurrent migrations

**Usage with When**:
```bash
curl -X POST http://localhost:3000/api/workflows/create \
  -H "Content-Type": "application/json" \
  -d @02-multi-graph-migration.json
```

**What it does**:
- Migrates 3 graphs: users, products, orders
- Runs up to 3 migrations in parallel
- Each graph is exported from source and imported to target
- Independent failure handling per graph

## Environment Variables

Before running these workflows, set up your credentials:

```bash
# GraphDB API Key
export GRAPHDB_API_KEY="your-pxgraphservice-api-key"

# Production GraphDB credentials
export PROD_PASSWORD="prod-admin-password"

# Backup GraphDB credentials
export BACKUP_PASSWORD="backup-admin-password"

# Source GraphDB credentials
export SOURCE_PASSWORD="source-admin-password"

# Target GraphDB credentials
export TARGET_PASSWORD="target-admin-password"
```

## Schema.org Types Used

### TransferAction
Represents moving data from source to target:
- **fromLocation**: Source repository (`SoftwareSourceCode`)
- **toLocation**: Target repository (`SoftwareSourceCode`)
- **object**: (Optional) Specific graph to transfer (`Dataset`)

### SoftwareSourceCode
Represents a GraphDB repository:
- **identifier**: Repository name
- **codeRepository**: Full repository URL
- **additionalProperty**: Credentials (username, password, serverUrl)

### Dataset
Represents a named graph (RDF dataset):
- **identifier**: Graph URI
- **encodingFormat**: RDF serialization format
- **includedInDataCatalog**: Parent repository

### Schedule
When to execute the action:
- **repeatFrequency**: ISO 8601 duration (P1D = daily, PT1H = hourly)
- **startTime**: Time of day to run

## Direct Testing (Without When)

You can test pxgraphservice semantic API directly:

```bash
# Test repository migration
curl -X POST http://localhost:8080/v1/api/semantic/action \
  -H "x-api-key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "@context": "https://schema.org",
    "@type": "TransferAction",
    "identifier": "test-migration",
    "fromLocation": {
      "@type": "SoftwareSourceCode",
      "identifier": "source-repo",
      "additionalProperty": {
        "serverUrl": "http://localhost:7200",
        "username": "admin",
        "password": "password"
      }
    },
    "toLocation": {
      "@type": "SoftwareSourceCode",
      "identifier": "target-repo",
      "additionalProperty": {
        "serverUrl": "http://localhost:7200",
        "username": "admin",
        "password": "password"
      }
    }
  }'
```

## Creating Custom Workflows

### Step 1: Choose Action Type
- **TransferAction**: Repository or graph migration
- **CreateAction**: Create new repository
- **DeleteAction**: Delete repository or graph
- **UpdateAction**: Rename repository or graph
- **UploadAction**: Import RDF data

### Step 2: Define Repositories
```json
{
  "@type": "SoftwareSourceCode",
  "identifier": "my-repo",
  "codeRepository": "http://graphdb:7200/repositories/my-repo",
  "additionalProperty": {
    "serverUrl": "http://graphdb:7200",
    "username": "admin",
    "password": "secret"
  }
}
```

### Step 3: Add Schedule (for When integration)
```json
{
  "schedule": {
    "@type": "Schedule",
    "repeatFrequency": "PT4H"
  }
}
```

### Step 4: Wrap in ScheduledAction
```json
{
  "@context": "https://schema.org",
  "@type": "ScheduledAction",
  "identifier": "my-workflow",
  "additionalProperty": {
    "url": "http://localhost:8080/v1/api/semantic/action",
    "httpMethod": "POST",
    "headers": {
      "x-api-key": "${GRAPHDB_API_KEY}"
    },
    "body": {
      // Your action here
    }
  },
  "schedule": {
    "@type": "Schedule",
    "repeatFrequency": "PT4H"
  }
}
```

## Monitoring

View migration logs and status:

**pxgraphservice UI**: http://localhost:8080/admin/migrations
**When UI**: http://localhost:3000/

## Advanced Examples

### Hourly Sync
```json
{
  "schedule": {
    "@type": "Schedule",
    "repeatFrequency": "PT1H"
  }
}
```

### Weekly Backup (Sundays at midnight)
```json
{
  "schedule": {
    "@type": "Schedule",
    "repeatFrequency": "P1W",
    "byDay": ["Sunday"],
    "startTime": "00:00:00"
  }
}
```

### Dependencies (Sequential Execution)
```json
{
  "@type": "ScheduledAction",
  "identifier": "cleanup-task",
  "dependsOn": ["backup-task"]
}
```

## Troubleshooting

**Issue**: Workflow not executing
- Check When UI for task status
- Verify API key in environment variables
- Check pxgraphservice logs

**Issue**: Migration fails
- Check GraphDB connectivity
- Verify repository names exist
- Check credentials in additionalProperty

**Issue**: Schedule not triggering
- Verify ISO 8601 duration format
- Check When daemon is running
- Review When task logs

## References

- [Schema.org Actions](https://schema.org/Action)
- [ISO 8601 Durations](https://en.wikipedia.org/wiki/ISO_8601#Durations)
- [When Documentation](http://localhost:3000/examples)
- [pxgraphservice API](http://localhost:8080/docs)

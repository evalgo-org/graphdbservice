#!/bin/bash
# Example script to import RDF data into a GraphDB repository
# using the semantic JSON-LD API with multipart/form-data

set -e

# Configuration
API_KEY="${API_KEY:-1234567890}"
API_URL="${API_URL:-http://localhost:9991}"
GRAPHDB_URL="${GRAPHDB_URL:-https://kks-dev.ontotext.com}"
REPO_NAME="${REPO_NAME:-Consumption-Navigator-DE-Intern-All-test1-amir12}"
GRAPH_URI="${GRAPH_URI:-http://example.org/graph/imported-data}"
DATA_FILE="${DATA_FILE:-Consumption-Navigator-DE-Intern-All-test1-amir12.brf}"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}GraphDB Data Import Example${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Check if files exist
ACTION_FILE="${SCRIPT_DIR}/08-graph-import-with-data.json"
DATA_PATH="${SCRIPT_DIR}/${DATA_FILE}"

if [ ! -f "$ACTION_FILE" ]; then
    echo -e "${RED}Error: Action file not found: ${ACTION_FILE}${NC}"
    exit 1
fi

if [ ! -f "$DATA_PATH" ]; then
    echo -e "${RED}Error: Data file not found: ${DATA_PATH}${NC}"
    echo -e "${YELLOW}Tip: Set DATA_FILE environment variable to specify a different file${NC}"
    exit 1
fi

echo -e "${YELLOW}Configuration:${NC}"
echo -e "  API URL:      ${API_URL}"
echo -e "  GraphDB URL:  ${GRAPHDB_URL}"
echo -e "  Repository:   ${REPO_NAME}"
echo -e "  Graph URI:    ${GRAPH_URI}"
echo -e "  Action file:  ${ACTION_FILE}"
echo -e "  Data file:    ${DATA_PATH}"
echo ""

# Get file size
FILE_SIZE=$(stat -f%z "$DATA_PATH" 2>/dev/null || stat -c%s "$DATA_PATH" 2>/dev/null)
echo -e "${YELLOW}Data file size: ${FILE_SIZE} bytes${NC}"
echo ""

# Update the JSON file with custom values (create temp file)
TEMP_ACTION_FILE=$(mktemp)
sed -e "s|https://kks-dev.ontotext.com|${GRAPHDB_URL}|g" \
    -e "s|my-new-repository|${REPO_NAME}|g" \
    -e "s|http://example.org/graph/imported-data|${GRAPH_URI}|g" \
    "$ACTION_FILE" > "$TEMP_ACTION_FILE"

echo -e "${YELLOW}Sending import request...${NC}"
echo ""

# Make the API request
# Note: The field name "data" is what the UploadAction expects for data files
RESPONSE=$(curl -X POST "${API_URL}/v1/api/semantic/action" \
  -H "x-api-key: ${API_KEY}" \
  -F "action=@${TEMP_ACTION_FILE}" \
  -F "data=@${DATA_PATH}" \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)

# Extract HTTP status
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
RESPONSE_BODY=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')

# Clean up temp files
rm -f "$TEMP_ACTION_FILE"

echo -e "${YELLOW}Response (HTTP ${HTTP_STATUS}):${NC}"
echo "$RESPONSE_BODY" | jq . 2>/dev/null || echo "$RESPONSE_BODY"
echo ""

# Check if successful
if [ "$HTTP_STATUS" -eq 200 ]; then
    echo -e "${GREEN}✓ Data imported successfully!${NC}"
    echo ""
    echo -e "${YELLOW}Next steps:${NC}"
    echo -e "  1. Query the graph: ${GRAPHDB_URL}/repositories/${REPO_NAME}"
    echo -e "  2. SPARQL endpoint: ${GRAPHDB_URL}/repositories/${REPO_NAME}/statements"
    echo -e "  3. Example SPARQL query:"
    echo -e "     SELECT * FROM <${GRAPH_URI}> WHERE { ?s ?p ?o } LIMIT 10"
    exit 0
else
    echo -e "${RED}✗ Failed to import data${NC}"
    exit 1
fi

#!/bin/bash
# Example script to create a GraphDB repository with Turtle configuration
# using the semantic JSON-LD API with multipart/form-data

set -e

# Configuration
API_KEY="${API_KEY:-1234567890}"
API_URL="${API_URL:-http://localhost:9991}"
GRAPHDB_URL="${GRAPHDB_URL:-https://kks-dev.ontotext.com}"
REPO_NAME="${REPO_NAME:-Consumption-Navigator-DE-Intern-All-test1-amir12}"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}GraphDB Repository Creation Example${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Check if files exist
ACTION_FILE="${SCRIPT_DIR}/07-repo-create-with-config.json"
CONFIG_FILE="${SCRIPT_DIR}/repo-config-example.ttl"

if [ ! -f "$ACTION_FILE" ]; then
    echo -e "${RED}Error: Action file not found: ${ACTION_FILE}${NC}"
    exit 1
fi

if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}Error: Config file not found: ${CONFIG_FILE}${NC}"
    exit 1
fi

echo -e "${YELLOW}Configuration:${NC}"
echo -e "  API URL:      ${API_URL}"
echo -e "  GraphDB URL:  ${GRAPHDB_URL}"
echo -e "  Repository:   ${REPO_NAME}"
echo -e "  Action file:  ${ACTION_FILE}"
echo -e "  Config file:  ${CONFIG_FILE}"
echo ""

# Update the JSON file with custom values (create temp file)
TEMP_ACTION_FILE=$(mktemp)
sed -e "s|http://localhost:7200|${GRAPHDB_URL}|g" \
    -e "s|my-new-repository|${REPO_NAME}|g" \
    "$ACTION_FILE" > "$TEMP_ACTION_FILE"

# Update the TTL file with repository name (create temp file)
TEMP_CONFIG_FILE=$(mktemp)
sed "s|my-new-repository|${REPO_NAME}|g" "$CONFIG_FILE" > "$TEMP_CONFIG_FILE"

echo -e "${YELLOW}Sending request...${NC}"
echo ""

# Make the API request
RESPONSE=$(curl -X POST "${API_URL}/v1/api/semantic/action" \
  -H "x-api-key: ${API_KEY}" \
  -F "action=@${TEMP_ACTION_FILE};type=application/json" \
  -F "config=@${TEMP_CONFIG_FILE}" \
  -w "\nHTTP_STATUS:%{http_code}" \
  -s)

# Extract HTTP status
HTTP_STATUS=$(echo "$RESPONSE" | grep "HTTP_STATUS:" | cut -d: -f2)
RESPONSE_BODY=$(echo "$RESPONSE" | sed '/HTTP_STATUS:/d')

# Clean up temp files
rm -f "$TEMP_ACTION_FILE" "$TEMP_CONFIG_FILE"

echo -e "${YELLOW}Response (HTTP ${HTTP_STATUS}):${NC}"
echo "$RESPONSE_BODY" | jq . 2>/dev/null || echo "$RESPONSE_BODY"
echo ""

# Check if successful
if [ "$HTTP_STATUS" -eq 200 ]; then
    echo -e "${GREEN}✓ Repository created successfully!${NC}"
    echo ""
    echo -e "${YELLOW}Next steps:${NC}"
    echo -e "  1. Access repository: ${GRAPHDB_URL}/repository/${REPO_NAME}"
    echo -e "  2. Import data using graph-import"
    echo -e "  3. Query via SPARQL endpoint: ${GRAPHDB_URL}/repositories/${REPO_NAME}"
    exit 0
else
    echo -e "${RED}✗ Failed to create repository${NC}"
    exit 1
fi

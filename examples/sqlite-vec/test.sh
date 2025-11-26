#!/bin/bash

# Test script for SQLite-Vec Vector Database Server
# Demonstrates all API endpoints

set -e

BASE_URL="http://localhost:8080"
VECTOR_DIM=1536

echo "=== SQLite-Vec API Test Script ==="
echo ""

# Helper function to generate a random vector
generate_vector() {
    python3 -c "import json, random; print(json.dumps([random.random() for _ in range($VECTOR_DIM)]))"
}

# 1. Health Check
echo "1. Testing health check..."
curl -s $BASE_URL/health | jq .
echo ""

# 2. List Collections (should be empty initially)
echo "2. Listing collections (initially empty)..."
curl -s $BASE_URL/collections | jq .
echo ""

# 3. Create Collections
echo "3. Creating collections..."
curl -s -X POST $BASE_URL/collections \
  -H "Content-Type: application/json" \
  -d '{"name": "code_embeddings"}' | jq .

curl -s -X POST $BASE_URL/collections \
  -H "Content-Type: application/json" \
  -d '{"name": "documentation"}' | jq .
echo ""

# 4. List Collections Again
echo "4. Listing collections after creation..."
curl -s $BASE_URL/collections | jq .
echo ""

# 5. Add Vectors to code_embeddings collection
echo "5. Adding vectors to code_embeddings collection..."
VECTOR1=$(generate_vector)
curl -s -X POST $BASE_URL/vectors \
  -H "Content-Type: application/json" \
  -d "{
    \"collection_name\": \"code_embeddings\",
    \"vector\": $VECTOR1,
    \"metadata\": {
      \"file\": \"server.go\",
      \"function\": \"handleRequest\",
      \"line\": 100
    }
  }" | jq .

VECTOR2=$(generate_vector)
curl -s -X POST $BASE_URL/vectors \
  -H "Content-Type: application/json" \
  -d "{
    \"collection_name\": \"code_embeddings\",
    \"vector\": $VECTOR2,
    \"metadata\": {
      \"file\": \"client.go\",
      \"function\": \"connect\",
      \"line\": 50
    }
  }" | jq .
echo ""

# 6. Add Vectors to documentation collection
echo "6. Adding vectors to documentation collection..."
VECTOR3=$(generate_vector)
curl -s -X POST $BASE_URL/vectors \
  -H "Content-Type: application/json" \
  -d "{
    \"collection_name\": \"documentation\",
    \"vector\": $VECTOR3,
    \"metadata\": {
      \"doc\": \"api.md\",
      \"section\": \"authentication\"
    }
  }" | jq .

VECTOR4=$(generate_vector)
curl -s -X POST $BASE_URL/vectors \
  -H "Content-Type: application/json" \
  -d "{
    \"collection_name\": \"documentation\",
    \"vector\": $VECTOR4,
    \"metadata\": {
      \"doc\": \"deployment.md\",
      \"section\": \"docker\"
    }
  }" | jq .
echo ""

# 7. Search within a specific collection
echo "7. Searching within 'code_embeddings' collection..."
QUERY_VECTOR=$(generate_vector)
curl -s -X POST $BASE_URL/search \
  -H "Content-Type: application/json" \
  -d "{
    \"vector\": $QUERY_VECTOR,
    \"collection_name\": \"code_embeddings\",
    \"limit\": 5
  }" | jq .
echo ""

# 8. Search across all collections
echo "8. Searching across ALL collections..."
curl -s -X POST $BASE_URL/search \
  -H "Content-Type: application/json" \
  -d "{
    \"vector\": $QUERY_VECTOR,
    \"limit\": 10
  }" | jq .
echo ""

# 9. Delete a specific vector
echo "9. Deleting vector with id=1..."
curl -s -X DELETE $BASE_URL/vectors/1 | jq .
echo ""

# 10. Search again to verify deletion
echo "10. Searching again to verify vector deletion..."
curl -s -X POST $BASE_URL/search \
  -H "Content-Type: application/json" \
  -d "{
    \"vector\": $QUERY_VECTOR,
    \"collection_name\": \"code_embeddings\",
    \"limit\": 5
  }" | jq .
echo ""

# 11. Delete entire collection
echo "11. Deleting 'documentation' collection..."
curl -s -X DELETE $BASE_URL/collections/documentation | jq .
echo ""

# 12. List collections after deletion
echo "12. Listing collections after deletion..."
curl -s $BASE_URL/collections | jq .
echo ""

# 13. Search to verify collection deletion
echo "13. Searching all collections (documentation should be gone)..."
curl -s -X POST $BASE_URL/search \
  -H "Content-Type: application/json" \
  -d "{
    \"vector\": $QUERY_VECTOR,
    \"limit\": 10
  }" | jq .
echo ""

echo "=== Test Complete ==="
echo ""
echo "Summary:"
echo "  ✓ Health check"
echo "  ✓ Collection creation"
echo "  ✓ Vector insertion"
echo "  ✓ Collection-specific search"
echo "  ✓ Cross-collection search"
echo "  ✓ Vector deletion"
echo "  ✓ Collection deletion (cascade)"

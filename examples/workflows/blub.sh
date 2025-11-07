curl -X PUT \
    "http://localhost:8080/graphdb?graph=http://example.org/graph/products" \
    -H "Content-Type: application/rdf+xml" \
    -H "X-Process-ID: proc-123" \
    -H "X-Process-Name: product-import" \
    --data-binary @products.rdf

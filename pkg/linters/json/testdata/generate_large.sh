#\!/bin/bash
# Generate a large JSON file > 1MB for testing size limits

echo '{"large_data": [' > large.json

for i in {1..20000}; do
    echo "  {\"id\": $i, \"data\": \"$(printf '%*s' 50 '' | tr ' ' 'x')\", \"timestamp\": \"2025-01-15T10:30:00Z\", \"extra\": \"padding data to increase size\"}," >> large.json
done

# Remove last comma and close the JSON
sed -i '' '$ s/,$//' large.json
echo ']' >> large.json
echo '}' >> large.json

echo "Generated large.json ($(wc -c < large.json) bytes)"

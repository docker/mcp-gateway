#!/bin/bash

# Read JSON from stdin into a variable
json_input=$(cat)

# Write the JSON input to whatever.txt
echo "$json_input" > whatever.txt

# Extract the response property and serialize it back to JSON
echo "$json_input" | jq -r '.response | tostring'

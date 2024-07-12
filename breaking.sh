#!/bin/bash

# Fetch the latest main branch
# Check for breaking changes
echo "Checking for breaking changes..."
CHANGED_FILES=$(git diff --name-only origin/main...HEAD)
echo "Changed files: $CHANGED_FILES"

FILE_CHANGED=false
STRUCT_CHANGED=false

for file in $CHANGED_FILES; do
  if echo $file | grep -qE 'nodebuilder/.*/config\.go'; then
    FILE_CHANGED=true
  fi
  if echo $file | grep -qE '\.proto$'; then
    FILE_CHANGED=true
  fi
  if echo $file | grep -qE 'nodebuilder/.*/config\.go'; then
    DIFF_OUTPUT=$(git diff origin/main...HEAD $file)
    if echo "$DIFF_OUTPUT" | grep -qE 'type Config struct|^\s+\w+\s+Config'; then
      STRUCT_CHANGED=true
    fi
  fi
done

if [ "$FILE_CHANGED" = true ] || [ "$STRUCT_CHANGED" = true ]; then
  echo "Breaking changes detected."
  exit 1
else
  echo "No breaking changes detected."
  exit 0
fi

#!/bin/bash

# Check if BASE_DIR is provided as an argument
if [[ -z "$1" ]]; then
  echo "Usage: $0 <BASE_DIR>"
  exit 1
fi

# Assign the first argument to BASE_DIR
BASE_DIR="$1"

# Make sure the blocks directory exists
BLOCKS_DIR="$BASE_DIR/blocks"
if [[ ! -d "$BLOCKS_DIR" ]]; then
  echo "Blocks directory not found: $BLOCKS_DIR"
  exit 1
fi

# Loop over all symlinks in the blocks directory
find "$BLOCKS_DIR" -type l | while read -r symlink; do
  # Get the absolute path of the target file
  target=$(readlink "$symlink")

  # Skip if it's a hard link or if the symlink is already relative
  if [[ "$target" == /* ]]; then
    # Extract the filename from the target path
    filename=$(basename "$target")

    # Calculate the relative path from the symlink's directory
    relpath="../$filename"

    # Update the symlink to use the relative path
    ln -sf "$relpath" "$symlink"

    echo "Updated symlink: $symlink -> $relpath"
  else
    echo "Skipping relative symlink: $symlink"
  fi
done

#!/bin/bash

# Script to remove trailing whitespace from modified files in git repo
# Usage: ./remove_trailing_whitespace.sh

set -e

echo "Removing trailing whitespace from modified files..."

# Get list of modified files (staged and unstaged)
modified_files=$(git diff --name-only HEAD 2>/dev/null || true)
staged_files=$(git diff --cached --name-only 2>/dev/null || true)

# Combine and remove duplicates
all_modified_files=$(echo -e "$modified_files\n$staged_files" | sort -u | grep -v '^$' || true)

if [ -z "$all_modified_files" ]; then
    echo "No modified files found."
    exit 0
fi

echo "Found modified files:"
echo "$all_modified_files"
echo ""

# Counter for processed files
count=0

# Process each modified file
for file in $all_modified_files; do
    # Check if file exists and is a text file
    if [ -f "$file" ] && [ -r "$file" ]; then
        # Skip binary files
        if file "$file" | grep -q "text\|ASCII\|UTF-8\|empty"; then
            echo "Processing: $file"
            
            # Create backup
            cp "$file" "$file.backup"
            
            # Remove trailing whitespace
            # Use sed to remove trailing spaces and tabs from each line
            sed -i 's/[[:space:]]*$//' "$file"
            
            # Check if file actually changed
            if ! cmp -s "$file" "$file.backup"; then
                echo "  ✓ Removed trailing whitespace from $file"
                count=$((count + 1))
            else
                echo "  - No trailing whitespace found in $file"
            fi
            
            # Remove backup
            rm "$file.backup"
        else
            echo "Skipping binary file: $file"
        fi
    else
        echo "Skipping non-existent or unreadable file: $file"
    fi
done

echo ""
echo "Summary: Processed $count files with trailing whitespace removed."

if [ $count -gt 0 ]; then
    echo ""
    echo "Files have been modified. You may want to:"
    echo "  git add -u    # Stage the changes"
    echo "  git commit    # Commit the whitespace fixes"
fi
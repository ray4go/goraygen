#!/bin/bash

set -e

# Check if go_version argument is provided
if [ -z "$1" ]; then
    echo "Usage: $0 <go_version>"
    echo "Example: $0 1.21"
    exit 1
fi

go_version="$1"
branch_name="go${go_version}"

# 0. Switch to or create the release branch
echo "Checking out branch: ${branch_name}"
if git rev-parse --verify "${branch_name}" >/dev/null 2>&1; then
    # Branch exists, checkout and reset onto master
    git checkout "${branch_name}"
    git reset --hard master
else
    # Branch doesn't exist, create and checkout
    git checkout -b "${branch_name}"
fi

# 1. Create go.mod file
echo "Creating go.mod file..."
cat > go.mod << EOF
module github.com/ray4go/go-ray/goraygen

go ${go_version}
EOF

# 2. Run go mod tidy
echo "Running go mod tidy..."
go mod tidy

# 3. Remove toolchain directive from go.mod
echo "Removing toolchain directive..."
sed -i.bak '/^toolchain/d' go.mod && rm go.mod.bak

# 4. Output the final go.mod content
echo "Final go.mod content:"
cat go.mod

# 5. Commit
echo "Committing changes..."
git add go.mod
git commit -m "Update go.mod for Go ${go_version}"

# 6. Force push to remote
echo "Force pushing to remote..."
git push -f origin "${branch_name}"

git checkout -

echo "Done!"
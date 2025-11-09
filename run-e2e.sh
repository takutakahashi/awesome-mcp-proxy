#!/bin/bash

# Wrapper script to run e2e tests with mise environment activated

# Activate mise to ensure tools like jq are available
if command -v mise &> /dev/null; then
    eval "$(mise activate bash)"
fi

# Run the actual e2e tests
exec ./test/e2e.sh "$@"
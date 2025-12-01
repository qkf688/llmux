#!/bin/bash
echo "Building webui..."
cd webui && pnpm run build && cd .. && echo "Starting server..." && go run main.go
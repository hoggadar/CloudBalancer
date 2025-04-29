#!/bin/bash

GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

send_request() {
    local response
    response=$(curl -s "http://localhost:8080/test")
    local server_msg=$(echo "$response" | grep "Server Message:" | cut -d':' -f2-)
    echo -e "${BLUE}Request $1:${NC} ${GREEN}$server_msg${NC}"
}

echo "Starting load balancing test..."
echo "Sending 25 requests to load balancer (localhost:8080)..."
echo "================================================"

for i in {1..25}; do
    send_request $i
    sleep 0.5
done

echo "================================================"
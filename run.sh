#!/bin/bash
cd "$(dirname "$0")"

export ANTHROPIC_AUTH_TOKEN="sk-ybU6kNS8J9U7sIh7ArIaXA"
export ANTHROPIC_MODEL="claude-sonnet-4-5-20250929"
export ANTHROPIC_BASE_URL="http://llm.moontontech.net"

./stock.exe

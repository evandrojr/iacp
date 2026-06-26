#!/bin/bash
echo "Dummy editor started, waiting 3 seconds..."
sleep 3
echo "feat: test message from dummy editor" > "$1"
echo "Dummy editor finished."

#!/bin/bash

curl -v -X 'POST' \
  'http://localhost:8080/api/v3/worker/register-worker' \
  -H 'accept: application/json' \
  -H 'Content-Type: application/json' \
  -d '{
  "name": "Test Worker",
  "secret": "abcdefg",
  "platform": "Linux",
  "supported_task_types": ["blender", "ffmpeg", "file-management", "misc"]
}'

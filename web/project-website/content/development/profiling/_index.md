---
title: Profiling Flamenco Manager
weight: 50
---

Flamenco Manager has built-in profiling support. To get a call graph with timing information, follow these steps:

1. Run `flamenco-manager -pprof` to enable its profiler HTTP endpoint.
2. Run `go tool pprof -http localhost:8082 'http://localhost:8080/debug/pprof/profile?seconds=60'`.
3. Do whatever you want to profile with Flamenco.
4. The tool will open a browser to show the call graph when it's done gathering the info.

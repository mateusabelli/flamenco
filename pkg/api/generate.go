// SPDX-License-Identifier: GPL-3.0-or-later
package api

//go:generate go tool oapi-codegen -config oapi-codegen.yaml -generate types  -o openapi_types.gen.go  flamenco-openapi.yaml
//go:generate go tool oapi-codegen -config oapi-codegen.yaml -generate server -o openapi_server.gen.go flamenco-openapi.yaml
//go:generate go tool oapi-codegen -config oapi-codegen.yaml -generate spec   -o openapi_spec.gen.go   flamenco-openapi.yaml
//go:generate go tool oapi-codegen -config oapi-codegen.yaml -generate client -o openapi_client.gen.go flamenco-openapi.yaml

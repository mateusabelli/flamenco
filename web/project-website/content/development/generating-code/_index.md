---
title: Generating Code
weight: 20
---

Some code (Go, Python, JavaScript) is generated from the OpenAPI specs in
`pkg/api/flamenco-openapi.yaml`. There are also Go files generated to create
mock implementations of interfaces for unit testing purposes.

## Installing the Code Generators

There are three code generators used by Flamenco:

- [`oapi-codegen`][oapi-codegen] for the OpenAPI server & client in Go.
- [`mockgen`][mockgen] for generating mocks for tests in Go.
- [`openapi-codegen`][openapi-codegen] for the OpenAPI clients in Python and JavaScript.

[oapi-codegen]: https://github.com/oapi-codegen/oapi-codegen
[mockgen]: https://github.com/uber-go/mock/
[openapi-codegen]: https://openapi-generator.tech/

### Python and JavaScript code generators


`openapi-codegen` is bundled with the Flamenco sources, but does need a Java
runtime environment to be installed.

{{< tabs "installing-java" >}}
{{< tab "Linux" >}}

On Ubuntu Linux this should be enough:

```bash
$ sudo apt install default-jre-headless
```

Other Linux distributions very likely have a similar package.

{{< /tab >}}
{{< tab "Windows" >}}

Use the [official Java installer](https://www.java.com/en/download/manual.jsp).

{{< /tab >}}
{{< tab "macOS" >}}

**Option 1** (Native install)

Use the [official Java installer](https://www.java.com/en/download/manual.jsp).

<br />

**Option 2** (Homebrew)

Install Java via homebrew:

```
brew install java
```

Note that this requires XCode to be installed.

{{< /tab >}}
{{< /tabs >}}


## Running the Code Generators

All three generators can be run with the following command:

```bash
$ go run mage.go generate
```

Individual runs are also available:

```bash
# Generate Go code
$ go run mage.go generateGo

# Generate JavaScript code
$ go run mage.go generateJs

# Generate Python code
$ go run mage.go generatePy
```


## Committing Generated Code

**Generated code is committed to Git**, so that after a checkout you shouldn't
need to re-run the generator to build Flamenco.

The following files & directories are generated. Generated directories are
completely erased before regeneration, so do not add any files there manually.

- `addon/flamenco/manager/`: Python API for the Blender add-on.
- `pkg/api/*.gen.go`: Go API shared by Manager and Worker.
- `internal/**/mocks/*.gen.go`: Generated mocks for Go unit tests.
- `web/app/src/manager-api/`: JavaScript API for the web front-end.

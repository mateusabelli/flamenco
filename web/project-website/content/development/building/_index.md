---
title: Building Flamenco
weight: 10
---

For the steps towards your first build, see [Getting Started][start].

[start]: {{< relref "../getting-started ">}}

## Building with Magefile

The Flamenco build tool is made in Go, using [Magefile][mage].

[mage]: https://magefile.org/

### Basic Builds

```sh
$ go run mage.go build
```

This builds Flamenco Manager, including its webapp, and Flamenco Worker.

### Listing Build Targets

```
$ go run mage.go -l
```

Will list these targets:

| Target                       | Description                                                                                          |
|------------------------------|------------------------------------------------------------------------------------------------------|
| build                        | Flamenco Manager and Flamenco Worker, including the webapp and the add-on                            |
| check                        | Run unit tests, check for vulnerabilities, and run the linter                                        |
| clean                        | Remove executables and other build output                                                            |
| devServerWebapp              |                                                                                                      |
| flamencoManager              | Build Flamenco Manager with the webapp and add-on ZIP embedded                                       |
| flamencoManagerRace          | Build the Flamenco Manager executable with race condition checker enabled, do not rebuild the webapp |
| flamencoManagerWithoutWebapp | Only build the Flamenco Manager executable, do not rebuild the webapp                                |
| flamencoWorker               | Build the Flamenco Worker executable                                                                 |
| format                       | Run `gofmt`, formatting all the source code.                                                         |
| formatCheck                  | Run `gofmt` on all the source code, reporting all differences.                                       |
| generate                     | code (OpenAPI and test mocks)                                                                        |
| generateGo                   | Generate Go code for Flamenco Manager and Worker                                                     |
| generateJS                   | Generate JavaScript code for the webapp                                                              |
| generatePy                   | Generate Python code for the add-on                                                                  |
| govulncheck                  | Check for known vulnerabilities.                                                                     |
| installDeps                  | Install build-time dependencies: code generators and NodeJS dependencies.                            |
| installDepsWebapp            | Use Yarn to install the webapp's NodeJS dependencies                                                 |
| installGenerators            | Install code generators.                                                                             |
| staticcheck                  | Analyse the source code.                                                                             |
| test                         | Run unit tests                                                                                       |
| version                      | Show which version information would be embedded in executables                                      |
| vet                          | Run `go vet`                                                                                         |
| webappStatic                 | Build the webapp as static files that can be served                                                  |


### Faster Re-builds

The above commands first build the build tool itself, and then run it. The build
tool can also be compiled once, and then used as any other program:

```sh
$ go run mage.go -compile mage
$ ./magefiles/mage build
```

### Building with `make`

`make` can be used as a convenient front-end for the Magefile build tool. A few
targets are available with `make` only. These are mostly for release-related
functionality like updating the Flamenco version, or building release packages.

These are the main `make` targets:

| Target                            | Description                                                                                                                                             |
|-----------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------|
| `application`                     | Builds Flamenco Manager, Worker, and the development version of the webapp. This is the default target when just running `make`                         |
| `flamenco-manager`                | Builds just Flamenco Manager. This includes packing the webapp and the Blender add-on into the executable.                                              |
| `flamenco-worker`                 | Builds just Flamenco Worker.                                                                                                                            |
| `flamenco-manager-without-webapp` | Builds Flamenco Manager without rebuilding the webapp. This is useful to speed up the build when you're using the webapp development server (see below) |
| `devserver-website`               | Run the website locally, in a development server that monitors for file changes and auto-refreshes your browser.                                        |
| `devserver-webapp`                | Run the Manager webapp locally in a development server.                                                                                                 |
| `generate`                        | Generate the Go, Python, and JavaScript code.                                                                                                           |
| `generate-go`                     | Generate the Go code, which includes OpenAPI code, as well as mocks for the unit tests.                                                                 |
| `generate-py`                     | Generate the Python code, containing the OpenAPI client code for the Blender add-on.                                                                    |
| `generate-js`                     | Generate the JavaScript code, containing the OpenAPI client code for the web interface.                                                                 |
| `test`                            | Run the unit tests.                                                                                                                                     |
| `check`                           | Run various checks on the Go code. This includes `go vet` and checks for known vulnerabilities.                                                         |
| `clean`                           | Remove build-time files.                                                                                                                                |
| `version`                         | Print some version numbers, mostly for debugging the Makefile itself.                                                                                   |
| `format`                          | Run the auto-formatter on all Go code.                                                                                                                  |
| `format-check`                    | Check that the Go source code is formatted correctly.                                                                                                   |
| `list-embedded`                   | List the files embedded into the `flamenco-manager` executable.                                                                                         |
| `tools`                           | Download FFmpeg for all supported platforms. Can be suffixed by `-linux`, `-windows`, or `-darwin` to only download for specific platforms.             |
| `update-version`                  | Takes the `VERSION` and `RELEASE_CYCLE` declared at the top of the `Makefile` and uses that to update various source files.                             |
| `release-package`                 | Builds release packages for all supported platforms. Can be suffixed by `-linux`, `-windows`, or `-darwin` to only build specific platforms.            |
| `db-migrate-status`               | Database migration: show the current version of the database schema.                                                                                    |
| `db-migrate-up`                   | Database migration: perform one migration step towards the latest version.                                                                              |
| `db-migrate-down`                 | Database migration: roll back one migration step, so go to an older version. This may not be lossless.                                                  |

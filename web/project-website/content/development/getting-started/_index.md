---
title: Getting Started
weight: 1
---

To start, get a **Git checkout** with either of these commands. The 1st one is for
public, read-only access. The 2nd one can be used if you have commit rights to
the project.

```
git clone https://projects.blender.org/studio/flamenco.git
git clone git@projects.blender.org:studio/flamenco.git
```

Then follow the steps below to get everything up & running.

## 1. Installing Go

Most of Flamenco is made in Go.

1. Install [the latest Go release](https://go.dev/). If you want to know specifically which version in required, check the
   [go.mod](https://projects.blender.org/studio/flamenco/src/branch/main/go.mod) file.
2. Optional: set the environment variable `GOPATH` to where you want Go to put its packages. Go will use `$HOME/go` by default.
3. Ensure `$GOPATH/bin` is included in your `$PATH` environment variable. Run `go env GOPATH` if you're not sure what path to use.

## 2. Installing NodeJS

The web UI is built with [Vue.js](https://vuejs.org/), and Socket.IO for
communication with the backend. **NodeJS+Yarn** is used to collect all of those
and build the frontend files.

{{< tabs "installing-nodejs" >}}
{{< tab "Linux" >}}
It's recommended to install Node via Snap:

```
sudo snap install node --classic --channel=18
```

If you install NodeJS in a different way, it may not be bundled with Yarn. In that case, run:

```
sudo npm install --global yarn
```

{{< /tab >}}
{{< tab "Windows" >}}
Install [Node v18 LTS](https://nodejs.org/en/download/). Be sure to enable the "Automatically install the necessary tools" checkbox.

Then install Yarn via:

```
npm install --global yarn
```

{{< /tab >}}
{{< tab "macOS" >}}
**Option 1** (Native install)

Install [Node v18 LTS](https://nodejs.org/en/download/) and then install Yarn via:

```
npm install --global yarn
```

<br />

**Option 2** (Homebrew)

Install Node 18 via homebrew:

```
brew install node@18
```

Then install yarn:

```
brew install yarn
```

{{< /tab >}}
{{< /tabs >}}

## 3. Utilities

Building Flamenco requires only a few tools to be installed on your system.


{{< tabs "installing-utils" >}}
{{< tab "Linux" >}}
On Linux only `make` is necessary, which can be installed via your package manager.

On Debian, and relatives like Ubuntu, run:

```
sudo apt install make
```
{{< /tab >}}
{{< tab "Windows" >}}
Install [MingW W64][mingw]. If in doubt which version to get, grab the `x86_64...seh` one.
You'll need [7Zip][7zip] to extract it.

[mingw]: https://github.com/niXman/mingw-builds-binaries/releases
[7zip]: https://www.7-zip.org/download.html
{{< /tab >}}
{{< tab "macOS" >}}
TODO: write this documentation.
{{< /tab >}}
{{< /tabs >}}

## 4. Your First Build

Run `make with-deps` to install build-time dependencies and build the application.
Subsequent builds can just run `make` without arguments.

You should now have two executables: `flamenco-manager` and `flamenco-worker`.
Both can be run with the `-help` CLI argument to see the available options.

See [building][building] for more `make` targets, for example to run unit tests,
enable the race condition checker, and all other kinds of useful things.

[building]: {{< relref "../building/" >}}

## 5. Get Involved

If you're interested in helping out with Flamenco development, please read [Get Involved][get-involved]!

If you need to change or add any database queries, read through the [database section][database].

[get-involved]: {{<ref "development/get-involved" >}}
[database]: {{<ref "development/database" >}}


## Software Design

The Flamenco software follows an **API-first** approach. All the functionality
of Flamenco Manager is exposed via [the OpenAPI interface][openapi] ([more
info](openapi-info)). The web interface is no exception; anything you can do
with the web interface, you can do with any other OpenAPI client.

- The API can be browsed by following the 'API' link in the top-right corner of
  the Flamenco Manager web interface. That's a link to
  `http://your.manager.address/api/v3/swagger-ui/`
- The web interface, Flamenco Worker, and the Blender add-on are all using that
  same API.

[openapi]: https://projects.blender.org/studio/flamenco/src/branch/main/pkg/api/flamenco-openapi.yaml
[openapi-info]: https://www.openapis.org/

## New Features

To add a new feature to Flamenco, these steps are recommended:

1. Define which changes to the API are necessary, and update the [flamenco-openapi.yaml][openapi] file for this.
1. Run `go generate ./pkg/...` to generate the OpenAPI Go code.
1. Implement any new operations in a minimal way, so that the code compiles (but doesn't do anything else).
1. Run `make generate` to regenerate all the code (so also the JavaScript and Python client, and Go mocks).
1. Write unit tests that test the new functionality.
1. Write the code necessary to make the unit tests pass.
1. Now that you know how it can work, refactor to clean it up.
1. Send in a pull request!

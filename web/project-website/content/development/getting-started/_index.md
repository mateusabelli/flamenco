---
title: Getting Started
weight: 1
aliases:
  - /devstart
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
sudo snap install node --classic --channel=22
```

If you install NodeJS in a different way, it may not be bundled with Yarn. In that case, run:

```
sudo npm install --global yarn
```

{{< /tab >}}
{{< tab "Windows" >}}
Install [Node v22 LTS](https://nodejs.org/en/download/). Be sure to enable the "Automatically install the necessary tools" checkbox.

Then install Yarn via:

```
npm install --global yarn
```

{{< /tab >}}
{{< tab "macOS" >}}
**Option 1** (Native install)

Install [Node v22 LTS](https://nodejs.org/en/download/) and then install Yarn via:

```
npm install --global yarn
```

<br />

**Option 2** (Homebrew)

Install Node 22 via homebrew:

```
brew install node@22
brew link node@22
```

Then install yarn:

```
brew install yarn
```

{{< /tab >}}
{{< /tabs >}}

## 3. Your First Build

Run `go run mage.go installDeps` to install build-time dependencies. This is
only necessary the first time you build Flamenco (or when these dependencies are
upgraded, which is rare)

Build the application with `go run mage.go build`.

You should now have two executables: `flamenco-manager` and `flamenco-worker`.
Both can be run with the `-help` CLI argument to see the available options.

See [building][building] for more `mage` targets, for example to run unit tests,
enable the race condition checker, and ways to speed up the build process.

[building]: {{< relref "../building/" >}}

## 4. Get Involved

If you're interested in helping out with Flamenco development, please read [Get Involved][get-involved]!

[Blender's guidelines on contributing code][contributing] also applies to
Flamenco. Be sure to give it a read-through, as it has useful information and
will make the whole process of getting your changes into Flamenco a more
pleasant one.

If you need to change or add any database queries, read through the [database section][database].

[get-involved]: {{<ref "development/get-involved" >}}
[database]: {{<ref "development/database" >}}
[contributing]: https://developer.blender.org/docs/handbook/contributing/


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

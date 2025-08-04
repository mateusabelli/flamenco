-include .env

PKG := projects.blender.org/studio/flamenco

# To update the version number in all the relevant places, update the VERSION
# and RELEASE_CYCLE variables below and run `make update-version`.
VERSION := 3.8-alpha1
# "alpha", "beta", or "release".
RELEASE_CYCLE := alpha

# _GIT_DESCRIPTION_OR_TAG is either something like '16-123abc' (when we're 16
# commits since the last tag) or it's something like `v3.0-beta2` (when exactly
# on a tagged version).
_GIT_DESCRIPTION_OR_TAG := $(subst v${VERSION}-,,$(shell git describe --tag --dirty --always --abbrev=9))
# In the above cases, GITHASH is either `16-123abc` (in the same case above) or
# `123abc` (when the tag matches the current commit exactly) or `dirty` (when
# the tag matches the current commit exactly, and there are subsequent
# uncommitted changes). This is done to prevent repetition of the same tag
# in the "extended version" of Flamenco, which combines ${VERSION} and
# ${GITHASH}.
GITHASH := $(subst v${VERSION},$(shell git rev-parse --short=9 HEAD),${_GIT_DESCRIPTION_OR_TAG})
ifeq (${GITHASH},dirty)
GITHASH := $(shell git rev-parse --short=9 HEAD)
endif

BUILDTOOL := mage
ifeq ($(OS),Windows_NT)
	BUILDTOOL := $(BUILDTOOL).exe
endif
BUILDTOOL_PATH := ${PWD}/${BUILDTOOL}

# Package name of the generated Python/JavaScript code for the Flamenco API.
PY_API_PKG_NAME=flamenco.manager
JS_API_PKG_NAME=manager

# The directory that will contain the built webapp files, and some other files
# that will be served as static files by the Flamenco Manager web server.
#
# WARNING: THIS IS USED IN `rm -rf ${WEB_STATIC}`, DO NOT MAKE EMPTY OR SET TO
# ANY ABSOLUTE PATH.
WEB_STATIC=web/static

# The Hugo package + its version.
HUGO_PKG := github.com/gohugoio/hugo@v0.121.2

# Prevent any dependency that requires a C compiler, i.e. only work with pure-Go libraries.
export CGO_ENABLED=0

all: application

# Install generators and build the software.
with-deps: buildtool
	"${BUILDTOOL_PATH}" installGenerators
	$(MAKE) application

vet: buildtool
	"${BUILDTOOL_PATH}" vet

check: buildtool
	"${BUILDTOOL_PATH}" check

application: flamenco-manager flamenco-worker

flamenco-manager: buildtool
	"${BUILDTOOL_PATH}" flamencoManager

flamenco-manager-without-webapp: buildtool
	"${BUILDTOOL_PATH}" flamencoManagerWithoutWebapp

flamenco-worker: buildtool
	"${BUILDTOOL_PATH}" flamencoWorker

# Builds the buildtool itself, for faster rebuilds of Skyfill.
buildtool: ${BUILDTOOL}
${BUILDTOOL}: mage.go $(wildcard magefiles/*.go) go.mod
	@echo "Building build tool $@"
	@go run mage.go -compile "${BUILDTOOL_PATH}"

# NOTE: these database migration commands are just for reference / debugging /
# development purposes. Flamenco Manager and Worker each perform their own
# migration at startup. In normal use, you'll never need those commands. This is
# also why the `with-deps` target doesn't install the Goose CLI program.
#
# Run `go install github.com/pressly/goose/v3/cmd/goose@latest` to install.
db-migrate-status:
	goose -dir ./internal/manager/persistence/migrations/ sqlite3 flamenco-manager.sqlite status
db-migrate-up:
	goose -dir ./internal/manager/persistence/migrations/ sqlite3 flamenco-manager.sqlite up
db-migrate-down:
	goose -dir ./internal/manager/persistence/migrations/ sqlite3 flamenco-manager.sqlite down
.PHONY: db-migrate-status db-migrate-up db-migrate-down

webapp-static: buildtool
	"${BUILDTOOL_PATH}" webappStatic

install-generators: buildtool
	"${BUILDTOOL_PATH}" installGenerators

generate: buildtool
	"${BUILDTOOL_PATH}" generate

generate-go: buildtool
	"${BUILDTOOL_PATH}" generateGo

generate-py: buildtool
	"${BUILDTOOL_PATH}" generatePy

generate-js: buildtool
	"${BUILDTOOL_PATH}" generateJS

.PHONY:
update-version:
	@echo "--- Updating Flamenco version to ${VERSION}"
	@echo "--- If this stops with exit status 42, it was already at that version."
	@echo
	go run ./cmd/update-version ${VERSION} ${RELEASE_CYCLE}
	$(MAKE) generate-py
	$(MAKE) generate-js
	@echo
	@echo 'File replacement done, commit with:'
	@echo
	@echo git commit -m '"Bumped version to ${VERSION}"' Makefile \
		addon/flamenco/__init__.py \
		addon/flamenco/manager \
		addon/flamenco/manager_README.md \
		magefiles/version.go \
		web/app/src/manager-api \
		web/project-website/data/flamenco.yaml
	@echo 'git tag -a -m "Tagged version ${VERSION}" v${VERSION}'

version:
	@echo "Package     : ${PKG}"
	@echo "Version     : ${VERSION}"
	@echo "Git Hash    : ${GITHASH}"
	@echo -n "GOOS        : "; go env GOOS
	@echo -n "GOARCH      : "; go env GOARCH
	@echo
	@env | grep GO

list-embedded:
	@go list -f '{{printf "%10s" .Name}}: {{.EmbedFiles}}' ${PKG}/...

swagger-ui:
	git clone --depth 1 https://github.com/swagger-api/swagger-ui.git tmp-swagger-ui
	rm -rf pkg/api/static/swagger-ui
	mv tmp-swagger-ui/dist pkg/api/static/swagger-ui
	rm -rf tmp-swagger-ui
	@echo
	@echo 'Now update pkg/api/static/swagger-ui/index.html to have url: "/api/openapi3.json",'

test: buildtool
	"${BUILDTOOL_PATH}" test

clean: buildtool
	"${BUILDTOOL_PATH}" clean

devserver-website:
	go run ${HUGO_PKG} -s web/project-website serve

devserver-webapp: buildtool
	"${BUILDTOOL_PATH}" devServerWebapp

format: buildtool
	"${BUILDTOOL_PATH}" format

format-check: buildtool
	"${BUILDTOOL_PATH}" formatCheck

deploy-website:
	$(MAKE) -s check-environment
	rm -rf web/project-website/public/
	go run ${HUGO_PKG} -s web/project-website --baseURL https://flamenco.blender.org/
	rsync web/project-website/public/ ${WEBSERVER_SSH}:${WEBSERVER_ROOT}/ \
		-e "ssh -p ${WEBSERVER_SSH_PORT}" \
		-rl \
		--exclude v2/ \
		--exclude downloads/ \
		--exclude .well-known/ \
		--exclude .htaccess \
		--delete-after

# For production deployments: check variables stored in .env
.PHONY: check-environment
check-environment:
ifndef WEBSERVER_SSH
	@echo "WEBSERVER_SSH not found. Check .env or .env.example"
	exit 1
endif
ifndef WEBSERVER_ROOT
	@echo "WEBSERVER_ROOT not found. Check .env or .env.example"
	exit 1
endif


# Download & install FFmpeg in the 'tools' directory for supported platforms.
.PHONY: tools
tools:
	$(MAKE) -s tools-linux
	$(MAKE) -s tools-darwin
	$(MAKE) -s tools-windows


# FFmpeg version to bundle.
FFMPEG_VERSION=7.0.2
TOOLS=./tools
TOOLS_DOWNLOAD=./tools/download

FFMPEG_PACKAGE_LINUX=$(TOOLS_DOWNLOAD)/ffmpeg-$(FFMPEG_VERSION)-linux-amd64-static.tar.xz
FFMPEG_PACKAGE_DARWIN=$(TOOLS_DOWNLOAD)/ffmpeg-$(FFMPEG_VERSION)-darwin-amd64.zip
FFMPEG_PACKAGE_WINDOWS=$(TOOLS_DOWNLOAD)/ffmpeg-$(FFMPEG_VERSION)-windows-amd64.zip

.PHONY: tools-linux
tools-linux:
	[ -e $(FFMPEG_PACKAGE_LINUX) ] || curl \
		--create-dirs -o $(FFMPEG_PACKAGE_LINUX) \
		https://www.johnvansickle.com/ffmpeg/releases/ffmpeg-$(FFMPEG_VERSION)-amd64-static.tar.xz
	tar xvf \
		$(FFMPEG_PACKAGE_LINUX) \
		ffmpeg-$(FFMPEG_VERSION)-amd64-static/ffmpeg
	mv ffmpeg-$(FFMPEG_VERSION)-amd64-static/ffmpeg $(TOOLS)/ffmpeg-linux-amd64
	rmdir ffmpeg-$(FFMPEG_VERSION)-amd64-static

.PHONY: tools-darwin
tools-darwin:
	[ -e $(FFMPEG_PACKAGE_DARWIN) ] || curl \
		--create-dirs -o $(FFMPEG_PACKAGE_DARWIN) \
		https://evermeet.cx/ffmpeg/ffmpeg-$(FFMPEG_VERSION).zip
	unzip $(FFMPEG_PACKAGE_DARWIN)
	mv ffmpeg $(TOOLS)/ffmpeg-darwin-amd64

.PHONY: tools-windows
tools-windows:
	[ -e $(FFMPEG_PACKAGE_WINDOWS) ] || curl \
		--create-dirs -o $(FFMPEG_PACKAGE_WINDOWS) \
		https://www.gyan.dev/ffmpeg/builds/packages/ffmpeg-$(FFMPEG_VERSION)-essentials_build.zip
	unzip -j $(FFMPEG_PACKAGE_WINDOWS) ffmpeg-$(FFMPEG_VERSION)-essentials_build/bin/ffmpeg.exe -d .
	mv ffmpeg.exe $(TOOLS)/ffmpeg-windows-amd64.exe


RELEASE_PACKAGE_LINUX_BASE := flamenco-${VERSION}-linux-amd64
RELEASE_PACKAGE_LINUX := ${RELEASE_PACKAGE_LINUX_BASE}.tar.gz

RELEASE_PACKAGE_DARWIN_BASE := flamenco-${VERSION}-macos-amd64
RELEASE_PACKAGE_DARWIN := ${RELEASE_PACKAGE_DARWIN_BASE}.tar.gz
RELEASE_PACKAGE_DARWIN_ARM64_BASE := flamenco-${VERSION}-macos-arm64
RELEASE_PACKAGE_DARWIN_ARM64 := ${RELEASE_PACKAGE_DARWIN_ARM64_BASE}.tar.gz

RELEASE_PACKAGE_WINDOWS_BASE := flamenco-${VERSION}-windows-amd64
RELEASE_PACKAGE_WINDOWS := ${RELEASE_PACKAGE_WINDOWS_BASE}.zip

RELEASE_PACKAGE_EXTRA_FILES := README.md LICENSE CHANGELOG.md
RELEASE_PACKAGE_SHAFILE := flamenco-${VERSION}.sha256

.PHONY: release-package
release-package:
	$(MAKE) -s vet
	$(MAKE) -s release-package-linux
	$(MAKE) -s release-package-darwin
	$(MAKE) -s release-package-windows
	$(MAKE) -s clean

.PHONY: release-package-linux
release-package-linux:
	$(MAKE) -s clean
	$(MAKE) -s webapp-static
	$(MAKE) -s flamenco-manager-without-webapp GOOS=linux GOARCH=amd64
	$(MAKE) -s flamenco-worker GOOS=linux GOARCH=amd64
	$(MAKE) -s tools-linux
	mkdir -p dist/${RELEASE_PACKAGE_LINUX_BASE}/tools
	cp flamenco-manager flamenco-worker ${RELEASE_PACKAGE_EXTRA_FILES} dist/${RELEASE_PACKAGE_LINUX_BASE}
	cp tools/*-linux* dist/${RELEASE_PACKAGE_LINUX_BASE}/tools
	cd dist; tar zcvf ${RELEASE_PACKAGE_LINUX} ${RELEASE_PACKAGE_LINUX_BASE}
	rm -rf dist/${RELEASE_PACKAGE_LINUX_BASE}
	@echo "Done! Created ${RELEASE_PACKAGE_LINUX}"

.PHONY: release-package-darwin
release-package-darwin:
	$(MAKE) -s clean
	$(MAKE) -s webapp-static

# AMD64
	$(MAKE) -s flamenco-manager-without-webapp GOOS=darwin GOARCH=amd64
	$(MAKE) -s flamenco-worker GOOS=darwin GOARCH=amd64
	$(MAKE) -s tools-darwin
	mkdir -p dist/${RELEASE_PACKAGE_DARWIN_BASE}/tools
	cp flamenco-manager flamenco-worker ${RELEASE_PACKAGE_EXTRA_FILES} dist/${RELEASE_PACKAGE_DARWIN_BASE}
	cp tools/*-darwin* dist/${RELEASE_PACKAGE_DARWIN_BASE}/tools
	cd dist; tar zcvf ${RELEASE_PACKAGE_DARWIN} ${RELEASE_PACKAGE_DARWIN_BASE}
	rm -rf dist/${RELEASE_PACKAGE_DARWIN_BASE}

# ARM64, without tools because ffmpeg.org doesn't link to any official ARM64 binary.
	$(MAKE) -s flamenco-manager-without-webapp GOOS=darwin GOARCH=arm64
	$(MAKE) -s flamenco-worker GOOS=darwin GOARCH=arm64
	mkdir -p dist/${RELEASE_PACKAGE_DARWIN_ARM64_BASE}/tools
	echo "Put an ffmpeg executable in this directory so that Flamenco Worker can find it" > dist/${RELEASE_PACKAGE_DARWIN_ARM64_BASE}/tools/put_ffmpeg_here.txt
	cp flamenco-manager flamenco-worker ${RELEASE_PACKAGE_EXTRA_FILES} dist/${RELEASE_PACKAGE_DARWIN_ARM64_BASE}
	cd dist; tar zcvf ${RELEASE_PACKAGE_DARWIN_ARM64} ${RELEASE_PACKAGE_DARWIN_ARM64_BASE}
	rm -rf dist/${RELEASE_PACKAGE_DARWIN_ARM64_BASE}

	@echo "Done! Created ${RELEASE_PACKAGE_DARWIN} and ${RELEASE_PACKAGE_DARWIN_ARM64}"

.PHONY: release-package-windows
release-package-windows:
	$(MAKE) -s clean
	$(MAKE) -s webapp-static
	$(MAKE) -s flamenco-manager-without-webapp GOOS=windows GOARCH=amd64
	$(MAKE) -s flamenco-worker GOOS=windows GOARCH=amd64
	$(MAKE) -s tools-windows
	mkdir -p dist/${RELEASE_PACKAGE_WINDOWS_BASE}/tools
	cp flamenco-manager.exe flamenco-worker.exe ${RELEASE_PACKAGE_EXTRA_FILES} dist/${RELEASE_PACKAGE_WINDOWS_BASE}
	cp tools/*-windows* dist/${RELEASE_PACKAGE_WINDOWS_BASE}/tools
	rm -f dist/${RELEASE_PACKAGE_WINDOWS}  # Don't update any existing ZIP.
	cd dist/${RELEASE_PACKAGE_WINDOWS_BASE}; zip -r -9 ../${RELEASE_PACKAGE_WINDOWS} flamenco-manager.exe flamenco-worker.exe ${RELEASE_PACKAGE_EXTRA_FILES} tools/*-windows*
	rm -rf dist/${RELEASE_PACKAGE_WINDOWS_BASE}
	@echo "Done! Created ${RELEASE_PACKAGE_WINDOWS}"

.PHONY: publish-release-packages
publish-release-packages:
	$(MAKE) -s check-environment
	cd dist; sha256sum ${RELEASE_PACKAGE_LINUX} ${RELEASE_PACKAGE_DARWIN} ${RELEASE_PACKAGE_DARWIN_ARM64} ${RELEASE_PACKAGE_WINDOWS} > ${RELEASE_PACKAGE_SHAFILE}
	cd dist; rsync -va -e "ssh -p ${WEBSERVER_SSH_PORT}" \
		${RELEASE_PACKAGE_LINUX} ${RELEASE_PACKAGE_DARWIN} ${RELEASE_PACKAGE_DARWIN_ARM64} ${RELEASE_PACKAGE_WINDOWS} ${RELEASE_PACKAGE_SHAFILE} \
		${WEBSERVER_SSH}:${WEBSERVER_ROOT}/downloads/

.PHONY: application version flamenco-manager flamenco-worker webapp webapp-static generate generate-go generate-py with-deps swagger-ui list-embedded test clean flamenco-manager-without-webapp format format-check

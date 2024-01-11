-include .env

PKG := projects.blender.org/studio/flamenco

# To update the version number in all the relevant places, update the VERSION
# variable below and run `make update-version`.
VERSION := 3.4-beta1
# "alpha", "beta", or "release".
RELEASE_CYCLE := beta

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

LDFLAGS := ${LDFLAGS} -X ${PKG}/internal/appinfo.ApplicationVersion=${VERSION} \
	-X ${PKG}/internal/appinfo.ApplicationGitHash=${GITHASH} \
	-X ${PKG}/internal/appinfo.ReleaseCycle=${RELEASE_CYCLE}
BUILD_FLAGS = -ldflags="${LDFLAGS}"

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
HUGO_PKG := github.com/gohugoio/hugo@v0.101.0

# Prevent any dependency that requires a C compiler, i.e. only work with pure-Go libraries.
export CGO_ENABLED=0

all: application

# Install generators and build the software.
with-deps:
	go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.9.0
	go install github.com/golang/mock/mockgen@v1.6.0
	$(MAKE) application

vet:
	go vet ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

application: webapp flamenco-manager flamenco-worker

flamenco-manager:
	$(MAKE) webapp-static
	go build -v ${BUILD_FLAGS} ${PKG}/cmd/flamenco-manager

.PHONY: flamenco-manager-without-webapp
flamenco-manager-without-webapp:
	go build -v ${BUILD_FLAGS} ${PKG}/cmd/flamenco-manager

flamenco-worker:
	go build -v ${BUILD_FLAGS} ${PKG}/cmd/flamenco-worker


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

.PHONY: stresser
stresser:
	go build -v ${BUILD_FLAGS} ${PKG}/cmd/stresser

.PHONY: job-creator
job-creator:
	go build -v ${BUILD_FLAGS} ${PKG}/cmd/job-creator

flamenco-addon.zip: addon-packer
	./addon-packer -filename ./flamenco-addon.zip

addon-packer: cmd/addon-packer/addon-packer.go
	go build -v ${BUILD_FLAGS} ${PKG}/cmd/addon-packer

flamenco-manager_race:
	CGO_ENABLED=1 go build -race -o $@ -v ${BUILD_FLAGS} ${PKG}/cmd/flamenco-manager

flamenco-worker_race:
	CGO_ENABLED=1 go build -race -o $@ -v ${BUILD_FLAGS} ${PKG}/cmd/flamenco-worker

.PHONY: shaman-checkout-id-setter
shaman-checkout-id-setter:
	go build -v ${BUILD_FLAGS} ${PKG}/cmd/shaman-checkout-id-setter

webapp:
	yarn --cwd web/app install

webapp-static: addon-packer
	$(MAKE) clean-webapp-static
# When changing the base URL, also update the line
# e.GET("/app/*", echo.WrapHandler(webAppHandler))
# in `cmd/flamenco-manager/main.go`
	yarn --cwd web/app build --outDir ../static --base=/app/ --logLevel warn
# yarn --cwd web/app build --outDir ../static --base=/app/ --minify false
	./addon-packer -filename ${WEB_STATIC}/flamenco-addon.zip
	@echo "Web app has been installed into ${WEB_STATIC}"

generate:
	$(MAKE) generate-go
	$(MAKE) generate-py
	$(MAKE) generate-js

generate-go:
	go generate ./pkg/api/...
	go generate ./internal/...
# The generators always produce UNIX line-ends. This creates false file
# modifications with Git. Convert them to DOS line-ends to avoid this.
ifeq ($(OS),Windows_NT)
	git status --porcelain | grep '^ M .*.gen.go' | cut -d' ' -f3 | xargs unix2dos --keepdate
endif

generate-py:
# The generator doesn't consistently overwrite existing files, nor does it
# remove no-longer-generated files.
	rm -rf addon/flamenco/manager

# See https://openapi-generator.tech/docs/generators/python for the options.
	java -jar addon/openapi-generator-cli.jar \
		generate \
		-i pkg/api/flamenco-openapi.yaml \
		-g python \
		-o addon/ \
		--package-name "${PY_API_PKG_NAME}" \
		--http-user-agent "Flamenco/${VERSION} (Blender add-on)" \
		-p generateSourceCodeOnly=true \
		-p projectName=Flamenco \
		-p packageVersion="${VERSION}" > .openapi-generator-py.log

# The generator outputs files so that we can write our own tests. We don't,
# though, so it's better to just remove those placeholders.
	rm -rf addon/flamenco/manager/test
# The generators always produce UNIX line-ends. This creates false file
# modifications with Git. Convert them to DOS line-ends to avoid this.
ifeq ($(OS),Windows_NT)
	git status --porcelain | grep '^ M addon/flamenco/manager' | cut -d' ' -f3 | xargs unix2dos --keepdate
endif

generate-js:
# The generator doesn't consistently overwrite existing files, nor does it
# remove no-longer-generated files.
	rm -rf web/app/src/manager-api
	rm -rf web/_tmp-manager-api-javascript

# See https://openapi-generator.tech/docs/generators/javascript for the options.
# Version '0.0.0' is used as NPM doesn't like Git hashes as versions.
#
# -p modelPropertyNaming=original is needed because otherwise the generator will
# use original naming internally, but generate docs with camelCase, and then
# things don't work properly.
	java -jar addon/openapi-generator-cli.jar \
		generate \
		-i pkg/api/flamenco-openapi.yaml \
		-g javascript \
		-o web/_tmp-manager-api-javascript \
		--http-user-agent "Flamenco/${VERSION} / webbrowser" \
		-p projectName=flamenco-manager \
		-p projectVersion="0.0.0" \
		-p apiPackage="${JS_API_PKG_NAME}" \
		-p disallowAdditionalPropertiesIfNotPresent=false \
		-p usePromises=true \
		-p moduleName=flamencoManager > .openapi-generator-js.log

# Cherry-pick the generated sources, and remove everything else.
# The only relevant bit is that the generated code depends on `superagent`,
# which is listed in our `.
	mv web/_tmp-manager-api-javascript/src web/app/src/manager-api
	rm -rf web/_tmp-manager-api-javascript
# The generators always produce UNIX line-ends. This creates false file
# modifications with Git. Convert them to DOS line-ends to avoid this.
ifeq ($(OS),Windows_NT)
	git status --porcelain | grep '^ M web/app/src/manager-api' | cut -d' ' -f3 | xargs unix2dos --keepdate
endif

.PHONY:
update-version:
	@echo "--- Updating Flamenco version to ${VERSION}"
	@echo "--- If this stops with exit status 42, it was already at that version."
	@echo
	go run ./cmd/update-version ${VERSION}
	$(MAKE) generate-py
	$(MAKE) generate-js
	@echo
	@echo 'File replacement done, commit with:'
	@echo
	@echo git commit -m '"Bumped version to ${VERSION}"' Makefile \
		addon/flamenco/__init__.py \
		addon/flamenco/manager \
		addon/flamenco/manager_README.md \
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

test:
# Ensure the web-static directory exists, so that `web/web_app.go` can embed something.
	mkdir -p ${WEB_STATIC}
	go test -short ./...

clean:
	@go clean -i -x
	rm -f flamenco*-v* flamenco-manager flamenco-worker *.exe flamenco-*_race addon-packer stresser
	$(MAKE) clean-webapp-static

clean-webapp-static:
# Start with `./` to avoid horrors when WEB_STATIC is absolute (like / or /home/yourname).
	rm -rf ./${WEB_STATIC}
# Make sure there is at least something to embed by Go, or it may cause some errors.
	mkdir -p ./${WEB_STATIC}
	touch ${WEB_STATIC}/emptyfile

devserver-website:
	go run ${HUGO_PKG} -s web/project-website serve

devserver-webapp:
	yarn --cwd web/app run dev --host

deploy-website:
	$(MAKE) -s check-environment
	rm -rf web/project-website/public/
	go run ${HUGO_PKG} -s web/project-website --baseURL https://flamenco.blender.org/
	rsync web/project-website/public/ ${WEBSERVER_SSH}:${WEBSERVER_ROOT}/ \
		-e "ssh" \
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
# Version 5.1.3 is the last release in the 5.x series, but binaries have not
# been made available. And then there are different 'latest' binaries for the
# different platforms.
FFMPEG_VERSION_LINUX=5.1.1
FFMPEG_VERSION_WINDOWS=5.1.2
FFMPEG_VERSION_DARWIN=5.1.2
TOOLS=./tools
TOOLS_DOWNLOAD=./tools/download

FFMPEG_PACKAGE_LINUX=$(TOOLS_DOWNLOAD)/ffmpeg-$(FFMPEG_VERSION_LINUX)-linux-amd64-static.tar.xz
FFMPEG_PACKAGE_DARWIN=$(TOOLS_DOWNLOAD)/ffmpeg-$(FFMPEG_VERSION_DARWIN)-darwin-amd64.zip
FFMPEG_PACKAGE_WINDOWS=$(TOOLS_DOWNLOAD)/ffmpeg-$(FFMPEG_VERSION_WINDOWS)-windows-amd64.zip

.PHONY: tools-linux
tools-linux:
	[ -e $(FFMPEG_PACKAGE_LINUX) ] || curl \
		--create-dirs -o $(FFMPEG_PACKAGE_LINUX) \
		https://www.johnvansickle.com/ffmpeg/old-releases/ffmpeg-$(FFMPEG_VERSION_LINUX)-amd64-static.tar.xz
	tar xvf \
		$(FFMPEG_PACKAGE_LINUX) \
		ffmpeg-$(FFMPEG_VERSION_LINUX)-amd64-static/ffmpeg \
		--strip-components=1
	mv ffmpeg $(TOOLS)/ffmpeg-linux-amd64

.PHONY: tools-darwin
tools-darwin:
	[ -e $(FFMPEG_PACKAGE_DARWIN) ] || curl \
		--create-dirs -o $(FFMPEG_PACKAGE_DARWIN) \
		https://evermeet.cx/ffmpeg/ffmpeg-$(FFMPEG_VERSION_DARWIN).zip
	unzip $(FFMPEG_PACKAGE_DARWIN)
	mv ffmpeg $(TOOLS)/ffmpeg-darwin-amd64

.PHONY: tools-windows
tools-windows:
	[ -e $(FFMPEG_PACKAGE_WINDOWS) ] || curl \
		--create-dirs -o $(FFMPEG_PACKAGE_WINDOWS) \
		https://www.gyan.dev/ffmpeg/builds/packages/ffmpeg-$(FFMPEG_VERSION_WINDOWS)-essentials_build.zip
	unzip -j $(FFMPEG_PACKAGE_WINDOWS) ffmpeg-$(FFMPEG_VERSION_WINDOWS)-essentials_build/bin/ffmpeg.exe -d .
	mv ffmpeg.exe $(TOOLS)/ffmpeg-windows-amd64.exe


RELEASE_PACKAGE_LINUX_BASE := flamenco-${VERSION}-linux-amd64
RELEASE_PACKAGE_LINUX := ${RELEASE_PACKAGE_LINUX_BASE}.tar.gz

RELEASE_PACKAGE_DARWIN_BASE := flamenco-${VERSION}-macos-amd64
RELEASE_PACKAGE_DARWIN := ${RELEASE_PACKAGE_DARWIN_BASE}.tar.gz
RELEASE_PACKAGE_DARWIN_ARM64_BASE := flamenco-${VERSION}-macos-arm64
RELEASE_PACKAGE_DARWIN_ARM64 := ${RELEASE_PACKAGE_DARWIN_ARM64_BASE}.tar.gz

RELEASE_PACKAGE_WINDOWS := flamenco-${VERSION}-windows-amd64.zip

RELEASE_PACKAGE_EXTRA_FILES := README.md LICENSE CHANGELOG.md
RELEASE_PACKAGE_SHAFILE := flamenco-${VERSION}.sha256

.PHONY: release-package
release-package:
	$(MAKE) -s release-package-linux
	$(MAKE) -s release-package-darwin
	$(MAKE) -s release-package-windows

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
	mkdir -p dist
	rm -f dist/${RELEASE_PACKAGE_WINDOWS}
	zip -r -9 dist/${RELEASE_PACKAGE_WINDOWS} flamenco-manager.exe flamenco-worker.exe ${RELEASE_PACKAGE_EXTRA_FILES} tools/*-windows*
	@echo "Done! Created ${RELEASE_PACKAGE_WINDOWS}"

.PHONY: publish-release-packages
publish-release-packages:
	$(MAKE) -s check-environment
	cd dist; sha256sum ${RELEASE_PACKAGE_LINUX} ${RELEASE_PACKAGE_DARWIN} ${RELEASE_PACKAGE_DARWIN_ARM64} ${RELEASE_PACKAGE_WINDOWS} > ${RELEASE_PACKAGE_SHAFILE}
	cd dist; rsync -va \
		${RELEASE_PACKAGE_LINUX} ${RELEASE_PACKAGE_DARWIN} ${RELEASE_PACKAGE_DARWIN_ARM64} ${RELEASE_PACKAGE_WINDOWS} ${RELEASE_PACKAGE_SHAFILE} \
		${WEBSERVER_SSH}:${WEBSERVER_ROOT}/downloads/

.PHONY: application version flamenco-manager flamenco-worker flamenco-manager_race flamenco-worker_race webapp webapp-static generate generate-go generate-py with-deps swagger-ui list-embedded test clean clean-webapp-static

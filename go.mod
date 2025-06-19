module projects.blender.org/studio/flamenco

go 1.24.4

require (
	github.com/adrg/xdg v0.4.0
	github.com/alessio/shellescape v1.4.2
	github.com/benbjohnson/clock v1.3.0
	github.com/deepmap/oapi-codegen v1.9.0
	github.com/disintegration/imaging v1.6.2
	github.com/dop251/goja v0.0.0-20230812105242-81d76064690d
	github.com/dop251/goja_nodejs v0.0.0-20211022123610-8dd9abb0616d
	github.com/eclipse/paho.golang v0.12.0
	github.com/fromkeith/gossdp v0.0.0-20180102154144-1b2c43f6886e
	github.com/gertd/go-pluralize v0.2.1
	github.com/getkin/kin-openapi v0.132.0
	github.com/golang/mock v1.6.0
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/google/uuid v1.5.0
	github.com/graarh/golang-socketio v0.0.0-20170510162725-2c44953b9b5f
	github.com/labstack/echo/v4 v4.9.1
	github.com/magefile/mage v1.15.0
	github.com/mattn/go-colorable v0.1.12
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c
	github.com/pressly/goose/v3 v3.15.1
	github.com/rs/zerolog v1.26.1
	github.com/stretchr/testify v1.9.0
	github.com/zcalusic/sysinfo v1.0.1
	github.com/ziflex/lecho/v3 v3.1.0
	golang.org/x/crypto v0.35.0
	golang.org/x/image v0.18.0
	golang.org/x/net v0.36.0
	golang.org/x/sync v0.11.0
	golang.org/x/sys v0.30.0
	golang.org/x/vuln v1.1.3
	gopkg.in/yaml.v2 v2.4.0
	honnef.co/go/tools v0.5.1
	modernc.org/sqlite v1.28.0
)

require (
	github.com/BurntSushi/toml v1.4.1-0.20240526193622-a339e1f7089c // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dlclark/regexp2 v1.7.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/google/pprof v0.0.0-20230207041349-798e818bf904 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/labstack/gommon v0.4.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/oasdiff/yaml v0.0.0-20250309154309-f31be36b4037 // indirect
	github.com/oasdiff/yaml3 v0.0.0-20250309153720-d2182401db90 // indirect
	github.com/perimeterx/marshmallow v1.1.5 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.1 // indirect
	golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678 // indirect
	golang.org/x/mod v0.19.0 // indirect
	golang.org/x/telemetry v0.0.0-20240522233618-39ace7a40ae7 // indirect
	golang.org/x/text v0.22.0 // indirect
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba // indirect
	golang.org/x/tools v0.23.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lukechampine.com/uint128 v1.3.0 // indirect
	modernc.org/cc/v3 v3.41.0 // indirect
	modernc.org/ccgo/v3 v3.16.15 // indirect
	modernc.org/libc v1.37.6 // indirect
	modernc.org/mathutil v1.6.0 // indirect
	modernc.org/memory v1.7.2 // indirect
	modernc.org/opt v0.1.3 // indirect
	modernc.org/strutil v1.2.0 // indirect
	modernc.org/token v1.1.0 // indirect
)

// Replace staticcheck release with a specific revision of their `main` branch,
// so that it includes my PR https://github.com/dominikh/go-tools/pull/1597
replace honnef.co/go/tools v0.5.1 => honnef.co/go/tools v0.0.0-20240920144234-9f4b51e3ab5a

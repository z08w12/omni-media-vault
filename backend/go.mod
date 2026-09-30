module github.com/video-site/backend

go 1.23.0

toolchain go1.23.4

require (
	github.com/andybalholm/brotli v1.2.2
	github.com/go-chi/chi/v5 v5.1.0
	golang.org/x/crypto v0.25.0
	golang.org/x/image v0.29.0
	golang.org/x/net v0.27.0
	golang.org/x/sys v0.30.0
	golang.org/x/text v0.28.0
	gopkg.in/yaml.v3 v3.0.1
	modernc.org/sqlite v1.34.5
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v0.1.9 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	modernc.org/libc v1.55.3 // indirect
	modernc.org/mathutil v1.6.0 // indirect
	modernc.org/memory v1.8.0 // indirect
)

// 依赖已通过 go mod vendor 打包进 backend/vendor/ 并入库，支持离线构建。
// 升级 SDK 请使用标准流程：
//   go get github.com/SheltonZhu/115driver@<版本>
//   go mod tidy
//   go mod vendor

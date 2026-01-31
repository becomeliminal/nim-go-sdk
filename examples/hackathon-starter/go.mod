module github.com/becomeliminal/nim-go-sdk/examples/hackathon-starter

go 1.23.0

toolchain go1.23.4

replace github.com/becomeliminal/nim-go-sdk => ../..

require (
	github.com/becomeliminal/nim-go-sdk v0.2.0
	github.com/joho/godotenv v1.5.1
)

require (
	github.com/anthropics/anthropic-sdk-go v1.20.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgraph-io/ristretto v0.1.1 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/golang/glog v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	golang.org/x/sys v0.34.0 // indirect
)

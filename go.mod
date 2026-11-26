module github.com/qnlbnsl/lgtm-studio

go 1.26.0

tool (
	connectrpc.com/connect/cmd/protoc-gen-connect-go
	google.golang.org/protobuf/cmd/protoc-gen-go
)

require (
	connectrpc.com/connect v1.21.1
	github.com/jackc/pgx/v5 v5.13.0
	go.etcd.io/bbolt v1.4.3
	golang.org/x/crypto v0.59.0
	golang.org/x/net v0.63.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/text v0.44.0 // indirect
)

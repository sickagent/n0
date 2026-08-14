module github.com/sickagent/n0/example/plugin

go 1.26

require (
	github.com/sickagent/n0/proto/gen/go v0.0.0
	google.golang.org/grpc v1.64.1
	google.golang.org/protobuf v1.34.2
)

require (
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
)

replace github.com/sickagent/n0/proto/gen/go => ../../proto/gen/go

module github.com/fish-tennis/gserver

go 1.16

require (
	github.com/fish-tennis/gentity v0.4.0
	github.com/fish-tennis/gnet v0.2.0
	github.com/go-redis/redis/v8 v8.11.4
	go.mongodb.org/mongo-driver v1.8.0
	google.golang.org/protobuf v1.26.0
)

replace (
	github.com/fish-tennis/gentity v0.4.0 => ./../gentity
)

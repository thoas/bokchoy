install:
	go get -u google.golang.org/grpc
	go get -u github.com/golang/protobuf/protoc-gen-go
	go get github.com/gogo/protobuf/protoc-gen-gofast

generate:
	rm -rf proto/*.go
	protoc \
		-I proto \
		-I=$$GOPATH/src \
		-I=$$GOPATH/src/github.com/gogo/protobuf/protobuf \
		--gogo_out=\
Mgoogle/protobuf/timestamp.proto=github.com/golang/protobuf/ptypes/timestamp,\
Mgoogle/protobuf/duration.proto=github.com/golang/protobuf/ptypes/duration,\
Mgoogle/protobuf/wrappers.proto=github.com/golang/protobuf/ptypes/wrappers,\
plugins=grpc:proto \
		proto/bokchoy.proto

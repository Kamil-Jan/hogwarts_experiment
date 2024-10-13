```
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin

go get google.golang.org/grpc@latest

go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```
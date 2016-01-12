# go-lambda example


## Usage

### Protocol Buffers (proto3)

    protoc -Iproto --go_out=plugins=grpc:service/test proto/test.proto

### Develop

    go run main.go

### Build

    go build -v -o bin/darwin64

### Publish

    mkdir -p {bin,dist,node_modules}
    GOOS=linux GOARCH=amd64 go build -v -o bin/linux64
    rm -f dist/lambda.zip
    zip -r dist/lambda.zip bin/linux64 node_modules/ index.js
    # Deploy dist/lambda.zip


## Sample Payload

### Lambda Event

    node .

```json
{
  "service": "Test",
  "method": "SayHello",
  "data": {
    "name": "foo"
  }
}
```

### Go process stdin

    go run main.go

```json
{"context":{"awsRequestId":"1"},"event":{"service":"Test","method":"SayHello","data":{"name":"foo"}}}
```

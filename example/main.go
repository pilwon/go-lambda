package main

import (
	"github.com/pilwon/go-lambda"

	"github.com/pilwon/go-lambda/example/service"
)

func main() {
	s := lambda.NewServer()
	s.Register(service.LambdaServices())
	s.Run()
}

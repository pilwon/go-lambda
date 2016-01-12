package service

import (
	"github.com/pilwon/go-lambda"

	"github.com/pilwon/go-lambda/example/service/test"
)

func LambdaServices() []lambda.Service {
	return []lambda.Service{
		{test.Desc(), new(test.Server)},
	}
}

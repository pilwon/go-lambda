package test

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func Desc() *grpc.ServiceDesc {
	return &_Test_serviceDesc
}

type Server struct {
}

func (s *Server) SayHello(c context.Context, req *HelloRequest) (*HelloResponse, error) {
	res := &HelloResponse{
		Message: "Hello, " + req.Name,
	}
	return res, nil
}

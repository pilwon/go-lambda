package lambda

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type Payload struct {
	Event   *PayloadEvent   `json:"event"`
	Context *PayloadContext `json:"context"`
}

func (p *Payload) String() string {
	id := ""
	if p.Context != nil {
		id = p.Context.AWSRequestID
	}
	return fmt.Sprintf("[Payload %s] %s", id, p.Event.String())
}

type PayloadEvent struct {
	Package *string          `json:"package"`
	Service *string          `json:"service"`
	Method  *string          `json:"method"`
	Data    *json.RawMessage `json:"data"`
}

func (e *PayloadEvent) String() string {
	return fmt.Sprintf("[%s] %s", NewMethodID(*e.Package, *e.Service, *e.Method), *e.Data)
}

type PayloadContext struct {
	FunctionName       string                       `json:"functionName"`
	FunctionVersion    string                       `json:"functionVersion"`
	InvokedFunctionARN string                       `json:"invokedFunctionArn"`
	MemoryLimitInMB    string                       `json:"memoryLimitInMB"`
	AWSRequestID       string                       `json:"awsRequestId"`
	LogGroupName       string                       `json:"logGroupName"`
	LogStreamName      string                       `json:"logStreamName"`
	Identity           *PayloadContextIdentity      `json:"identity"`
	ClientContext      *PayloadContextClientContext `json:"clientContext"`
}

type PayloadContextClientContext struct {
	Client *PayloadContextClientContextClient `json:"client"`
	Custom interface{}
	Env    *PayloadContextClientContextEnv `json:"env"`
}

type PayloadContextClientContextClient struct {
	InstallationID string `json:"installation_id"`
	AppTitle       string `json:"app_title"`
	AppVersionName string `json:"app_version_name"`
	AppVersionCode string `json:"app_version_code"`
	AppPackageName string `json:"app_package_name"`
}

type PayloadContextClientContextEnv struct {
	PlatformVersion string `json:"platform_version"`
	Platform        string `json:"platform"`
	Make            string `json:"make"`
	Model           string `json:"model"`
	Locale          string `json:"locale"`
}

type PayloadContextIdentity struct {
	CognitoIdentityID     string `json:"cognito_identity_id"`
	CognitoIdentityPoolID string `json:"cognito_identity_pool_id"`
}

type Response struct {
	Context *PayloadContext
	Reply   *proto.Message
	Error   error
}

func (r *Response) String() string {
	id := ""
	if r.Context != nil {
		id = r.Context.AWSRequestID
	}
	if r.Error != nil {
		return fmt.Sprintf("[Response %s] Error: %s", id, r.Error.Error())
	}
	return fmt.Sprintf("[Response %s] %s", id, (*r.Reply).String())
}

var replyMarshaler = &jsonpb.Marshaler{}

func NewResponse(c *PayloadContext, reply *proto.Message, err error) *Response {
	return &Response{
		Context: c,
		Reply:   reply,
		Error:   err,
	}
}

func (r *Response) EncodeToJSON() string {
	id := "null"
	if r.Context != nil {
		id = fmt.Sprintf(`"%s"`, r.Context.AWSRequestID)
	}
	if r.Error != nil {
		return fmt.Sprintf(`{"id":%s,"error":"%s"}`, id, strings.Replace(r.Error.Error(), `"`, `\"`, -1))
	}
	reply, err := replyMarshaler.MarshalToString(*r.Reply)
	if err != nil {
		log.Fatalf("Failed to encode response to JSON: %s", err.Error())
	}
	return fmt.Sprintf(`{"id":%s,"reply":%s}`, id, reply)
}

type Service struct {
	ServiceDesc *grpc.ServiceDesc
	Server      interface{}
}

type MethodID string

func NewMethodID(pkg string, svc string, mtd string) MethodID {
	var id string
	if pkg == "" {
		id = fmt.Sprintf("%s/%s", svc, mtd)
	} else {
		id = fmt.Sprintf("%s.%s/%s", pkg, svc, mtd)
	}
	return MethodID(id)
}

func (id MethodID) String() string {
	return string(id)
}

type handler struct {
	srv interface{}
	md  *grpc.MethodDesc
}

type Server struct {
	handlers map[MethodID]handler
}

func NewServer() *Server {
	return &Server{
		handlers: map[MethodID]handler{},
	}
}

func (s *Server) Register(svcs []Service) {
	for _, svc := range svcs {
		for _, md := range svc.ServiceDesc.Methods {
			uid := NewMethodID("", svc.ServiceDesc.ServiceName, md.MethodName)
			s.handlers[uid] = handler{svc.Server, &md}
		}
	}
}

func (s *Server) Run() {
	payloadCh := make(chan *Payload)
	resCh := make(chan *Response)
	errCh := make(chan error)
	go s.listenStdin(payloadCh, resCh, errCh)
	for {
		select {
		case payload := <-payloadCh:
			log.Printf("go-lambda RCVD %s\n", payload.String())
			go s.processPayload(payload, resCh)
		case res := <-resCh:
			fmt.Println(res.EncodeToJSON())
			log.Printf("go-lambda SENT %s\n", res.String())
		case err := <-errCh:
			log.Fatal(err)
		}
	}
}

func (s *Server) listenStdin(payloadCh chan *Payload, resCh chan *Response, errCh chan error) {
	log.Println("go-lambda listening stdin...")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var payload Payload
		if err := json.Unmarshal(scanner.Bytes(), &payload); err != nil {
			resCh <- NewResponse(payload.Context, nil, fmt.Errorf("invalid payload"))
			continue
		}
		payloadCh <- &payload
	}
	if err := scanner.Err(); err != nil {
		errCh <- err
	}
}

func (s *Server) processPayload(payload *Payload, resCh chan *Response) {
	var (
		reply *proto.Message
		err   error
	)
	switch {
	case payload.Event == nil:
		err = fmt.Errorf("payload missing event")
	case payload.Event.Package == nil:
		*payload.Event.Package = ""
		fallthrough
	case payload.Event.Service == nil:
		err = fmt.Errorf("payload missing event.service")
	case payload.Event.Method == nil:
		err = fmt.Errorf("payload missing event.method")
	default:
		var data io.Reader
		if payload.Event.Data == nil {
			data = strings.NewReader("{}")
		} else {
			data = bytes.NewReader(*payload.Event.Data)
		}
		methodID := NewMethodID(*payload.Event.Package, *payload.Event.Service, *payload.Event.Method)
		reply, err = s.callGRPCMethod(methodID, data)
	}
	resCh <- NewResponse(payload.Context, reply, err)
}

func (s *Server) callGRPCMethod(id MethodID, data io.Reader) (*proto.Message, error) {
	decode := func(v interface{}) error {
		if err := jsonpb.Unmarshal(data, v.(proto.Message)); err != nil {
			return fmt.Errorf("invalid method (%s) data", id)
		}
		return nil
	}
	h, ok := s.handlers[id]
	if !ok {
		return nil, fmt.Errorf("method (%s) handler not found", id)
	}
	reply, err := h.md.Handler(h.srv, context.Background(), decode)
	if err != nil {
		return nil, err
	}
	replyMsg := reply.(proto.Message)
	return &replyMsg, nil
}

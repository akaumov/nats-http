package nats_http

import (
	"github.com/akaumov/nats-http/pb"
	"github.com/akaumov/natspool"
	"github.com/golang/protobuf/proto"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type NatsHttp struct {
	config   *Config
	natsPool *natspool.Pool
}

func New(config *Config) *NatsHttp {
	return &NatsHttp{
		config: config,
	}
}

func (h *NatsHttp) handleRequest(writer http.ResponseWriter, request *http.Request) {

	var err error
	var body []byte

	if request.Body != nil {
		body, err = ioutil.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, "ServerError", 500)
			return
		}
		request.Body.Close()

		if body != nil && len(body) == 0 {
			body = nil
		}
	}

	requestPacket := pb.Request{
		Method:     request.Method,
		Host:       request.Host,
		RemoteAddr: request.RemoteAddr,
		RequestURI: request.RequestURI,
		Body:       body,
	}

	requestPacketData, err := proto.Marshal(&requestPacket)

	if err != nil {
		http.Error(writer,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
		return
	}

	natsClient, err := h.natsPool.Get()
	if err != nil {
		http.Error(writer,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
		return
	}
	h.natsPool.Put(natsClient)

	timeout := time.Duration(h.config.Timeout) * time.Millisecond
	responseMsg, err := natsClient.Request(h.config.NatsOutputSubject, requestPacketData, timeout)

	if err != nil {
		http.Error(writer,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
		return
	}

	var response pb.Response
	err = proto.Unmarshal(responseMsg.Data, &response)

	if err != nil {
		http.Error(writer,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(int(response.Status))
	if response.Body != nil && len(response.Body) > 0 {
		writer.Write(response.Body)
	}
}

func (h *NatsHttp) Start() {

	natsPool, err := natspool.New(h.config.NatsAddress, h.config.NatsPoolSize)
	if err != nil {
		log.Fatalf("can't connect to nats: %v", err)
		return
	}

	h.natsPool = natsPool

	http.HandleFunc(h.config.UrlPattern, h.handleRequest)
	log.Println("Start nats-http on: " + h.config.ListenInterface)
	log.Fatal(http.ListenAndServe(h.config.ListenInterface, nil))
}

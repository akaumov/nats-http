package nats_http

import (
	"encoding/json"
	"fmt"
	"github.com/akaumov/nats-http/js"
	"github.com/akaumov/nats-http/pb"
	"github.com/akaumov/natspool"
	"github.com/golang/protobuf/proto"
	"github.com/nats-io/go-nats"
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

func (h *NatsHttp) packRequest(request *http.Request) ([]byte, error) {

	var err error
	var body []byte

	if request.Body != nil {
		body, err = ioutil.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		request.Body.Close()

		if body != nil && len(body) == 0 {
			body = nil
		}
	}

	switch h.config.PacketFormat {
	case "json":
		requestPacket := js.Request{
			Method:     request.Method,
			Host:       request.Host,
			RemoteAddr: request.RemoteAddr,
			RequestURI: request.RequestURI,
			Body:       body,
		}
		requestPacketData, err := json.Marshal(&requestPacket)
		return requestPacketData, err

	case "protobuf":
		requestPacket := pb.Request{
			Method:     request.Method,
			Host:       request.Host,
			RemoteAddr: request.RemoteAddr,
			RequestURI: request.RequestURI,
			Body:       body,
		}

		requestPacketData, err := proto.Marshal(&requestPacket)
		return requestPacketData, err

	default:

	}

	log.Panicf("Unsuported format: %v", h.config.PacketFormat)
	return nil, fmt.Errorf("unsuported format: %v", h.config.PacketFormat)
}

func (h *NatsHttp) handleResponse(responseMsg *nats.Msg, writer http.ResponseWriter) error {

	switch h.config.PacketFormat {
	case "json":
		var response js.Response

		err := json.Unmarshal(responseMsg.Data, &response)
		if err != nil {
			return err
		}

		writer.WriteHeader(int(response.Status))
		if response.Body != nil && len(response.Body) > 0 {
			writer.Write(response.Body)
		}

	case "protobuf":
		var response pb.Response

		err := proto.Unmarshal(responseMsg.Data, &response)
		if err != nil {
			return err
		}

		writer.WriteHeader(int(response.Status))
		if response.Body != nil && len(response.Body) > 0 {
			writer.Write(response.Body)
		}
	}

	return nil
}

func (h *NatsHttp) handleRequest(writer http.ResponseWriter, request *http.Request) {

	requestPacketData, err := h.packRequest(request)
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

	h.handleResponse(responseMsg, writer)
	if err != nil {
		http.Error(writer,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
		return
	}
}

func (h *NatsHttp) Start() {

	natsPool, err := natspool.New(h.config.NatsAddress, h.config.NatsPoolSize)
	if err != nil {
		log.Panicf("can't connect to nats: %v", err)
	}

	h.natsPool = natsPool

	http.HandleFunc(h.config.UrlPattern, h.handleRequest)
	log.Println("Start nats-http on: " + h.config.ListenInterface)
	log.Fatal(http.ListenAndServe(h.config.ListenInterface, nil))
}

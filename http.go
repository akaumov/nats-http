package nats_http

import (
	"encoding/json"
	"fmt"
	"github.com/SermoDigital/jose/crypto"
	"github.com/SermoDigital/jose/jws"
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

type PackRequestFunc func(userId string, deviceId string, packetFormat string, request *http.Request) ([]byte, error)

type NatsHttp struct {
	config      *Config
	natsPool    *natspool.Pool
	packRequest PackRequestFunc
}

func New(config *Config) *NatsHttp {
	return &NatsHttp{
		config:      config,
		packRequest: packRequestDefault,
	}
}

func NewCustom(config *Config, packRequestHook PackRequestFunc) *NatsHttp {
	return &NatsHttp{
		config:      config,
		packRequest: packRequestHook,
	}
}

func (h *NatsHttp) getLoginData(tokenString string) (*string, *string, error) {

	if tokenString == "" {
		return nil, nil, fmt.Errorf("empty token")
	}

	newToken, err := jws.ParseJWT([]byte(tokenString))
	if err != nil {
		return nil, nil, err
	}

	err = newToken.Validate([]byte(h.config.JwtSecret), crypto.SigningMethodHS256)
	if err != nil {
		return nil, nil, err
	}

	claims := newToken.Claims()
	userId := claims.Get("userId").(string)
	deviceId := claims.Get("deviceId").(string)

	return &userId, &deviceId, nil
}

func packRequestDefault(userId string, deviceId string, packetFormat string, request *http.Request) ([]byte, error) {

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

	switch packetFormat {
	case "json":
		requestPacket := js.Request{
			UserId:     userId,
			DeviceId:   deviceId,
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
			UserId:     userId,
			DeviceId:   deviceId,
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

	log.Panicf("Unsuported format: %v", packetFormat)
	return nil, fmt.Errorf("unsuported format: %v", packetFormat)
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

	token := request.Header.Get("X-Auth-Token")

	userId, deviceId, err := h.getLoginData(token)
	if err != nil {
		http.Error(writer,
			http.StatusText(http.StatusUnauthorized),
			http.StatusUnauthorized)
		return
	}

	requestPacketData, err := h.packRequest(*userId, *deviceId, h.config.PacketFormat, request)
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
		if err == nats.ErrTimeout {
			http.Error(writer,
				http.StatusText(http.StatusGatewayTimeout),
				http.StatusGatewayTimeout)
			return
		}

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

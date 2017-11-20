package nats_http

type Config struct {
	ListenInterface string `json:"listenInterface"`
	UrlPattern      string `json:"urlPattern"`
	PacketFormat    string `json:"packetFormat"`
	Timeout         int64  `json:"timeout"`
	JwtSecret       string `json:"jwtSecret"`

	NatsAddress       string `json:"natsAddress"`
	NatsPoolSize      int    `json:"natsPoolSize"`
	NatsOutputSubject string `json:"natsOutputSubject"`
}

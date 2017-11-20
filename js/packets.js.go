package js

type Request struct {
	Method     string `json:"method"`
	Host       string `json:"host"`
	RemoteAddr string `json:"remoteAddr"`
	RequestURI string `json:"requestURI"`
	Body       []byte `json:"body"`
}

type Response struct {
	Status int64  `json:"status"`
	Body   []byte `json:"body"`
}

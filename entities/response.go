package entities

import (
	"encoding/json"
	"time"
	"vindr-lab-api/constants"
)

type Response struct {
	ErrorCode  int                     `json:"error_code"`
	ServerTime int64                   `json:"server_time"`
	Count      int                     `json:"count,omitempty"`
	Data       interface{}             `json:"data,omitempty"`
	Agg        *map[string]interface{} `json:"agg,omitempty"`
	Meta       *map[string]interface{} `json:"meta,omitempty"`
}

func new(data interface{}, errCode int) Response {
	var res Response
	res.Data = data
	res.ErrorCode = errCode
	res.ServerTime = time.Now().Unix()
	return res
}

func (resp *Response) New() {
	*resp = new(nil, constants.ServerOK)
}

func NewResponse() *Response {
	var res Response
	res.New()
	return &res
}

func (resp *Response) String() string {
	b, err := json.Marshal(resp)
	if err != nil {
		return "{}"
	}
	return string(b)
}

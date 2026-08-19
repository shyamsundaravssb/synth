package ipc

import (
	"bufio"
	"encoding/json"
	"io"
)

type Request struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

const (
	TypePing   = "ping"
	TypeSearch = "search"
	TypeStatus = "status"
)

type PingPayload struct{}

type SearchPayload struct {
	Query     string `json:"query"`
	Limit     int    `json:"limit"`
	FilePath  string `json:"file,omitempty"`
	Since     string `json:"since,omitempty"`
	Developer string `json:"developer,omitempty"`
}

type StatusPayload struct{}

type Response struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
}

const (
	StatusOK    = "ok"
	StatusError = "error"
)

type PingData struct {
	PID     int    `json:"pid"`
	Version string `json:"version"`
}

type SearchResultItem struct {
	ID         string  `json:"id"`
	FilePath   string  `json:"file"`
	Type       string  `json:"type"`
	Branch     string  `json:"branch"`
	Developer  string  `json:"developer"`
	Timestamp  string  `json:"timestamp"`
	What       string  `json:"what"`
	Why        string  `json:"why"`
	Impact     string  `json:"impact,omitempty"`
	Score      float64 `json:"score"`
	SearchMode string  `json:"search_mode"`
}

type SearchData struct {
	Results    []SearchResultItem `json:"results"`
	Count      int                `json:"count"`
	SearchMode string             `json:"search_mode"`
	Query      string             `json:"query"`
}

type LowContextFileItem struct {
	FilePath         string `json:"file"`
	SaveCount        int    `json:"save_count"`
	HasEverBeenNoted bool   `json:"has_note"`
	DaysSinceNote    int    `json:"days_since_note"`
}

type StatusData struct {
	Running         bool                 `json:"running"`
	PID             int                  `json:"pid"`
	UptimeS         int64                `json:"uptime_seconds"`
	NotesCount      int                  `json:"notes_count"`
	FileSavesCount  int                  `json:"file_saves_count"`
	EmbeddingsCount int                  `json:"embeddings_count"`
	LowContextCount int                  `json:"low_context_count"`
	LowContextFiles []LowContextFileItem `json:"low_context_files"`
	LogFile         string               `json:"log_file"`
	SockFile        string               `json:"socket_file"`
}

type ErrorData struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

func NewRequest(msgType string, payload interface{}) (*Request, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &Request{Type: msgType, Payload: raw}, nil
}

func NewOKResponse(data interface{}) (*Response, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &Response{Status: StatusOK, Data: raw}, nil
}

func NewErrorResponse(message, code string) *Response {
	ed := ErrorData{Message: message, Code: code}
	raw, _ := json.Marshal(ed)
	return &Response{Status: StatusError, Data: raw}
}

func ParseSearchPayload(r *Request) (*SearchPayload, error) {
	var sp SearchPayload
	if err := json.Unmarshal(r.Payload, &sp); err != nil {
		return nil, err
	}
	return &sp, nil
}

func ParseSearchData(r *Response) (*SearchData, error) {
	var sd SearchData
	if err := json.Unmarshal(r.Data, &sd); err != nil {
		return nil, err
	}
	return &sd, nil
}

func ParsePingData(r *Response) (*PingData, error) {
	var pd PingData
	if err := json.Unmarshal(r.Data, &pd); err != nil {
		return nil, err
	}
	return &pd, nil
}

func ParseStatusData(r *Response) (*StatusData, error) {
	var sd StatusData
	if err := json.Unmarshal(r.Data, &sd); err != nil {
		return nil, err
	}
	return &sd, nil
}

func ParseErrorData(r *Response) (*ErrorData, error) {
	var ed ErrorData
	if err := json.Unmarshal(r.Data, &ed); err != nil {
		return nil, err
	}
	return &ed, nil
}

func EncodeRequest(w io.Writer, req *Request) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}

func DecodeRequest(r io.Reader) (*Request, error) {
	br := bufio.NewReader(r)
	b, err := br.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	var req Request
	if err := json.Unmarshal(b, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func EncodeResponse(w io.Writer, resp *Response) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}

func DecodeResponse(r io.Reader) (*Response, error) {
	br := bufio.NewReader(r)
	b, err := br.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	var resp Response
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

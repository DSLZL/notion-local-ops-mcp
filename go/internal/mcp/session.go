package mcp

type Session struct {
	id     any
	method string
}

func NewSession(req Request) *Session {
	return &Session{
		id:     req.ID,
		method: req.Method,
	}
}

func (s *Session) Method() string {
	return s.method
}

func (s *Session) Response(result map[string]any) response {
	return response{
		JSONRPC: "2.0",
		ID:      s.id,
		Result:  result,
	}
}

func (s *Session) Error(code int, message string) response {
	return response{
		JSONRPC: "2.0",
		ID:      s.id,
		Error: &responseError{
			Code:    code,
			Message: message,
		},
	}
}

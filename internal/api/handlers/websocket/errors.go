package websocket

import "errors"

var (
	ErrPrinterNotConnected = errors.New("printer not connected")
	ErrSendBufferFull      = errors.New("send buffer full")
)


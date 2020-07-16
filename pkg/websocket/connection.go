package websocket

import (
	"context"
	"io"
	"time"

	ws "github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type InfiniteConnection = interface {
	Run(ctx context.Context, onMessage func(io.Reader), onDisconnect func(), reconnect func() (Connection, error)) func(io.Reader)
}

type infiniteConnection struct{}

func (c *infiniteConnection) Run(ctx context.Context, onMessage func(io.Reader), onDisconnect func(error), reconnect func() (Connection, error)) func(io.Reader) {
	var conn Connection
	for {
		conn, err := reconnect()
		if err != nil {
			onDisconnect(err)
		}
	}
}

type Connection = interface {
	Run(onMessage func(io.Reader), onDisconnect func()) func(io.Reader) error
}

type connection struct {
	conn     *ws.Conn
	Log      log.Logger
	PongWait time.Duration
}

func (c *connection) Run(onMessage func([]byte), onDisconnect func()) func(io.Reader) error {
	go c.readPump(c.conn, onMessage, onDisconnect)
	return func(r io.Reader) error {
		w, err := c.conn.NextWriter(ws.BinaryMessage)
		if err != nil {
			return err
		}
		c.conn.NextWriter(ws.BinaryMessage)
		if _, err := io.Copy(w, r); err != nil {
			return err
		}

		if err := w.Close(); err != nil {
			return err
		}
		return nil
	}
}

func (c *connection) readPump(conn *ws.Conn, onMessage func([]byte), onDisconnect func()) {
	err := conn.SetReadDeadline(time.Now().Add(c.PongWait))
	if err != nil {
		c.Log.Debug("SetReadDeadline error: ", err)
	}

	conn.SetPongHandler(func(string) error {
		c.Log.WithFields(log.Fields{
			"prefix": "websocket.Client.readPump",
		}).Debug("Received pong message")

		err := conn.SetReadDeadline(time.Now().Add(c.PongWait))
		if err != nil {
			c.Log.Debug("SetReadDeadline error: ", err)
		}

		return nil
	})

	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				switch {
				case !ws.IsCloseError(err):
					// read errors do not prevent websocket reconnects in the CLI so we should
					// only display this on debug-level logging
					c.Log.WithFields(log.Fields{
						"prefix": "websocket.Client.Close",
					}).Debug("read error: ", err)
				case ws.IsUnexpectedCloseError(err, ws.CloseNormalClosure):
					c.Log.WithFields(log.Fields{
						"prefix": "websocket.Client.Close",
					}).Error("close error: ", err)
					c.Log.WithFields(log.Fields{
						"prefix": "stripecli.ADDITIONAL_INFO",
					}).Error("If you run into issues, please re-run with `--log-level debug` and share the output with the Stripe team on GitHub.")
				default:
					c.Log.Error("other error: ", err)
					c.Log.WithFields(log.Fields{
						"prefix": "stripecli.ADDITIONAL_INFO",
					}).Error("If you run into issues, please re-run with `--log-level debug` and share the output with the Stripe team on GitHub.")
				}
				onDisconnect()
				return
			}

			c.Log.WithFields(log.Fields{
				"prefix":  "websocket.Client.readPump",
				"message": string(data),
			}).Debug("Incoming message")

			onMessage(data)
		}
	}()
}

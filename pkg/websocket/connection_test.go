package websocket

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	ws "github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func waitFor(t *testing.T, done chan struct{}, timeout time.Duration) {
	timedOut := make(chan struct{})
	go func() {
		time.Sleep(timeout)
		close(timedOut)
	}()
	select {
	case <-done:
		return
	case <-timedOut:
		t.Fatal("Timed out waiting for server to receive message from client")
	}
}

func TestRun(t *testing.T) {
	upgrader := ws.Upgrader{}
	gotMessageFromClient := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)

		msg := []byte("\"hi\"")

		err = c.WriteMessage(ws.TextMessage, msg)
		require.NoError(t, err)

		go func() {
			_, reader, err := c.NextReader()
			require.NoError(t, err)
			_, _ = ioutil.ReadAll(reader)
			close(gotMessageFromClient)
		}()
	}))

	defer ts.Close()
	ctx := context.Background()
	url := "ws" + strings.TrimPrefix(ts.URL, "http")

	dialer := ws.Dialer{}
	conn, _, err := dialer.DialContext(ctx, url, http.Header{})
	require.NoError(t, err)

	myConn := connection{
		conn: conn,
	}

	var send func(io.Reader) error
	gotMessageFromServer := make(chan struct{})
	onMessage := func(msg []byte) {
		close(gotMessageFromServer)
		send(bytes.NewReader([]byte("\"fromClient\"")))
	}

	onDisconnect := func() {}

	send = myConn.Run(onMessage, onDisconnect)

	waitFor(t, gotMessageFromServer, time.Second*2)
	waitFor(t, gotMessageFromClient, time.Second*2)
}

func TestRunHandlesExpiredReadDeadlines(t *testing.T) {
	upgrader := ws.Upgrader{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		require.NoError(t, err)

		time.Sleep(time.Second * 2)
		msg := []byte("\"hi\"")

		err = c.WriteMessage(ws.TextMessage, msg)
		require.NoError(t, err)
	}))

	defer ts.Close()
	ctx := context.Background()
	url := "ws" + strings.TrimPrefix(ts.URL, "http")

	dialer := ws.Dialer{}
	conn, _, err := dialer.DialContext(ctx, url, http.Header{})
	require.NoError(t, err)

	logger := log.New()
	logger.Level = log.DebugLevel

	myConn := connection{
		conn:     conn,
		PongWait: time.Second,
		Log:      *logger,
	}
	onMessage := func(msg []byte) {
		t.Fatal(string(msg))
	}

	onDisconnectCalled := make(chan struct{})
	onDisconnect := func() {
		close(onDisconnectCalled)
	}

	timedOut := make(chan struct{})
	go func() {
		time.Sleep(time.Second * 3)
		close(timedOut)
	}()
	send := myConn.Run(onMessage, onDisconnect)
	send(bytes.NewReader([]byte("\"fromClient\"")))
	select {
	case <-onDisconnectCalled:
		return
	case <-timedOut:
		t.Fatal("Timed out waiting for onDisconnect to be called")
	}

	err = send(bytes.NewReader([]byte("should error")))
	if err == nil {
		t.Fatal("Expected send to error after the connection was disconnected")
	}
}

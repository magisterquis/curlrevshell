package hsrv

/*
 * websocket.go
 * Upgrade to websockets if asked.
 * By J. Stuart McMurray
 * Created 20260805
 * Last Modified 20260806
 */

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/magisterquis/curlrevshell/internal/hsrv/hsrvws"
	"golang.org/x/net/websocket"
)

// maybeWS checks if r would like to be a websocket, and if so upgrades it.
// nil is returned if the connection did not need to be or was not able to be
// pgraded.
func maybeWS(
	ctx context.Context,
	sl *slog.Logger,
	w http.ResponseWriter,
	r *http.Request,
) *websocket.Conn {
	/* If this isn't a websocket connection, life's easy. */
	if !hsrvws.IsWebsocketUpgradeRequest(r) {
		return nil
	}

	/* Websocket connection, maybe? */
	wsCh := make(chan *websocket.Conn, 1)
	go websocket.Server{
		/* Handshake makes sure that conf.Origin is set so we don't end
		up with a nil pointer derefence if someone calls RemoteAddr. */
		Handshake: func(
			conf *websocket.Config,
			r *http.Request,
		) error {
			conf.Origin = &url.URL{Host: r.RemoteAddr}
			return nil
		},
		/* Handler handshaekes, upgrades, and sends back the
		connection. */
		Handler: func(c *websocket.Conn) {
			defer close(wsCh)
			sl.Debug(LMUpgradedToWebsocket)
			wsCh <- c
			<-ctx.Done()
		},
	}.ServeHTTP(w, r)

	/* Grab the websocket. */
	c := <-wsCh
	if nil == c {
		sl.Debug(LMUpgradeToWebsocketFailed)
		return nil
	}
	return c
}

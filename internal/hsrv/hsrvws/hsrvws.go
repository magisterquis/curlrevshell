// Package hsrvws - Websocket things used by hsrv and others
package hsrvws

/*
 * hsrvws.go
 * Websocket things used by hsrv and others
 * By J. Stuart McMurray
 * Created 20260807
 * Last Modified 20260807
 */

import (
	"encoding/base64"
	"net/http"
	"strings"
)

const (
	websocketKeyB64Len = 24
	websocketKeyLen    = 16
	websocketVersion   = "13"
)

// IsWebsocketUpgradeRequest indicates whether or not r is a request for an
// upgrade to a websocket.
func IsWebsocketUpgradeRequest(r *http.Request) bool {
	/* Much of this inspired by
	https://cs.opensource.google/go/x/net/+/refs/tags/v0.57.0:websocket/hybi.go
	and https://websocket.org/reference/headers/ */

	/* Method is always GET. */
	if http.MethodGet != r.Method {
		return false
	}
	/* These two headers are always present. */
	if "websocket" != strings.ToLower(r.Header.Get("Upgrade")) {
		return false
	}
	if !strings.Contains(
		strings.ToLower(r.Header.Get("Connection")),
		"upgrade",
	) {
		return false
	}
	/* Only one version nowadays. */
	if websocketVersion != r.Header.Get("Sec-WebSocket-Version") {
		return false
	}
	/* Need a 16-byte key to prevent accidental upgrades. */
	keyB64 := r.Header.Get("Sec-WebSocket-Key")
	if 24 != len(keyB64) || !strings.HasSuffix(keyB64, "==") {
		return false
	}
	if b, err := base64.StdEncoding.DecodeString(
		keyB64,
	); nil != err || 16 != len(b) {
		return false
	}

	/* Looks websocketish, then. */
	return true

}

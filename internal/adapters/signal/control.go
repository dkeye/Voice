package signal

func (ctl *SignalWSController) handlePing(
	conn *WsSignalConn,
) {
	resp := struct {
		Type string `json:"type"`
	}{
		Type: "pong",
	}
	ctl.sendJSON(conn, resp)
}

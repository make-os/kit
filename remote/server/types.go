package server

import (
	"net"

	tmservice "github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/p2p"
)

// Peer is an interface representing a peer connected on a reactor.
type Peer interface {
	tmservice.Service
	FlushStop()

	ID() p2p.ID           // peer's cryptographic ID
	RemoteIP() net.IP     // remote IP of the connection
	RemoteAddr() net.Addr // remote address of the connection

	IsOutbound() bool   // did we dial the peer
	IsPersistent() bool // do we redial this peer when we disconnect

	CloseConn() error // close original connection

	NodeInfo() p2p.NodeInfo // peer's info
	Status() p2p.ConnectionStatus
	SocketAddr() *p2p.NetAddress // actual address of the socket

	Send(byte, []byte) bool
	TrySend(byte, []byte) bool

	Set(string, interface{})
	Get(string) interface{}
}

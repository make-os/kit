package types

import (
	"net"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/p2p"

	tmconn "github.com/tendermint/tendermint/p2p/conn"
)

// Peer is an interface representing a peer connected on a reactor.
type Peer interface {
	cmn.Service
	FlushStop()

	ID() p2p.ID           // peer's cryptographic ID
	RemoteIP() net.IP     // remote IP of the connection
	RemoteAddr() net.Addr // remote address of the connection

	IsOutbound() bool   // did we dial the peer
	IsPersistent() bool // do we redial this peer when we disconnect

	CloseConn() error // close original connection

	NodeInfo() p2p.NodeInfo // peer's info
	Status() tmconn.ConnectionStatus
	SocketAddr() *p2p.NetAddress // actual address of the socket

	Send(byte, []byte) bool
	TrySend(byte, []byte) bool

	Set(string, interface{})
	Get(string) interface{}
}

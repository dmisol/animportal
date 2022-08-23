package anim

import (
	"net"

	lksdk "github.com/livekit/server-sdk-go"
)

const (
	port = 10000
)

// monitor dir
// encode video
// feed it both to hall (as ID) and to dummy (as "flexatar")

type Video struct {
	Ms *int64

	dummy, hall *lksdk.Room
	owner       string

	conn    *net.TCPConn
	Compute func()
}

func (v *Video) Run() (err error) {
	return
}

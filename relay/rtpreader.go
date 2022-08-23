package relay

import (
	"context"
	"errors"
	"log"
	"net"
	"sync/atomic"

	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

type RtpReader interface {
	ReadRTP() (*rtp.Packet, interceptor.Attributes, error)
	Close() (err error)
}

func NewRtpReader(ctx context.Context, in interface{}) (rr RtpReader, err error) {
	if port, ok := in.(int); ok {
		rr = newUdpRtpReader(ctx, port)
		return
	}
	if t, ok := in.(*webrtc.TrackRemote); ok {
		rr = newTrackRtpReader(ctx, t)
		return
	}
	err = errors.New("can't make RtpReader")
	return
}
func newUdpRtpReader(ctx context.Context, port int) (r RtpReader) {
	u := &UdpReader{
		mq:   make(chan []byte, rtpqueue),
		stop: make(chan bool),
		Port: port,
	}
	u.ctx, u.cancel = context.WithCancel((ctx))

	r = u
	return
}

type UdpReader struct {
	ctx    context.Context
	cancel context.CancelFunc
	Port   int

	mq   chan []byte
	stop chan bool
	fill int32

	started bool
}

func (r *UdpReader) start() {

	go func() {
		udp, err := net.ListenUDP("udp", &net.UDPAddr{Port: r.Port})
		if err != nil {
			r.Println(err)
			return
		}
		defer func() {
			r.Println("closing", r.Port)
			udp.Close()
			r.started = false
		}()
		udp.SetReadBuffer(rtplinuxbuf)
		thr := int32(0.7 * float64(rtpqueue))

		r.Println("starting rtp thread", r.Port)
		for {
			select {
			case <-r.ctx.Done():
				return
			default:
				p := make([]byte, 4096)
				n, _, err := udp.ReadFrom(p)
				if err != nil {
					r.Println("udp rd", err)
					return
				}

				x := atomic.AddInt32(&r.fill, 1)
				r.mq <- p[:n]
				if x >= thr {
					r.Println("buffers in queue", x)
				}
			}
		}
	}()
}

func (r *UdpReader) ReadRTP() (packet *rtp.Packet, attr interceptor.Attributes, err error) {
	if !r.started {
		r.started = true
		r.start()
	}

	packet = &rtp.Packet{}

	atomic.AddInt32(&r.fill, -1)
	p := <-r.mq
	err = packet.Unmarshal(p)
	return
}

func (r *UdpReader) Close() (err error) {
	r.Println("close()")
	r.cancel()
	return
}

func (r *UdpReader) Println(i ...interface{}) {
	log.Println("udp", i)
}

func newTrackRtpReader(ctx context.Context, t *webrtc.TrackRemote) (r RtpReader) {
	u := &TrackRtpReader{
		TrackRemote: t,
	}
	r = u
	return
}

type TrackRtpReader struct {
	*webrtc.TrackRemote
}

func (r TrackRtpReader) Close() (err error) {
	return
}

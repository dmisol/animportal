package anim

import (
	"context"
	"io"
	"log"
	"net"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dmisol/animportal/defs"
	"github.com/dmisol/animportal/relay"
	"github.com/gen2brain/x264-go"
	lksdk "github.com/livekit/server-sdk-go"
	"github.com/pion/webrtc/v3"
)

var (
	opts = &x264.Options{
		Width:     defs.InitialJson.W,
		Height:    defs.InitialJson.H,
		FrameRate: defs.InitialJson.FPS,
		Tune:      "zerolatency",
		Preset:    "veryfast",
		Profile:   "baseline",
		LogLevel:  x264.LogDebug,
	}
)

func NewEngine(ctx context.Context, addr string, ram string, room *lksdk.Room) (e *Engine, err error) {
	e = &Engine{
		Room: room,
		t0:   time.Now(),
	}
	e.Context, _ = context.WithCancel(ctx)
	e.animation = newAnimation(e.Context, addr, path.Join(ram, "pcm"), e.onEncodedVideo)
	e.bridge = &bridge{}

	e.enc, err = x264.NewEncoder(e.bridge, opts)
	return
}

type Engine struct {
	mu           sync.Mutex
	audio, video io.ReadCloser
	*animation
	enc *x264.Encoder
	*bridge

	context.Context
	*lksdk.Room
	t0 time.Time

	*relay.Relay
	dir     string
	started int32
}

func (e *Engine) OnAuioTrack(remote *webrtc.TrackRemote) {
	// read audio, decode, resample, feed to animation

	/* agreed on:
	- files@ramdisk, name over tcp socket
	- 16 bits, mono, 16kHz, 50 ms
	- byte order to be verifier
	- anim server removes useless files
	- client removes folder @ ramdisk
	*/
	e.Println("start sending audio for animation")

	e.audio = NewAudioProc(remote, e.animation)
	e.video = NewVideo(e.animation)

	go func() {
		<-e.Context.Done()
		e.Println("stop sending audio for animation")

		e.audio.Close()
		e.video.Close()
		e.enc.Close()
	}()

}

func (e *Engine) onEncodedVideo() {
	x := atomic.AddInt32(&e.started, 1)
	if x != 1 {
		return
	}
	var err error
	if e.Relay, err = relay.NewRelay(e.Context, e.Room); err != nil {

	}
	e.Relay.AddReadCloser(e.audio, webrtc.MimeTypeOpus)
	e.Relay.AddReadCloser(e.bridge, webrtc.MimeTypeH264)
}

func newAnimation(ctx context.Context, addr string, dir string, f func()) (p *animation) {
	var err error
	// mkdir in ramfs
	os.MkdirAll(dir, 0755)

	// connect to port
	p.conn, err = net.Dial("tcp", addr)
	// send initial json

	// start reading images
	go func() {
		defer p.conn.Close()

		for {
			select {
			case <-ctx.Done():
				p.Println("killex (ctx)")
				return
			default:
				b := make([]byte, 1024)
				i, err := p.conn.Read(b)
				if err != nil {
					p.Println("sock rd", err)
					return
				}
				name := string(b[:i])
				if err = p.procImage(name); err != nil {
					p.Println("h264 encoding", err)
					return
				}
			}
		}
	}()
}

func (e *Engine) Println(i ...interface{}) {
	log.Println("anim.engine", i)
}

type animation struct {
	conn net.Conn
	dir  string
}

func (p *animation) procImage(name string) (err error) {

}

// Write() will be called when PCM portion is ready to be sent for animation computing
func (p *animation) Write(b []byte) (i int, err error) {
	return
}

func (p *animation) Println(i ...interface{}) {
	log.Println("anim", i)
}

// converts Writer to ReadCloser
// x264enc -> bridge -> relay
type bridge struct {
	// todo: convert to RFC 6184 ?
	mu   sync.Mutex
	data [][]byte
}

func (b *bridge) Write(p []byte) (i int, err error) {
}

func (b *bridge) Read(p []byte) (i int, err error) {
}

func (b *bridge) Close() (err error) {}

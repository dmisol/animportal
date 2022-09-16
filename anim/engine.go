package anim

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
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

func NewEngine(ctx context.Context, addr string, ram string, room *lksdk.Room, conf defs.PortalConf) (e *Engine, err error) {
	e = &Engine{
		Room: room,
		t0:   time.Now(),
	}
	e.Context, _ = context.WithCancel(ctx)
	if e.animation, err = newAnimation(e.Context, addr, path.Join(ram, "pcm"), e.onEncodedVideo, conf.InitialJson); err != nil {
		return
	}

	return
}

type Engine struct {
	audio io.ReadCloser
	*animation

	context.Context
	*lksdk.Room
	t0 time.Time

	*relay.Relay
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

	e.audio = newAudioProc(remote, e.animation)

	go func() {
		<-e.Context.Done()
		e.Println("stop sending audio for animation")

		e.audio.Close()
		e.animation.Close()
	}()

}

func (e *Engine) onEncodedVideo() {
	x := atomic.AddInt32(&e.started, 1)
	if x != 1 {
		return
	}
	var err error
	if e.Relay, err = relay.NewRelay(e.Context, e.Room); err != nil {
		e.Println("newRelay", err)
		return
	}
	e.Relay.AddReadCloser(e.audio, webrtc.MimeTypeOpus)
	e.Relay.AddReadCloser(e.animation.bridge, webrtc.MimeTypeH264)
}

func (e *Engine) Println(i ...interface{}) {
	log.Println("anim.engine", i)
}

func newAnimation(ctx context.Context, addr string, dir string, f func(), conf defs.InitialJson) (p *animation, err error) {
	// mkdir in ramfs
	os.MkdirAll(dir, 0755)

	// create structure
	p = &animation{dir: dir}
	p.bridge = &bridge{}
	opts := &x264.Options{
		Width:     conf.W,
		Height:    conf.H,
		FrameRate: conf.FPS,
		Tune:      "zerolatency",
		Preset:    "veryfast",
		Profile:   "baseline",
		LogLevel:  x264.LogDebug,
	}
	if p.enc, err = x264.NewEncoder(p.bridge, opts); err != nil {
		return
	}

	// connect to port
	if p.conn, err = net.Dial("tcp", addr); err != nil {
		return
	}

	// send initial json
	var b []byte
	if b, err = json.Marshal(conf); err != nil {
		return
	}
	if _, err = p.conn.Write(b); err != nil {
		return
	}

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
	return
}

type animation struct {
	conn net.Conn
	dir  string

	index int64

	enc *x264.Encoder
	*bridge
}

func (p *animation) procImage(name string) (err error) {
	var r *os.File
	if r, err = os.Open(name); err != nil {
		return
	}

	var img image.Image
	if img, _, err = image.Decode(r); err != nil {
		return
	}
	// conv data to h264 and Write() to *bridge
	err = p.enc.Encode(img)
	return
}

// Write() will be called when PCM portion is ready to be sent for animation computing
func (p *animation) Write(pcm []byte) (i int, err error) {
	// create file
	name := fmt.Sprintf("%s/%d.pcm", p.dir, atomic.AddInt64(&p.index, 1))
	if err = os.WriteFile(name, pcm, 0666); err != nil {
		return
	}
	i = len(pcm)

	// send name to socket
	_, err = p.conn.Write([]byte(name))
	return
}

func (p *animation) Close() (err error) {
	p.enc.Close()
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

	remained []byte
}

func (b *bridge) Write(p []byte) (i int, err error) {
	i = len(p)

	b.mu.Lock()
	defer b.mu.Unlock()

	b.data = append(b.data, p)
	return
}

func (b *bridge) Read(p []byte) (i int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.remained) > 0 {
		i = copy(p, b.remained)
		b.remained = b.remained[:i]
		return
	}

	if len(b.data) == 0 {
		return
	}

	b.remained = b.data[0]
	b.data = b.data[1:]

	i = copy(p, b.remained)
	b.remained = b.remained[:i]
	return
}

func (b *bridge) Close() (err error) { return }

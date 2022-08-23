package anim

import (
	"context"
	"io"
	"sync/atomic"
	"time"

	"github.com/dmisol/animportal/relay"
	lksdk "github.com/livekit/server-sdk-go"
	"github.com/pion/webrtc/v3"
)

func NewEngine(ctx context.Context, room *lksdk.Room) (e *Engine, err error) {
	e = &Engine{
		Room: room,
		t0:   time.Now(),
	}
	e.Context, _ = context.WithCancel(ctx)
	return
}

type Engine struct {
	Audio, Video io.ReadCloser

	context.Context
	*lksdk.Room
	t0 time.Time

	*relay.Relay
	dir     string
	started int32
}

func (e *Engine) OnAuioTrack(remote *webrtc.TrackRemote) {
	// read audio, decode, resampe, feed to animation
}

func (e *Engine) onEncodedVideo() {
	x := atomic.AddInt32(&e.started, 1)
	if x != 1 {
		return
	}
	var err error
	if e.Relay, err = relay.NewRelay(e.Context, e.Room); err != nil {

	}
	e.Relay.AddReadCloser(e.Audio, webrtc.MimeTypeOpus)
	e.Relay.AddReadCloser(e.Video, webrtc.MimeTypeH264)
}

package relay

import (
	"context"
	"io"
	"log"

	lksdk "github.com/livekit/server-sdk-go"
	webrtc "github.com/pion/webrtc/v3"
)

const (
	rtpqueue    = 200
	rtplinuxbuf = 200000
)

func NewRelay(ctx context.Context, room *lksdk.Room) (r *Relay, err error) {
	r = &Relay{Room: room}
	r.Context, r.CancelFunc = context.WithCancel(ctx)

	go func() {
		<-r.Context.Done()
		r.Room.Disconnect()
	}()
	return
}

type Relay struct {
	*lksdk.Room

	context.Context
	context.CancelFunc
}

func (r *Relay) AddTrack(remote *webrtc.TrackRemote) {
	mime := webrtc.MimeTypeH264
	if remote.Kind() == webrtc.RTPCodecTypeAudio {
		mime = webrtc.MimeTypeOpus
	}

	rr := newTrackRtpReader(r.Context, remote)
	track, err := lksdk.NewLocalReaderTrack(NewTrackReadCloser(rr, mime), mime)
	if err != nil {
		r.Println("local track", err)
		return
	}

	if _, err = r.Room.LocalParticipant.PublishTrack(track, &lksdk.TrackPublicationOptions{}); err != nil {
		r.Println("addTrack", err)
		return
	}
	r.Println("relaying track", mime)
}

func (r *Relay) AddReadCloser(rc io.ReadCloser, mime string) {
	track, err := lksdk.NewLocalReaderTrack(rc, mime)
	if err != nil {
		r.Println("local track", err)
		return
	}

	if _, err = r.Room.LocalParticipant.PublishTrack(track, &lksdk.TrackPublicationOptions{}); err != nil {
		r.Println("addRc", err)
		return
	}
	r.Println("relaying rc", mime)
}

func (r *Relay) Close() {
	r.Println("closing")
	r.CancelFunc()
}

func (r *Relay) Println(i ...interface{}) {
	log.Println("relay", r.Room.LocalParticipant.GetPublisherPeerConnection())
}

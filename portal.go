package animportal

import (
	"context"
	"log"
	"path"
	"sync"
	"time"

	"github.com/dmisol/animportal/anim"
	"github.com/dmisol/animportal/relay"
	lksdk "github.com/livekit/server-sdk-go"
	webrtc "github.com/pion/webrtc/v3"
)

const (
	rtcTimeout = 2 * time.Hour
)

func NewPortal(ctx context.Context, hall string, dummy string, name string) (p *Portal, err error) {
	p = &Portal{
		Relays: make(map[string]*relay.Relay),
		Owner:  name,
		room:   dummy,
	}
	p.Context, p.CancelFunc = context.WithCancel(ctx)

	// subscribe to hall, set cb to colect participants
	if p.Hall, err = lksdk.ConnectToRoom(conf.Ws, lksdk.ConnectInfo{
		APIKey:              conf.Key,
		APISecret:           conf.Secret,
		RoomName:            hall,
		ParticipantIdentity: name,
	}, &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed: p.hallCb,
		},
	}, func(cp *lksdk.ConnectParams) { cp.AutoSubscribe = false }); err != nil {
		panic(err)
	}

	if p.Engine, err = anim.NewEngine(p.Context, conf.AnimAddr, path.Join(conf.Ram, dummy), p.Hall); err != nil {
		return
	}

	// subscribe to dummy, forward audio for (processing, hall)
	// also publish video to dummy as "flexatar", for monitoring
	if p.Dummy, err = lksdk.ConnectToRoom(conf.Ws, lksdk.ConnectInfo{
		APIKey:              conf.Key,
		APISecret:           conf.Secret,
		RoomName:            dummy,
		ParticipantIdentity: "anim",
	}, &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed:  p.dummyCb,
			OnTrackUnpublished: p.stop,
		},
	}, func(cp *lksdk.ConnectParams) { cp.AutoSubscribe = false }); err != nil {
		panic(err)
	}

	return
}

func (p *Portal) stop(publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
	if rp.Identity() == p.Owner {
		p.Println("owner left, closing")
		p.CancelFunc()
	}
}

func (p *Portal) Close() {

	if p.Dummy != nil {
		p.Dummy.Disconnect()
	}
	if p.Hall != nil {
		p.Hall.Disconnect()
	}

	p.CancelFunc()
}

func (p *Portal) Println(i ...interface{}) {
	log.Println("portal", i)
}

type Portal struct {
	*anim.Engine

	context.Context
	context.CancelFunc
	mu sync.Mutex

	room string

	Dummy, Hall *lksdk.Room             // connections for the given user, who is to be replaced with flexatar
	Relays      map[string]*relay.Relay //*lksdk.Room // connections to Dummy to publish all Halls' publishers
	Owner       string
}

// to get new publoshers in Hall to fill []*Relays
func (p *Portal) hallCb(remote *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
	id := rp.Identity()

	p.mu.Lock()
	defer p.mu.Unlock()

	r, ok := p.Relays[id]
	if !ok {
		p.Println("relaying (hall->dummy", id)

		room, err := lksdk.ConnectToRoom(conf.Ws, lksdk.ConnectInfo{
			APIKey:              conf.Key,
			APISecret:           conf.Secret,
			RoomName:            p.room,
			ParticipantIdentity: id,
		}, &lksdk.RoomCallback{
			ParticipantCallback: lksdk.ParticipantCallback{},
		})
		if err != nil {
			p.Println("romm error", id, err)
			return
		}

		r, _ = relay.NewRelay(p.Context, room)
		p.Relays[id] = r
	}
	if id == p.Owner && remote.Kind() == webrtc.RTPCodecTypeAudio {
		p.Println("do not publish audio back, ft only - skipping")
		return
	}
	r.AddTrack(remote)
}

func (p *Portal) dummyCb(remote *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
	if p.Owner != rp.Identity() {
		return
	}
	if remote.Kind() != webrtc.RTPCodecTypeAudio {
		return
	}
	go p.Engine.OnAuioTrack(remote)
}

type PattGen interface {
}

type ImgGen interface {
}

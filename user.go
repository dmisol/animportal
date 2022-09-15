package animportal

import (
	"context"
	"log"
	"path"
	"sync"
	"time"

	"github.com/dmisol/animportal/anim"
	"github.com/dmisol/animportal/defs"
	"github.com/dmisol/animportal/relay"
	lksdk "github.com/livekit/server-sdk-go"
	webrtc "github.com/pion/webrtc/v3"
)

const (
	rtcTimeout = 2 * time.Hour
)

func (ap *AnimationPortal) newUser(ctx context.Context, hall string, dummy string, name string) (p *user, err error) {
	p = &user{
		Relays: make(map[string]*relay.Relay),
		Owner:  name,
		room:   dummy,
		conf:   ap.PortalConf,
	}
	p.Context, p.CancelFunc = context.WithCancel(ctx)

	// subscribe to hall, set cb to colect participants
	if p.Hall, err = lksdk.ConnectToRoom(ap.PortalConf.Ws, lksdk.ConnectInfo{
		APIKey:              ap.PortalConf.Key,
		APISecret:           ap.PortalConf.Secret,
		RoomName:            hall,
		ParticipantIdentity: name,
	}, &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed: p.hallCb,
		},
	}, func(cp *lksdk.ConnectParams) { cp.AutoSubscribe = false }); err != nil {
		panic(err)
	}

	if p.Engine, err = anim.NewEngine(p.Context, ap.PortalConf.AnimAddr, path.Join(ap.PortalConf.Ram, dummy), p.Hall); err != nil {
		return
	}

	// subscribe to dummy, forward audio for (processing, hall)
	// also publish video to dummy as "flexatar", for monitoring
	if p.Dummy, err = lksdk.ConnectToRoom(ap.PortalConf.Ws, lksdk.ConnectInfo{
		APIKey:              ap.PortalConf.Key,
		APISecret:           ap.PortalConf.Secret,
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

func (p *user) stop(publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
	if rp.Identity() == p.Owner {
		p.Println("owner left, closing")
		p.CancelFunc()
	}
}

func (p *user) Close() {

	if p.Dummy != nil {
		p.Dummy.Disconnect()
	}
	if p.Hall != nil {
		p.Hall.Disconnect()
	}

	p.CancelFunc()
}

func (p *user) Println(i ...interface{}) {
	log.Println("portal", i)
}

type user struct {
	*anim.Engine

	context.Context
	context.CancelFunc
	mu sync.Mutex

	room string

	Dummy, Hall *lksdk.Room             // connections for the given user, who is to be replaced with flexatar
	Relays      map[string]*relay.Relay //*lksdk.Room // connections to Dummy to publish all Halls' publishers
	Owner       string

	conf *defs.PortalConf
}

// to get new publoshers in Hall to fill []*Relays
func (p *user) hallCb(remote *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
	id := rp.Identity()

	p.mu.Lock()
	defer p.mu.Unlock()

	r, ok := p.Relays[id]
	if !ok {
		p.Println("relaying (hall->dummy", id)

		room, err := lksdk.ConnectToRoom(p.conf.Ws, lksdk.ConnectInfo{
			APIKey:              p.conf.Key,
			APISecret:           p.conf.Secret,
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

func (p *user) dummyCb(remote *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
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

package animportal

import (
	"context"
	"log"
	"path"
	"sync"

	"github.com/dmisol/animportal/anim"
	"github.com/dmisol/animportal/defs"
	"github.com/dmisol/animportal/relay"
	lksdk "github.com/livekit/server-sdk-go"
	webrtc "github.com/pion/webrtc/v3"
)

func (ap *AnimationPortal) newUser(ctx context.Context, hall string, dummy string, name string, conf defs.PortalConf) (u *user, err error) {
	u = &user{
		Relays: make(map[string]*relay.Relay),
		Owner:  name,
		room:   dummy,
		conf:   ap.PortalConf,
	}
	u.Context, u.CancelFunc = context.WithCancel(ctx)

	// subscribe to hall, set cb to colect participants
	if u.Hall, err = lksdk.ConnectToRoom(ap.PortalConf.Ws, lksdk.ConnectInfo{
		APIKey:              ap.PortalConf.Key,
		APISecret:           ap.PortalConf.Secret,
		RoomName:            hall,
		ParticipantIdentity: name,
	}, &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed: u.hallCb,
		},
	}, func(cp *lksdk.ConnectParams) { cp.AutoSubscribe = false }); err != nil {
		panic(err)
	}

	if u.Engine, err = anim.NewEngine(u.Context, ap.PortalConf.AnimAddr, path.Join(ap.PortalConf.Ram, dummy), u.Hall, conf); err != nil {
		return
	}

	// subscribe to dummy, forward audio for (processing, hall)
	// also publish video to dummy as "flexatar", for monitoring
	if u.Dummy, err = lksdk.ConnectToRoom(ap.PortalConf.Ws, lksdk.ConnectInfo{
		APIKey:              ap.PortalConf.Key,
		APISecret:           ap.PortalConf.Secret,
		RoomName:            dummy,
		ParticipantIdentity: "anim",
	}, &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed:  u.dummyCb,
			OnTrackUnpublished: u.stop,
		},
	}, func(cp *lksdk.ConnectParams) { cp.AutoSubscribe = false }); err != nil {
		panic(err)
	}

	return
}

func (u *user) stop(publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
	if rp.Identity() == u.Owner {
		u.Println("owner left, closing")
		u.CancelFunc()
	}
}

func (u *user) Close() {

	if u.Dummy != nil {
		u.Dummy.Disconnect()
	}
	if u.Hall != nil {
		u.Hall.Disconnect()
	}

	u.CancelFunc()
}

func (u *user) Println(i ...interface{}) {
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
func (u *user) hallCb(remote *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
	id := rp.Identity()

	u.mu.Lock()
	defer u.mu.Unlock()

	r, ok := u.Relays[id]
	if !ok {
		u.Println("relaying (hall->dummy", id)

		room, err := lksdk.ConnectToRoom(u.conf.Ws, lksdk.ConnectInfo{
			APIKey:              u.conf.Key,
			APISecret:           u.conf.Secret,
			RoomName:            u.room,
			ParticipantIdentity: id,
		}, &lksdk.RoomCallback{
			ParticipantCallback: lksdk.ParticipantCallback{},
		})
		if err != nil {
			u.Println("romm error", id, err)
			return
		}

		r, _ = relay.NewRelay(u.Context, room)
		u.Relays[id] = r
	}
	if id == u.Owner && remote.Kind() == webrtc.RTPCodecTypeAudio {
		u.Println("do not publish audio back, ft only - skipping")
		return
	}
	r.AddTrack(remote)
}

func (u *user) dummyCb(remote *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
	if u.Owner != rp.Identity() {
		return
	}
	if remote.Kind() != webrtc.RTPCodecTypeAudio {
		return
	}
	u.Engine.OnAuioTrack(remote)
}

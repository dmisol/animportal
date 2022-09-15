package animportal

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/dmisol/animportal/defs"
	"github.com/google/uuid"
	"github.com/livekit/protocol/auth"
	"github.com/valyala/fasthttp"
	"gopkg.in/yaml.v2"
)

const (
	lifetime = 2 * time.Hour
)

type AnimationPortal struct {
	*defs.PortalConf
}

func (ap *AnimationPortal) Init(name string) (err error) {
	var cont []byte
	cont, err = ioutil.ReadFile(name)
	if err != nil {
		return
	}

	ap.PortalConf = &defs.PortalConf{}
	if err = yaml.Unmarshal(cont, ap.PortalConf); err != nil {
		return
	}

	cont, err = ioutil.ReadFile("./config.json")
	if err != nil {
		return err
	}
	err = json.Unmarshal(cont, &defs.InitialJson)
	return
}

// /animate?name=xxx - hall's name is ft
// /animate?name=xxx&hall=yyy
func (ap *AnimationPortal) Handler(r *fasthttp.RequestCtx) {
	name := string(r.FormValue("name"))
	hall := string(r.FormValue("hall"))
	dummy := uuid.NewString()

	if name == "" {
		r.Error("no name", fasthttp.StatusBadRequest)
		return
	}
	if hall == "" {
		hall = "ft"
	}

	t, err := ap.signToken(lifetime, name, "", dummy)
	if err != nil {
		r.Error("can't make token", fasthttp.StatusInternalServerError)
		return
	}

	ctx, _ := context.WithTimeout(context.Background(), lifetime)
	go func() {
		p, err := ap.newUser(ctx, hall, dummy, name)
		if err != nil {
			r.Error("can't start portal", fasthttp.StatusInternalServerError)
			return
		}
		r.WriteString(t)

		<-ctx.Done()
		p.Println("portal closed")
	}()
}

func (ap *AnimationPortal) signToken(lifetime time.Duration, uid, name, room string) (token string, err error) {

	canPublish := true
	canSubscribe := true

	at := auth.NewAccessToken(ap.PortalConf.Key, ap.PortalConf.Secret)
	grant := &auth.VideoGrant{
		RoomJoin:     true,
		Room:         room,
		CanPublish:   &canPublish,
		CanSubscribe: &canSubscribe,
	}

	at.AddGrant(grant).SetIdentity(uid)
	if len(name) > 0 {
		at.SetName(name)
	}
	at.AddGrant(grant).SetValidFor(lifetime)

	token, err = at.ToJWT()
	return
}

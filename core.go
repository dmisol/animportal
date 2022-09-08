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

var (
	conf *defs.PortalConf
)

const (
	lifetime = 2 * time.Hour
)

func Init(name string) (err error) {
	var cont []byte
	cont, err = ioutil.ReadFile(name)
	if err != nil {
		return
	}

	conf = &defs.PortalConf{}
	if err = yaml.Unmarshal(cont, conf); err != nil {
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
func Handler(r *fasthttp.RequestCtx) {
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

	t, err := signToken(lifetime, name, "", dummy)
	if err != nil {
		r.Error("can't make token", fasthttp.StatusInternalServerError)
		return
	}

	ctx, _ := context.WithTimeout(context.Background(), lifetime)
	go func() {
		p, err := NewPortal(ctx, hall, dummy, name)
		if err != nil {
			r.Error("can't start portal", fasthttp.StatusInternalServerError)
			return
		}
		r.WriteString(t)

		<-ctx.Done()
		p.Println("portal closed")
	}()
}

func signToken(lifetime time.Duration, uid, name, room string) (token string, err error) {

	canPublish := true
	canSubscribe := true

	at := auth.NewAccessToken(conf.Key, conf.Secret)
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

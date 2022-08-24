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
)

var (
	lkKey, lkSecret, lkWs string
	lkDefaultHall         string
	engineAddress         string

	initialJson defs.Init
)

const (
	lifetime = 2 * time.Hour
)

func Init(key, secret, ws, hall, engineAddress string) (err error) {
	lkKey = key
	lkSecret = secret
	lkWs = ws
	lkDefaultHall = hall
	engineAddress = engineAddress

	var cont []byte
	cont, err = ioutil.ReadFile("./config.json")
	if err != nil {
		return err
	}
	err = json.Unmarshal(cont, &initialJson)
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

	at := auth.NewAccessToken(lkKey, lkSecret)
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

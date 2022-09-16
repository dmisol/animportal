package animportal

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sync/atomic"
	"time"

	"github.com/dmisol/animportal/defs"
	"github.com/google/uuid"
	"github.com/livekit/protocol/auth"
	"github.com/valyala/fasthttp"
	"gopkg.in/yaml.v2"
)

const (
	lifetime = 2 * time.Hour
	initJson = "init.json"
)

type AnimationPortal struct {
	*defs.PortalConf
	index int64
}

func NewPortal(name string) (ap *AnimationPortal, err error) {

	var cont []byte
	cont, err = ioutil.ReadFile(name)
	if err != nil {
		return
	}

	ap = &AnimationPortal{
		PortalConf: &defs.PortalConf{},
	}
	if err = yaml.Unmarshal(cont, ap.PortalConf); err != nil {
		return
	}

	jn := initJson
	if len(ap.PortalConf.DefaultInitJson) > 0 {
		jn = ap.PortalConf.DefaultInitJson
	}
	if cont, err = ioutil.ReadFile(jn); err != nil {
		return
	}
	if err = json.Unmarshal(cont, &ap.PortalConf.InitialJson); err != nil {
		return
	}
	ap.PortalConf.InitialJson.Ftar = ap.PortalConf.DefaultFtar
	return
}

// /animate?name=xxx
// /animate?name=xxx&hall=yyy&ftar=zzz
// if body exists, it contains alternative InitialJson
func (ap *AnimationPortal) Handler(r *fasthttp.RequestCtx) {
	name := string(r.FormValue("name"))
	hall := string(r.FormValue("hall"))
	ftar := string(r.FormValue("ftar"))

	dummy := uuid.NewString()

	if name == "" {
		r.Error("no name", fasthttp.StatusBadRequest)
		return
	}
	if hall == "" {
		hall = "ft"
	}

	conf := *ap.PortalConf

	body := r.Request.Body()
	if len(body) > 0 {
		if err := json.Unmarshal(body, &conf.InitialJson); err != nil {
			r.Error("invalid body - initial json", fasthttp.StatusBadRequest)
			return
		}
	}

	if len(ftar) != 0 {
		conf.InitialJson.Ftar = path.Join(path.Dir(conf.DefaultFtar), ftar)
	}

	x := atomic.AddInt64(&ap.index, 1)
	conf.InitialJson.Dir = fmt.Sprintf("%s/%d", conf.Ram, x)
	if err := os.MkdirAll(conf.InitialJson.Dir, 0777); err != nil {
		r.Error("can't create ramfs folder", fasthttp.StatusInternalServerError)
		return
	}

	t, err := ap.signToken(lifetime, name, "", dummy)
	if err != nil {
		r.Error("can't make token", fasthttp.StatusInternalServerError)
		return
	}

	ctx, _ := context.WithTimeout(context.Background(), lifetime)
	go func() {

		p, err := ap.newUser(ctx, hall, dummy, name, conf)
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

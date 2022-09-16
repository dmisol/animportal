package animportal

import (
	"context"
	"path"
	"testing"
	"time"

	"github.com/dmisol/animportal/dummyclient"
)

func TestPortal(t *testing.T) {

	ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)

	ap, err := NewPortal(path.Join("testdata", "portal.yaml"))

	privateRoom := "test"
	botName := "testbot"

	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	u, err := ap.newUser(ctx, "ft", privateRoom, botName, *ap.PortalConf)
	if err != nil {
		t.Fatalf(err.Error())
		return
	}
	time.Sleep(time.Second)
	go dummyclient.Play(ctx, privateRoom, botName, path.Join("testdata", "audio.ogg"), *ap.PortalConf)

	<-ctx.Done()
	u.Println("portal closed")
}

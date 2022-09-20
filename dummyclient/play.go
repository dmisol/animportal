package dummyclient

import (
	"context"
	"log"
	"time"

	"github.com/dmisol/animportal/defs"
	lksdk "github.com/livekit/server-sdk-go"
)

const (
	lkAudioFrame = 20 * time.Millisecond
)

func Play(ctx context.Context, roomName string, botname string, filename string, c defs.PortalConf) {
	room, err := lksdk.ConnectToRoom(c.Ws, lksdk.ConnectInfo{
		APIKey:              c.Key,
		APISecret:           c.Secret,
		RoomName:            roomName,
		ParticipantIdentity: botname,
		ParticipantName:     botname,
	}, nil)
	if err != nil {
		log.Println("can't connect", room, err)
		return
	}

	if err = publishFile(room, filename, lkAudioFrame); err != nil {
		log.Println("audio pub", err)
		return
	}
	<-ctx.Done()
	room.Disconnect()
}

func publishFile(room *lksdk.Room, filename string, dur time.Duration) error {
	var pub *lksdk.LocalTrackPublication
	opts := []lksdk.ReaderSampleProviderOption{
		lksdk.ReaderTrackWithOnWriteComplete(func() {
			log.Println("finished writing file", filename)
			if pub != nil {
				_ = room.LocalParticipant.UnpublishTrack(pub.SID())
			}
		}),
	}

	opts = append(opts, lksdk.ReaderTrackWithFrameDuration(dur))

	// Create track and publish
	track, err := lksdk.NewLocalFileTrack(filename, opts...)
	if err != nil {
		return err
	}
	if pub, err = room.LocalParticipant.PublishTrack(track, &lksdk.TrackPublicationOptions{
		Name: filename,
	}); err != nil {
		return err
	}
	return nil
}

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"log"
	"net"
	"os"
	"time"

	"github.com/dmisol/animportal/defs"
	"gocv.io/x/gocv"
)

func main() {
	// Listen for incoming connections.
	l, err := net.Listen("tcp", ":50000")
	if err != nil {
		log.Println("istening:", err.Error())
		os.Exit(1)
	}
	// Close the listener when the application closes.
	defer l.Close()
	log.Println("waiting for connections")
	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			log.Println("accepting: ", err.Error())
			os.Exit(1)
		}
		log.Println("accepted", conn.RemoteAddr())
		// Handle connections in a new goroutine.
		go handler(conn)
	}
}

func handler(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 2048)
	// initial json
	if _, err := conn.Read(buf); err != nil {
		log.Println("reading:", err.Error())
		return
	}
	var init *defs.InitialJson

	log.Println("initial json read")

	if err := json.Unmarshal(buf, init); err != nil {
		log.Println("initial json", err)
		return
	}

	log.Println("unmarshalled")

	var started chan bool
	go func() {
		// wait for the first "audio" file
		audio := <-started
		if !audio {
			return
		}

		img := gocv.NewMatWithSize(init.W, init.H, gocv.MatTypeCV8UC3)
		fps := 24
		if init.FPS != 0 {
			fps = init.FPS
		}
		dt := 10000000000 / fps
		t := time.NewTicker(time.Nanosecond * time.Duration(dt))
		defer t.Stop()

		log.Println("image created")
		index := 0
		for {
			<-t.C
			name := fmt.Sprintf("%s/%d.png", init.Dir, index)
			index++

			if err := firePng(conn, &img, name); err != nil {
				log.Println("senging png", err)
				return
			}
			log.Println("file sent", name)
		}
	}()

	running := false
	for {
		i, err := conn.Read(buf)
		if err != nil {
			log.Println("read", err)
			if !running {
				started <- false
			}
			return
		}
		if err = os.Remove(string(buf[:i])); err != nil {
			log.Println("removing", err)
			if !running {
				started <- false
			}
			return
		}
		if !running {
			log.Println("first audio")
			started <- true
			running = true
		}
	}
}

func firePng(c net.Conn, img *gocv.Mat, name string) (err error) {
	gocv.PutText(img, time.Now().String(), image.Point{5, 5}, 0, 5.0, color.RGBA{0, 0, 255, 0}, 2)
	if res := gocv.IMWrite(name, *img); !res {
		err = errors.New("failed to write " + name)
		return
	}
	_, err = c.Write([]byte(name))
	return
}

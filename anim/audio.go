package anim

// #cgo linux CFLAGS: -I/uc/include/opus
// #cgo linux LDFLAGS: -L/uc/lib/x86_64-linux-gnu -lopus
// #include <opus.h>
import "C"
import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"

	"github.com/zaf/resample"
)

const (
	audiochan = 1
	opusRate  = 48000
	voskRate  = 16000
)

var (
	ErrDecoding = errors.New("Error decoding opus")
)

func newConv(dest io.Writer) (c *conv) {
	c = &conv{
		dest: dest,
	}
	var err error
	if c.res, err = resample.New(c.dest, float64(opusRate), float64(voskRate), audiochan, resample.I16, resample.HighQ); err != nil {
		c.Println("resampler creating", err)
	}

	e := C.int(0)
	er := &e
	c.dec = C.opus_decoder_create(C.int(opusRate), C.int(audiochan), er)

	return
}

type conv struct {
	dest io.Writer
	dec  *C.OpusDecoder
	res  *resample.Resampler
	b    []byte
}

func (c *conv) Close() error {
	C.opus_decoder_destroy(c.dec)
	return nil
}

func (c *conv) AppendRTP(rtp *rtp.Packet) (err error) {

	samplesPerFrame := int(C.opus_packet_get_samples_per_frame((*C.uchar)(&rtp.Payload[0]), C.int(48000)))
	pcm := make([]int16, samplesPerFrame)
	samples := C.opus_decode(c.dec, (*C.uchar)(&rtp.Payload[0]), C.opus_int32(len(rtp.Payload)), (*C.opus_int16)(&pcm[0]), C.int(cap(pcm)/audiochan), 0)
	if samples < 0 {
		err = ErrDecoding
		return
	}
	pcmData := make([]byte, 0)
	pcmBuffer := bytes.NewBuffer(pcmData)
	for _, v := range pcm {
		binary.Write(pcmBuffer, binary.LittleEndian, v)
	}
	err = c.AppendBytes(pcmBuffer.Bytes())
	return
}

func (c *conv) AppendBytes(b []byte) (err error) {
	if _, err = c.res.Write(b); err != nil {
		c.Println("resampling", err)
	}
	return
}

func (c *conv) Println(i ...interface{}) {
	log.Println("conv", i)
}

type AudioProc struct {
	mu   sync.Mutex
	fifo []*rtp.Packet
	*conv

	sinceLast int64
	stop      chan bool
}

func NewAudioProc(remote *webrtc.TrackRemote, anim io.Writer) (a *AudioProc) {
	a = &AudioProc{
		stop: make(chan bool),
		conv: newConv(anim),
	}
	go a.run(remote)
	return
}

func (a *AudioProc) Read(p []byte) (i int, err error) {
	// todo: use sync.Cond
	for {
		if x := atomic.LoadInt64(&a.sinceLast); x > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	var totx *rtp.Packet
	func() {
		a.mu.Lock()
		defer a.mu.Unlock()

		totx, a.fifo = a.fifo[0], a.fifo[1:]
	}()

	atomic.AddInt64(&a.sinceLast, -1)
	i = copy(p, totx.Payload)
	return
}

func (a *AudioProc) Close() (err error) {
	a.Println("closing")

	a.stop <- true
	a.conv.Close()

	return
}

func (a *AudioProc) run(remote *webrtc.TrackRemote) {
	for {
		select {
		case <-a.stop:
			a.Println("killed(ctx)")
			return
		default:
			p, _, err := remote.ReadRTP()
			if err != nil {
				a.Println("rtp rd", err)
				return
			}
			func() {
				a.mu.Lock()
				a.mu.Unlock()
				a.fifo = append(a.fifo, p)
			}()
			atomic.AddInt64(&a.sinceLast, 1)
			a.conv.AppendRTP(p)
		}
	}
}

func (a *AudioProc) Println(i ...interface{}) {
	log.Println("audio", i)
}

package relay

import (
	"io"
	"log"
	"sync"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v3"
)

func NewTrackReadCloser(rr RtpReader, mime string) io.ReadCloser {

	trc := TrackReadCloser{rtpReader: rr}
	if mime == webrtc.MimeTypeH264 {
		trc.read = trc.h264Read
		trc.logtype = "h264"
	} else {
		trc.read = trc.opusRead
		trc.logtype = "opus"
	}
	return &trc
}

type TrackReadCloser struct {
	mu sync.Mutex

	rtpReader RtpReader //*webrtc.TrackRemote
	data      []byte
	seq       uint16

	read func(b []byte) (n int, err error)

	logtype string
}

func (trc *TrackReadCloser) Read(b []byte) (n int, err error) {
	return trc.read(b)
}

func (trc *TrackReadCloser) opusRead(b []byte) (n int, err error) {
	trc.mu.Lock()
	defer trc.mu.Unlock()

	var p *rtp.Packet
	if p, _, err = trc.rtpReader.ReadRTP(); err != nil {
		trc.Println("h.264 rtp read err", err)
		return
	}

	// try to fetch audio level
	// todo: re-design?

	ids := p.GetExtensionIDs()
	if len(ids) > 0 {
		trc.Println("ext:", ids, p.GetExtension(ids[0]))
	}
	n = copy(b, p.Payload)
	return
}

func (trc *TrackReadCloser) h264Read(b []byte) (n int, err error) {
	trc.mu.Lock()
	defer trc.mu.Unlock()

	// feed from buffer if non-zero
	dl := len(trc.data)

	if dl == 0 {
		//trc.Println("fetching rtp")
		pkt := codecs.H264Packet{}
		for {
			var p *rtp.Packet
			if p, _, err = trc.rtpReader.ReadRTP(); err != nil {
				trc.Println("rtp read err", err)
				return
			}
			// todo: process sseq mismatch
			if trc.seq != p.SequenceNumber {
				trc.Println("seq mismatch, expected", trc.seq, "got", p.SequenceNumber)
			}
			//trc.Println("rtp", p.SequenceNumber, len(p.Payload), p.Payload[0]&0x1f, p.Marker)

			b := p.Payload[0]
			if b&0x80 != 0 {
				trc.Println("forbidden bit", b)
			} else {
				nri := (b >> 5) & 3
				typ := b & 0x1f

				if nri == 0 && len(p.Payload) <= 2 {
					continue
				}

				//if nri!=2 || typ != 28 {
				log.Println("nri/typ/p.len", nri, typ, len(p.Payload), p.Marker)
				//}
			}

			trc.seq = p.SequenceNumber + 1

			var v []byte
			if v, err = pkt.Unmarshal(p.Payload); err != nil {
				trc.Println("payload unmarshal err", err)
				//trc.Println(p.String(), p.Payload)
				//return
			}
			if len(v) > 0 {
				trc.data = append(trc.data, v...)
				//dl = len(trc.data)
				//trc.Println("debug data:", len(v), dl, v[:10])
			}
			if p.Marker {
				break
			}
		}
	}

	n = copy(b, trc.data)
	trc.data = trc.data[n:]
	//trc.Println("feed", n, len(trc.data))
	return

}

func (trc *TrackReadCloser) Close() (err error) {
	return
}

func (trc *TrackReadCloser) Println(i ...interface{}) {
	log.Println("trc", trc.logtype, i)
}

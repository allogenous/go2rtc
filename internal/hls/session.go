package hls

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/AlexxIT/go2rtc/pkg/mp4"
)

type Session struct {
	cons     core.Consumer
	id       string
	template string
	init     []byte
	buffer   []byte
	seq      int
	alive    *time.Timer
	mu       sync.Mutex
	// extraQuery 附加到 HLS 分片 URL（如 &token=...）
	extraQuery string
}

func NewSession(cons core.Consumer, extraQuery string) *Session {
	s := &Session{
		id:         core.RandString(8, 62),
		cons:       cons,
		extraQuery: extraQuery,
	}

	// two segments important for Chromecast
	if _, ok := cons.(*mp4.Consumer); ok {
		s.template = `#EXTM3U
#EXT-X-VERSION:6
#EXT-X-TARGETDURATION:1
#EXT-X-MEDIA-SEQUENCE:%d
#EXT-X-MAP:URI="init.mp4?id=` + s.id + `"
#EXTINF:0.500,
segment.m4s?id=` + s.id + `&n=%d
#EXTINF:0.500,
segment.m4s?id=` + s.id + `&n=%d`
	} else {
		s.template = `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:1
#EXT-X-MEDIA-SEQUENCE:%d
#EXTINF:0.500,
segment.ts?id=` + s.id + `&n=%d
#EXTINF:0.500,
segment.ts?id=` + s.id + `&n=%d`
	}

	return s
}

func (s *Session) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	if s.init == nil {
		s.init = p
	} else {
		s.buffer = append(s.buffer, p...)
	}
	s.mu.Unlock()
	return len(p), nil
}

func (s *Session) Run() {
	_, _ = s.cons.(io.WriterTo).WriteTo(s)
}

func (s *Session) Main() []byte {
	type withCodecs interface {
		Codecs() []*core.Codec
	}

	codecs := mp4.MimeCodecs(s.cons.(withCodecs).Codecs())
	codecs = strings.Replace(codecs, mp4.MimeFlac, "fLaC", 1)

	// bandwidth important for Safari, codecs useful for smooth playback
	return []byte(`#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=192000,CODECS="` + codecs + `"
hls/playlist.m3u8?id=` + s.id + s.extraQuery)
}

func (s *Session) Playlist() []byte {
	body := fmt.Sprintf(s.template, s.seq, s.seq, s.seq+1)
	if s.extraQuery != "" {
		body = strings.ReplaceAll(body, "?id="+s.id, "?id="+s.id+s.extraQuery)
	}
	return []byte(body)
}

func (s *Session) Init() (init []byte) {
	for i := 0; i < 60 && init == nil; i++ {
		if i > 0 {
			time.Sleep(50 * time.Millisecond)
		}

		s.mu.Lock()
		// return init only when have some buffer
		if len(s.buffer) > 0 {
			init = s.init
		}
		s.mu.Unlock()
	}

	return
}

func (s *Session) Segment() (segment []byte) {
	for i := 0; i < 60 && segment == nil; i++ {
		if i > 0 {
			time.Sleep(50 * time.Millisecond)
		}

		s.mu.Lock()
		if len(s.buffer) > 0 {
			segment = s.buffer
			if _, ok := s.cons.(*mp4.Consumer); ok {
				s.buffer = nil
			} else {
				// for TS important to start new segment with init
				s.buffer = s.init
			}
			s.seq++
		}
		s.mu.Unlock()
	}

	return
}

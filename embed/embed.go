// Package embed 将 go2rtc 作为库嵌入宿主 HTTP 服务：注册 HLS 路由、按需拉流。
package embed

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/AlexxIT/go2rtc/internal/api"
	"github.com/AlexxIT/go2rtc/internal/app"
	"github.com/AlexxIT/go2rtc/internal/exec"
	"github.com/AlexxIT/go2rtc/internal/ffmpeg"
	"github.com/AlexxIT/go2rtc/internal/hls"
	g2rhttp "github.com/AlexxIT/go2rtc/internal/http"
	"github.com/AlexxIT/go2rtc/internal/mp4"
	"github.com/AlexxIT/go2rtc/internal/rtmp"
	"github.com/AlexxIT/go2rtc/internal/rtsp"
	"github.com/AlexxIT/go2rtc/internal/streams"
)

// Config 嵌入式 go2rtc 初始化参数。
type Config struct {
	// Mux 宿主 ServeMux；路由以 PathPrefix 为前缀注册。
	Mux *http.ServeMux
	// PathPrefix 如 /ipcgateway（无尾斜杠）
	PathPrefix string
	// RTSPListen ffmpeg copy 管线所需的本机 RTSP 回环地址，如 127.0.0.1:18554
	RTSPListen string
	// FFmpegBin ffmpeg 可执行文件，空则使用 "ffmpeg"
	FFmpegBin string
}

// Init 初始化嵌入式 go2rtc（不独立监听 HTTP）。须在注册其它 go2rtc 路由前调用一次。
func Init(cfg Config) error {
	if cfg.Mux == nil {
		return fmt.Errorf("embed: mux is nil")
	}
	prefix := strings.TrimRight(cfg.PathPrefix, "/")
	if prefix == "" {
		prefix = "/ipcgateway"
	}
	rtspListen := strings.TrimSpace(cfg.RTSPListen)
	if rtspListen == "" {
		rtspListen = "127.0.0.1:18554"
	}
	ffmpegBin := strings.TrimSpace(cfg.FFmpegBin)
	if ffmpegBin == "" {
		ffmpegBin = "ffmpeg"
	}

	allow := []string{
		prefix + "/api/stream.m3u8",
		prefix + "/api/stream.mp4",
		prefix + "/api/hls/playlist.m3u8",
		prefix + "/api/hls/init.mp4",
		prefix + "/api/hls/segment.m4s",
		prefix + "/api/hls/segment.ts",
	}

	yaml := fmt.Sprintf(`
api:
  listen: ""
  origin: "*"
  base_path: %q
  allow_paths:
    - %s
    - %s
    - %s
    - %s
    - %s
    - %s
rtsp:
  listen: %q
rtmp:
  listen: ""
ffmpeg:
  bin: %q
log:
  level: info
  api: error
  streams: info
  hls: info
  ffmpeg: error
  exec: error
`, prefix, allow[0], allow[1], allow[2], allow[3], allow[4], allow[5], rtspListen, ffmpegBin)

	app.AppendConfigYAML([]byte(yaml))
	api.SetEmbedMux(cfg.Mux)
	app.InitEmbedded()
	api.Init()
	streams.Init()
	g2rhttp.Init()
	rtsp.Init()
	rtmp.Init()
	ffmpeg.Init()
	exec.Init()
	mp4.Init()
	hls.Init()
	return nil
}

// PatchStream 按名称注册或更新源（懒拉流，不立即 Dial）。
func PatchStream(name, source string) error {
	_, err := streams.Patch(name, source)
	return err
}

// GetStream 返回已注册流；不存在则为 nil。
func GetStream(name string) *streams.Stream {
	return streams.Get(name)
}

// DeleteStream 删除命名流。
func DeleteStream(name string) {
	streams.Delete(name)
}

// ConsumerCount 返回流上的播放消费者数量。
func ConsumerCount(name string) int {
	s := streams.Get(name)
	if s == nil {
		return 0
	}
	return s.ConsumerCount()
}

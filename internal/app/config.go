package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/AlexxIT/go2rtc/pkg/creds"
	"github.com/AlexxIT/go2rtc/pkg/yaml"
)

func LoadConfig(v any) {
	for _, data := range configs {
		if err := yaml.Unmarshal(data, v); err != nil {
			Logger.Warn().Err(err).Send()
		}
	}
}

var configMu sync.Mutex

func PatchConfig(path []string, value any) error {
	if ConfigPath == "" {
		return errors.New("config file disabled")
	}

	configMu.Lock()
	defer configMu.Unlock()

	// empty config is OK
	b, _ := os.ReadFile(ConfigPath)

	b, err := yaml.Patch(b, path, value)
	if err != nil {
		return err
	}

	return os.WriteFile(ConfigPath, b, 0644)
}

type flagConfig []string

func (c *flagConfig) String() string {
	return strings.Join(*c, " ")
}

func (c *flagConfig) Set(value string) error {
	*c = append(*c, value)
	return nil
}

var configs [][]byte

// AppendConfigYAML 追加嵌入式 YAML 配置（在 Init / InitEmbedded 加载模块前调用）。
func AppendConfigYAML(data []byte) {
	if len(data) == 0 {
		return
	}
	configs = append(configs, data)
}

// InitEmbedded 供宿主进程库嵌入初始化：不解析命令行 flag，不监听独立 HTTP 端口。
func InitEmbedded() {
	if Version == "" {
		Version = "1.9.14"
	}

	revision, vcsTime := readRevisionTime()

	UserAgent = "go2rtc/" + Version

	Info["version"] = Version
	Info["revision"] = revision

	initLogger()

	platform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	Logger.Info().Str("version", Version).Str("platform", platform).Str("revision", revision).Msg("go2rtc embedded")
	Logger.Debug().Str("version", runtime.Version()).Str("vcs.time", vcsTime).Msg("build")

	var cfg struct {
		Mod struct {
			Modules []string `yaml:"modules"`
		} `yaml:"app"`
	}

	LoadConfig(&cfg)

	Modules = cfg.Mod.Modules
}

func initConfig(confs flagConfig) {
	if confs == nil {
		confs = []string{"go2rtc.yaml"}
	}

	for _, conf := range confs {
		if len(conf) == 0 {
			continue
		}
		if conf[0] == '{' {
			// config as raw YAML or JSON
			configs = append(configs, []byte(conf))
		} else if data := parseConfString(conf); data != nil {
			configs = append(configs, data)
		} else {
			// config as file
			if ConfigPath == "" {
				ConfigPath = conf
				initStorage()
			}

			if data, _ = os.ReadFile(conf); data == nil {
				continue
			}

			loadEnv(data)
			data = creds.ReplaceVars(data)
			configs = append(configs, data)
		}
	}

	if ConfigPath != "" {
		if !filepath.IsAbs(ConfigPath) {
			if cwd, err := os.Getwd(); err == nil {
				ConfigPath = filepath.Join(cwd, ConfigPath)
			}
		}
		Info["config_path"] = ConfigPath
	}
}

func parseConfString(s string) []byte {
	i := strings.IndexByte(s, '=')
	if i < 0 {
		return nil
	}

	items := strings.Split(s[:i], ".")
	if len(items) < 2 {
		return nil
	}

	// `log.level=trace` => `{log: {level: trace}}`
	var pre string
	var suf = s[i+1:]
	for _, item := range items {
		pre += "{" + item + ": "
		suf += "}"
	}

	return []byte(pre + suf)
}

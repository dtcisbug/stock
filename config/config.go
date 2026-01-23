package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// YAMLConfig YAML配置文件结构
type YAMLConfig struct {
	API struct {
		Token   string `yaml:"token"`
		BaseURL string `yaml:"base_url"`
		Model   string `yaml:"model"`
	} `yaml:"api"`

	Monitor struct {
		Stocks  []string `yaml:"stocks"`
		Futures []string `yaml:"futures"`
	} `yaml:"monitor"`

	Server struct {
		Port         int  `yaml:"port"`
		EnableAI     bool `yaml:"enable_ai"`
		SyncInterval int  `yaml:"sync_interval"`
	} `yaml:"server"`
}

// Config 配置
type Config struct {
	// HTTP 服务端口
	Port int

	// 数据刷新间隔(交易时间内)
	RefreshInterval time.Duration

	// 检查交易时间间隔(非交易时间)
	CheckInterval time.Duration

	// AI分析间隔(交易时间内)
	AnalysisInterval time.Duration

	// Claude API Key
	ClaudeAPIKey string

	// Claude API Base URL (支持代理)
	ClaudeAPIBase string

	// Claude Model
	ClaudeModel string

	// 监控的股票列表
	Stocks []string

	// 监控的期货列表
	Futures []string

	// 是否启用AI分析
	EnableAI bool
}

// DefaultConfig 默认配置
var DefaultConfig = Config{
	Port:             19527,
	RefreshInterval:  3 * time.Second,
	CheckInterval:    1 * time.Minute,
	AnalysisInterval: 1 * time.Hour,
	ClaudeAPIKey:     "",
	ClaudeAPIBase:    "https://api.anthropic.com",
	ClaudeModel:      "claude-sonnet-4-5-20250929",
	EnableAI:         true,
	Stocks: []string{
		"sz002415", // 海康威视
		"sh513130", // 恒生科技ETF
		"sh600362", // 江西铜业
	},
	Futures: []string{
		"nf_I0",  // 铁矿石主连
		"nf_B0",  // 豆二主连
		"nf_MA0", // 甲醇主连
		"nf_UR0", // 尿素主连
		"nf_EB0", // 苯乙烯主连
	},
}

var futuresShortCodeRe = regexp.MustCompile(`^([A-Za-z]+)([0-9]+)$`)

func normalizeFuturesCode(code string) string {
	c := strings.TrimSpace(code)
	if c == "" {
		return c
	}

	// 统一 nf_ 前缀（大小写不敏感）
	if len(c) >= 3 && strings.EqualFold(c[:3], "nf_") {
		rest := strings.TrimSpace(c[3:])
		if rest == "" {
			return "nf_"
		}
		if m := futuresShortCodeRe.FindStringSubmatch(rest); len(m) == 3 {
			return "nf_" + strings.ToUpper(m[1]) + m[2]
		}
		return "nf_" + rest
	}

	// 支持简写：pp2605 -> nf_PP2605，AU0 -> nf_AU0
	if m := futuresShortCodeRe.FindStringSubmatch(c); len(m) == 3 {
		return "nf_" + strings.ToUpper(m[1]) + m[2]
	}

	return c
}

func normalizeFuturesCodes(codes []string) []string {
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		out = append(out, normalizeFuturesCode(code))
	}
	return out
}

// LoadFromFile 从YAML文件加载配置
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var yamlConfig YAMLConfig
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 从YAML配置转换为Config
	config := DefaultConfig

	// API配置
	if yamlConfig.API.Token != "" {
		config.ClaudeAPIKey = yamlConfig.API.Token
	}
	if yamlConfig.API.BaseURL != "" {
		config.ClaudeAPIBase = yamlConfig.API.BaseURL
	}
	if yamlConfig.API.Model != "" {
		config.ClaudeModel = yamlConfig.API.Model
	}

	// 监控配置
	if len(yamlConfig.Monitor.Stocks) > 0 {
		config.Stocks = yamlConfig.Monitor.Stocks
	}
	if len(yamlConfig.Monitor.Futures) > 0 {
		config.Futures = normalizeFuturesCodes(yamlConfig.Monitor.Futures)
	}

	// 服务配置
	if yamlConfig.Server.Port > 0 {
		config.Port = yamlConfig.Server.Port
	}
	config.EnableAI = yamlConfig.Server.EnableAI
	if yamlConfig.Server.SyncInterval > 0 {
		config.RefreshInterval = time.Duration(yamlConfig.Server.SyncInterval) * time.Second
	}

	return &config, nil
}

// GetConfig 获取配置 (优先级: 配置文件 > 环境变量 > 默认值)
func GetConfig(configPath string) *Config {
	config := DefaultConfig

	// 尝试从配置文件加载
	if configPath != "" {
		if cfg, err := LoadFromFile(configPath); err == nil {
			config = *cfg
		} else {
			fmt.Printf("警告: 无法加载配置文件 %s: %v\n", configPath, err)
		}
	}

	// 环境变量覆盖配置文件 (向后兼容)
	if key := getAPIKey(); key != "" {
		config.ClaudeAPIKey = key
	}
	if url := getAPIBaseURL(); url != "" {
		config.ClaudeAPIBase = url
	}
	if model := getModel(); model != "" {
		config.ClaudeModel = model
	}

	// 统一期货代码格式，兼容简写（如 pp2605 / AU0）
	config.Futures = normalizeFuturesCodes(config.Futures)

	return &config
}

// getAPIKey 获取 API Key
func getAPIKey() string {
	// 优先使用 AUTH_TOKEN(代理服务常用)
	if key := os.Getenv("ANTHROPIC_AUTH_TOKEN"); key != "" {
		return key
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return key
	}
	return os.Getenv("CLAUDE_API_KEY")
}

// getAPIBaseURL 获取 API Base URL,默认官方地址
func getAPIBaseURL() string {
	if url := os.Getenv("ANTHROPIC_BASE_URL"); url != "" {
		return url
	}
	if url := os.Getenv("ANTHROPIC_API_BASE"); url != "" {
		return url
	}
	return ""
}

// getModel 获取模型名称
func getModel() string {
	if model := os.Getenv("ANTHROPIC_MODEL"); model != "" {
		return model
	}
	return ""
}

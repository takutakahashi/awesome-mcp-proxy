package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"github.com/takutakahashi/awesome-mcp-proxy/config"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to configuration file")
	flag.Parse()

	// 設定を読み込み
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// JSON形式で出力
	output, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal config: %v", err)
	}

	fmt.Println(string(output))
}
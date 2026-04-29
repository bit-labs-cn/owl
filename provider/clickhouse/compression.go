package clickhouse

import (
	"fmt"
	"strings"

	ch "github.com/ClickHouse/clickhouse-go/v2"
)

func compressionFromConfig(cfg CompressionConfig) (*ch.Compression, error) {
	method := strings.ToLower(strings.TrimSpace(cfg.Method))
	if method == "" || method == "none" {
		return nil, nil
	}

	cm, err := parseCompressionMethod(method)
	if err != nil {
		return nil, err
	}

	comp := &ch.Compression{Method: cm}
	switch cm {
	case ch.CompressionLZ4, ch.CompressionLZ4HC, ch.CompressionGZIP, ch.CompressionDeflate, ch.CompressionBrotli:
		if cfg.Level != 0 {
			comp.Level = cfg.Level
		}
	case ch.CompressionZSTD:
		// zstd：Level 在驱动侧通常忽略，保留 Method 即可
		if cfg.Level != 0 {
			comp.Level = cfg.Level
		}
	default:
		if cfg.Level != 0 {
			return nil, fmt.Errorf("clickhouse: 压缩算法 %s 不支持配置 level", method)
		}
	}
	return comp, nil
}

func parseCompressionMethod(s string) (ch.CompressionMethod, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "none", "":
		return ch.CompressionNone, nil
	case "lz4":
		return ch.CompressionLZ4, nil
	case "lz4hc":
		return ch.CompressionLZ4HC, nil
	case "zstd":
		return ch.CompressionZSTD, nil
	case "gzip":
		return ch.CompressionGZIP, nil
	case "deflate":
		return ch.CompressionDeflate, nil
	case "br":
		return ch.CompressionBrotli, nil
	default:
		return 0, fmt.Errorf("clickhouse: 未知 compression.method %q（支持 none/lz4/lz4hc/zstd/gzip/deflate/br）", s)
	}
}

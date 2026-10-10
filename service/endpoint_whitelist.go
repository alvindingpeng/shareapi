// Package service: official endpoint whitelist for ShareLLM-style
// base-URL certification (P7-4).
//
// ShareLLM certifies channels whose traffic terminates at official upstream
// endpoints ("base-URL 认证池"). A channel claiming to serve an official
// model but pointing at an arbitrary third-party host is a shell risk
// (套壳) and cannot be certified.
package service

import (
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

// officialEndpointHosts maps channel type to the official upstream hosts.
// A channel whose base URL host is in this list terminates at the official
// endpoint and is eligible for certification.
var officialEndpointHosts = map[int][]string{
	constant.ChannelTypeOpenAI: {
		"api.openai.com",
	},
	constant.ChannelTypeAnthropic: {
		"api.anthropic.com",
	},
	constant.ChannelTypeGemini: {
		"generativelanguage.googleapis.com",
	},
	constant.ChannelTypeVertexAi: {
		"us-central1-aiplatform.googleapis.com",
		"aiplatform.googleapis.com",
	},
	constant.ChannelTypeXai: {
		"api.x.ai",
	},
	constant.ChannelTypeDeepSeek: {
		"api.deepseek.com",
	},
	constant.ChannelTypeMoonshot: {
		"api.moonshot.cn",
		"api.moonshot.ai",
	},
	constant.ChannelTypeMistral: {
		"api.mistral.ai",
	},
	constant.ChannelTypeCohere: {
		"api.cohere.ai",
	},
	constant.ChannelTypePerplexity: {
		"api.perplexity.ai",
	},
	constant.ChannelTypeZhipu: {
		"open.bigmodel.cn",
	},
	constant.ChannelTypeZhipu_v4: {
		"open.bigmodel.cn",
	},
	constant.ChannelTypeAli: {
		"dashscope.aliyuncs.com",
	},
	constant.ChannelTypeBaidu: {
		"aip.baidubce.com",
	},
	constant.ChannelTypeBaiduV2: {
		"qianfan.baidubce.com",
	},
	constant.ChannelTypeTencent: {
		"hunyuan.cloud.tencent.com",
	},
	constant.ChannelTypeXunfei: {
		"spark-api-open.xf-yun.com",
	},
	constant.ChannelTypeVolcEngine: {
		"ark.cn-beijing.volces.com",
	},
	constant.ChannelTypeMiniMax: {
		"api.minimax.chat",
	},
	constant.ChannelTypeSiliconFlow: {
		"api.siliconflow.cn",
	},
	constant.ChannelTypeLingYiWanWu: {
		"api.lingyiwanwu.com",
	},
	constant.ChannelTypeAws: {
		"bedrock-runtime.us-east-1.amazonaws.com",
		"bedrock-runtime.us-west-2.amazonaws.com",
	},
	constant.ChannelTypeAzure: {
		"openai.azure.com",
	},
	constant.ChannelTypeOllama: {
		"localhost",
		"127.0.0.1",
	},
	constant.ChannelTypeCodex: {
		"chatgpt.com",
		"api.openai.com",
	},
}

// IsOfficialEndpoint reports whether the given base URL terminates at an
// official upstream endpoint for the channel type. Types without a
// whitelist entry return (false, false): unknown, not certified.
// The second return value reports whether a whitelist exists for the type.
func IsOfficialEndpoint(channelType int, baseURL string) (official bool, known bool) {
	hosts, ok := officialEndpointHosts[channelType]
	if !ok {
		return false, false
	}
	host := extractHost(baseURL)
	if host == "" {
		return false, true
	}
	host = strings.ToLower(host)
	// Strip port if present.
	if i := strings.LastIndex(host, ":"); i > 0 && !strings.Contains(host[i+1:], "]") {
		host = host[:i]
	}
	for _, h := range hosts {
		if host == strings.ToLower(h) {
			return true, true
		}
	}
	return false, true
}

func extractHost(rawURL string) string {
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

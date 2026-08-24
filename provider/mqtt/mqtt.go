package mqtt

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// ErrConnecting 表示首次 Connect 超时，但 ConnectRetry 仍在后台重试。
// 调用方必须保留客户端，不能丢弃；后续 OnConnect / IsConnected 会继续推进。
var ErrConnecting = errors.New("mqtt connection retrying")

// Options MQTT 配置选项
type Options struct {
	Broker       string             `json:"broker" mapstructure:"broker" yaml:"broker"`
	Client       ClientConfig       `json:"client" mapstructure:"client" yaml:"client"`
	TLS          TLSConfig          `json:"tls" mapstructure:"tls" yaml:"tls"`
	Message      MessageConfig      `json:"message" mapstructure:"message" yaml:"message"`
	Subscription SubscriptionConfig `json:"subscription" mapstructure:"subscription" yaml:"subscription"`
	Publish      PublishConfig      `json:"publish" mapstructure:"publish" yaml:"publish"`
	Logging      LoggingConfig      `json:"logging" mapstructure:"logging" yaml:"logging"`
	Buffer       BufferConfig       `json:"buffer" mapstructure:"buffer" yaml:"buffer"`
	Performance  PerformanceConfig  `json:"performance" mapstructure:"performance" yaml:"performance"`
}

// ClientConfig 客户端配置
type ClientConfig struct {
	ID                   string `json:"id" mapstructure:"id" yaml:"id"`
	Username             string `json:"username" mapstructure:"username" yaml:"username"`
	Password             string `json:"password" mapstructure:"password" yaml:"password"`
	CleanSession         bool   `json:"clean-session" mapstructure:"clean-session" yaml:"clean-session"`
	KeepAlive            int    `json:"keep-alive" mapstructure:"keep-alive" yaml:"keep-alive"`
	PingTimeout          int    `json:"ping-timeout" mapstructure:"ping-timeout" yaml:"ping-timeout"`
	ConnectTimeout       int    `json:"connect-timeout" mapstructure:"connect-timeout" yaml:"connect-timeout"`
	AutoReconnect        bool   `json:"auto-reconnect" mapstructure:"auto-reconnect" yaml:"auto-reconnect"`
	ReconnectInterval    int    `json:"reconnect-interval" mapstructure:"reconnect-interval" yaml:"reconnect-interval"`
	MaxReconnectInterval int    `json:"max-reconnect-interval" mapstructure:"max-reconnect-interval" yaml:"max-reconnect-interval"`
	ConnectRetry         bool   `json:"connect-retry" mapstructure:"connect-retry" yaml:"connect-retry"`
	ResumeSubs           bool   `json:"resume-subs" mapstructure:"resume-subs" yaml:"resume-subs"`
	MaxReconnectAttempts int    `json:"max-reconnect-attempts" mapstructure:"max-reconnect-attempts" yaml:"max-reconnect-attempts"`
}

// TLSConfig TLS/SSL 配置
type TLSConfig struct {
	Enabled            bool   `json:"enabled" mapstructure:"enabled" yaml:"enabled"`
	CertFile           string `json:"cert-file" mapstructure:"cert-file" yaml:"cert-file"`
	KeyFile            string `json:"key-file" mapstructure:"key-file" yaml:"key-file"`
	CAFile             string `json:"ca-file" mapstructure:"ca-file" yaml:"ca-file"`
	InsecureSkipVerify bool   `json:"insecure-skip-verify" mapstructure:"insecure-skip-verify" yaml:"insecure-skip-verify"`
}

// MessageConfig 消息配置
type MessageConfig struct {
	DefaultQoS byte `json:"default-qos" mapstructure:"default-qos" yaml:"default-qos"`
	Retain     bool `json:"retain" mapstructure:"retain" yaml:"retain"`
	Timeout    int  `json:"timeout" mapstructure:"timeout" yaml:"timeout"`
}

// SubscriptionConfig 订阅配置
type SubscriptionConfig struct {
	DefaultQoS byte `json:"default-qos" mapstructure:"default-qos" yaml:"default-qos"`
	Timeout    int  `json:"timeout" mapstructure:"timeout" yaml:"timeout"`
}

// PublishConfig 发布配置
type PublishConfig struct {
	DefaultQoS byte `json:"default-qos" mapstructure:"default-qos" yaml:"default-qos"`
	Timeout    int  `json:"timeout" mapstructure:"timeout" yaml:"timeout"`
	WaitForAck bool `json:"wait-for-ack" mapstructure:"wait-for-ack" yaml:"wait-for-ack"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Debug               bool `json:"debug" mapstructure:"debug" yaml:"debug"`
	LogConnectionEvents bool `json:"log-connection-events" mapstructure:"log-connection-events" yaml:"log-connection-events"`
	LogMessageEvents    bool `json:"log-message-events" mapstructure:"log-message-events" yaml:"log-message-events"`
}

// BufferConfig 缓冲区配置
type BufferConfig struct {
	SendBufferSize    int `json:"send-buffer-size" mapstructure:"send-buffer-size" yaml:"send-buffer-size"`
	ReceiveBufferSize int `json:"receive-buffer-size" mapstructure:"receive-buffer-size" yaml:"receive-buffer-size"`
	MessageQueueSize  int `json:"message-queue-size" mapstructure:"message-queue-size" yaml:"message-queue-size"`
}

// PerformanceConfig 性能配置
type PerformanceConfig struct {
	MaxConcurrentConnections int `json:"max-concurrent-connections" mapstructure:"max-concurrent-connections" yaml:"max-concurrent-connections"`
	MessageHandlerCount      int `json:"message-handler-count" mapstructure:"message-handler-count" yaml:"message-handler-count"`
	BatchSize                int `json:"batch-size" mapstructure:"batch-size" yaml:"batch-size"`
}

type subscriptionEntry struct {
	qos      byte
	callback mqtt.MessageHandler
}

// MQTTClient MQTT 客户端包装器
type MQTTClient struct {
	client  mqtt.Client
	options *Options
	ctx     context.Context
	cancel  context.CancelFunc

	mu   sync.RWMutex
	subs map[string]subscriptionEntry
}

// InitMQTT 初始化 MQTT 客户端
func InitMQTT(opt *Options) *MQTTClient {
	setDefaults(opt)

	clientID := opt.Client.ID
	if clientID == "" {
		clientID = fmt.Sprintf("mqtt-client-%d", time.Now().UnixNano())
		opt.Client.ID = clientID
	}

	m := &MQTTClient{
		options: opt,
		subs:    make(map[string]subscriptionEntry),
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	m.cancel = cancel

	opts := buildClientOptions(opt, m)
	m.client = mqtt.NewClient(opts)
	return m
}

// buildClientOptions 构建 Paho 客户端选项（导出给单测断言）
func buildClientOptions(opt *Options, m *MQTTClient) *mqtt.ClientOptions {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(opt.Broker)
	opts.SetClientID(opt.Client.ID)

	if opt.Client.Username != "" {
		opts.SetUsername(opt.Client.Username)
	}
	if opt.Client.Password != "" {
		opts.SetPassword(opt.Client.Password)
	}

	opts.SetCleanSession(opt.Client.CleanSession)
	opts.SetKeepAlive(time.Duration(opt.Client.KeepAlive) * time.Second)
	opts.SetPingTimeout(time.Duration(opt.Client.PingTimeout) * time.Second)
	opts.SetConnectTimeout(time.Duration(opt.Client.ConnectTimeout) * time.Second)
	opts.SetAutoReconnect(opt.Client.AutoReconnect)
	opts.SetResumeSubs(opt.Client.ResumeSubs)
	opts.SetConnectRetry(opt.Client.ConnectRetry)
	opts.SetConnectRetryInterval(time.Duration(opt.Client.ReconnectInterval) * time.Second)
	opts.SetMaxReconnectInterval(time.Duration(opt.Client.MaxReconnectInterval) * time.Second)
	opts.SetOrderMatters(false)

	if opt.TLS.Enabled {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: opt.TLS.InsecureSkipVerify,
		}
		if opt.TLS.CertFile != "" && opt.TLS.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(opt.TLS.CertFile, opt.TLS.KeyFile)
			if err != nil {
				log.Printf("[MQTT] client=%s broker=%s load TLS certificate failed: %v", opt.Client.ID, opt.Broker, err)
			} else {
				tlsConfig.Certificates = []tls.Certificate{cert}
			}
		}
		opts.SetTLSConfig(tlsConfig)
	}

	if opt.Logging.Debug {
		mqtt.DEBUG = log.New(log.Writer(), "[MQTT-DEBUG] ", log.LstdFlags)
		mqtt.WARN = log.New(log.Writer(), "[MQTT-WARN] ", log.LstdFlags)
		mqtt.CRITICAL = log.New(log.Writer(), "[MQTT-CRITICAL] ", log.LstdFlags)
		mqtt.ERROR = log.New(log.Writer(), "[MQTT-ERROR] ", log.LstdFlags)
	}

	clientID := opt.Client.ID
	broker := opt.Broker
	logConn := opt.Logging.LogConnectionEvents

	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		if logConn {
			log.Printf("[MQTT] connection lost client=%s broker=%s err=%v", clientID, broker, err)
		}
	})
	opts.SetReconnectingHandler(func(_ mqtt.Client, _ *mqtt.ClientOptions) {
		if logConn {
			log.Printf("[MQTT] reconnecting client=%s broker=%s", clientID, broker)
		}
	})
	opts.SetOnConnectHandler(func(cli mqtt.Client) {
		if m != nil {
			m.onConnect(cli)
			return
		}
		if logConn {
			log.Printf("[MQTT] connected client=%s broker=%s", clientID, broker)
		}
	})

	return opts
}

// onConnect 在独立协程里重放订阅。Paho 禁止在 callback 内 token.Wait / WaitTimeout。
func (m *MQTTClient) onConnect(cli mqtt.Client) {
	if m.options != nil && m.options.Logging.LogConnectionEvents {
		log.Printf("[MQTT] connected client=%s broker=%s", m.options.Client.ID, m.options.Broker)
	}
	go m.resubscribeAll(cli)
}

func (m *MQTTClient) resubscribeAll(cli mqtt.Client) {
	if m.ctx != nil {
		select {
		case <-m.ctx.Done():
			return
		default:
		}
	}

	m.mu.RLock()
	snapshot := make(map[string]subscriptionEntry, len(m.subs))
	for topic, entry := range m.subs {
		snapshot[topic] = entry
	}
	m.mu.RUnlock()

	if len(snapshot) == 0 {
		return
	}

	timeout := time.Duration(m.options.Subscription.Timeout) * time.Second
	for topic, entry := range snapshot {
		if m.ctx != nil {
			select {
			case <-m.ctx.Done():
				return
			default:
			}
		}
		token := cli.Subscribe(topic, entry.qos, entry.callback)
		ok := token.WaitTimeout(timeout)
		if !ok {
			log.Printf("[MQTT] re-subscribe timeout client=%s broker=%s topic=%s", m.options.Client.ID, m.options.Broker, topic)
			continue
		}
		if err := token.Error(); err != nil {
			log.Printf("[MQTT] re-subscribe failed client=%s broker=%s topic=%s err=%v", m.options.Client.ID, m.options.Broker, topic, err)
			continue
		}
		if m.options.Logging.LogConnectionEvents {
			log.Printf("[MQTT] re-subscribed client=%s broker=%s topic=%s qos=%d", m.options.Client.ID, m.options.Broker, topic, entry.qos)
		}
	}
}

// Connect 连接到 MQTT 服务器。
// ConnectRetry 开启时，超时不丢弃客户端，返回 ErrConnecting 表示后台仍在重试。
func (m *MQTTClient) Connect() error {
	token := m.client.Connect()
	if !token.WaitTimeout(time.Duration(m.options.Client.ConnectTimeout) * time.Second) {
		if m.options.Client.ConnectRetry {
			return fmt.Errorf("%w: broker %s", ErrConnecting, m.options.Broker)
		}
		return fmt.Errorf("failed to connect to MQTT broker %s: timeout", m.options.Broker)
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("failed to connect to MQTT broker %s: %w", m.options.Broker, err)
	}
	return nil
}

// Disconnect 断开连接
func (m *MQTTClient) Disconnect() {
	m.cancel()
	m.client.Disconnect(250)
}

// Publish 发布消息
func (m *MQTTClient) Publish(topic string, payload interface{}) error {
	return m.PublishWithQoS(topic, payload, m.options.Publish.DefaultQoS)
}

// PublishWithQoS 使用指定 QoS 发布消息
func (m *MQTTClient) PublishWithQoS(topic string, payload interface{}, qos byte) error {
	token := m.client.Publish(topic, qos, m.options.Message.Retain, payload)

	if m.options.Publish.WaitForAck {
		ok := token.WaitTimeout(time.Duration(m.options.Publish.Timeout) * time.Second)
		if !ok {
			return fmt.Errorf("failed to publish message to %s: timeout", topic)
		}
		if err := token.Error(); err != nil {
			return fmt.Errorf("failed to publish message to %s: %w", topic, err)
		}
	}

	return nil
}

// Subscribe 订阅主题
func (m *MQTTClient) Subscribe(topic string, callback mqtt.MessageHandler) error {
	return m.SubscribeWithQoS(topic, m.options.Subscription.DefaultQoS, callback)
}

// SubscribeWithQoS 使用指定 QoS 订阅主题
func (m *MQTTClient) SubscribeWithQoS(topic string, qos byte, callback mqtt.MessageHandler) error {
	if m.options.Logging.LogMessageEvents {
		originalCallback := callback
		callback = func(client mqtt.Client, msg mqtt.Message) {
			log.Printf("[MQTT] message client=%s topic=%s payload=%s", m.options.Client.ID, msg.Topic(), string(msg.Payload()))
			originalCallback(client, msg)
		}
	}

	// 先登记意图，失败后 OnConnect 仍能补订。
	m.mu.Lock()
	m.subs[topic] = subscriptionEntry{qos: qos, callback: callback}
	m.mu.Unlock()

	token := m.client.Subscribe(topic, qos, callback)
	ok := token.WaitTimeout(time.Duration(m.options.Subscription.Timeout) * time.Second)
	if !ok {
		return fmt.Errorf("failed to subscribe to topic %s: timeout", topic)
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("failed to subscribe to topic %s: %w", topic, err)
	}

	return nil
}

// Unsubscribe 取消订阅
func (m *MQTTClient) Unsubscribe(topics ...string) error {
	token := m.client.Unsubscribe(topics...)
	ok := token.WaitTimeout(time.Duration(m.options.Subscription.Timeout) * time.Second)
	if !ok {
		return fmt.Errorf("failed to unsubscribe from topics: timeout")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("failed to unsubscribe from topics: %w", err)
	}

	m.mu.Lock()
	for _, topic := range topics {
		delete(m.subs, topic)
	}
	m.mu.Unlock()

	return nil
}

// Subscriptions 返回当前登记的订阅主题（用于诊断与测试）
func (m *MQTTClient) Subscriptions() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.subs))
	for topic := range m.subs {
		out = append(out, topic)
	}
	return out
}

// IsConnected 检查连接状态
func (m *MQTTClient) IsConnected() bool {
	return m.client.IsConnected()
}

// GetClient 获取原始客户端
func (m *MQTTClient) GetClient() mqtt.Client {
	return m.client
}

// setDefaults 设置默认值
func setDefaults(opt *Options) {
	if opt == nil {
		return
	}
	applyStableReconnectDefaults(&opt.Client)

	if opt.Client.KeepAlive == 0 {
		opt.Client.KeepAlive = 10
	}
	if opt.Client.PingTimeout == 0 {
		opt.Client.PingTimeout = 2
	}
	if opt.Client.ConnectTimeout == 0 {
		opt.Client.ConnectTimeout = 30
	}
	if opt.Client.ReconnectInterval == 0 {
		opt.Client.ReconnectInterval = 5
	}
	if opt.Client.MaxReconnectInterval == 0 {
		opt.Client.MaxReconnectInterval = 5
	}
	if opt.Message.Timeout == 0 {
		opt.Message.Timeout = 30
	}
	if opt.Subscription.Timeout == 0 {
		opt.Subscription.Timeout = 10
	}
	if opt.Publish.Timeout == 0 {
		opt.Publish.Timeout = 10
	}
	if opt.Buffer.SendBufferSize == 0 {
		opt.Buffer.SendBufferSize = 1024
	}
	if opt.Buffer.ReceiveBufferSize == 0 {
		opt.Buffer.ReceiveBufferSize = 1024
	}
	if opt.Buffer.MessageQueueSize == 0 {
		opt.Buffer.MessageQueueSize = 100
	}
	if opt.Performance.MaxConcurrentConnections == 0 {
		opt.Performance.MaxConcurrentConnections = 100
	}
	if opt.Performance.MessageHandlerCount == 0 {
		opt.Performance.MessageHandlerCount = 10
	}
	if opt.Performance.BatchSize == 0 {
		opt.Performance.BatchSize = 50
	}
}

// applyStableReconnectDefaults 保证长连接订阅场景的稳定重连默认行为。
// - 零值 Options（测试/代码直构）默认开启 AutoReconnect
// - 本包装层始终启用 ResumeSubs，并在 OnConnect 的独立协程中重放订阅
// - AutoReconnect 开启时同步开启首次 ConnectRetry
func applyStableReconnectDefaults(c *ClientConfig) {
	if c.KeepAlive == 0 && c.ConnectTimeout == 0 && c.ReconnectInterval == 0 &&
		c.MaxReconnectInterval == 0 && !c.AutoReconnect && !c.ConnectRetry && !c.ResumeSubs {
		c.AutoReconnect = true
	}
	c.ResumeSubs = true
	if c.AutoReconnect {
		c.ConnectRetry = true
	}
}

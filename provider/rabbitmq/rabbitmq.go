package rabbitmq

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"bit-labs.cn/owl/contract/log"
	"bit-labs.cn/owl/provider/otel"
	"github.com/streadway/amqp"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Options RabbitMQ 配置选项
type Options struct {
	Connection ConnectionConfig `json:"connection"`
	TLS        TLSConfig        `json:"tls"`
	Pool       PoolConfig       `json:"pool"`
	Exchange   ExchangeConfig   `json:"exchange"`
	Queue      QueueConfig      `json:"queue"`
	Consumer   ConsumerConfig   `json:"consumer"`
	Producer   ProducerConfig   `json:"producer"`
	Message    MessageConfig    `json:"message"`
	Retry      RetryConfig      `json:"retry"`
	Monitoring MonitoringConfig `json:"monitoring"`
}

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	Host                 string `json:"host"`
	Port                 int    `json:"port"`
	Username             string `json:"username"`
	Password             string `json:"password"`
	VHost                string `json:"vhost"`
	Timeout              int    `json:"timeout"`
	Heartbeat            int    `json:"heartbeat"`
	TLS                  bool   `json:"tls"`
	MaxReconnectAttempts int    `json:"max-reconnect-attempts"`
	ReconnectInterval    int    `json:"reconnect-interval"`
}

// TLSConfig TLS/SSL 配置
type TLSConfig struct {
	Enabled            bool   `json:"enabled"`
	CertFile           string `json:"cert-file"`
	KeyFile            string `json:"key-file"`
	CAFile             string `json:"ca-file"`
	InsecureSkipVerify bool   `json:"insecure-skip-verify"`
}

// PoolConfig 连接池配置
type PoolConfig struct {
	MaxConnections     int `json:"max-connections"`
	MinIdleConnections int `json:"min-idle-connections"`
	IdleTimeout        int `json:"idle-timeout"`
	MaxLifetime        int `json:"max-lifetime"`
}

// ExchangeConfig 交换机配置
type ExchangeConfig struct {
	DefaultName string `json:"default-name"`
	DefaultType string `json:"default-type"`
	Durable     bool   `json:"durable"`
	AutoDelete  bool   `json:"auto-delete"`
	Internal    bool   `json:"internal"`
	NoWait      bool   `json:"no-wait"`
}

// QueueConfig 队列配置
type QueueConfig struct {
	NamePrefix string                 `json:"name-prefix"`
	Durable    bool                   `json:"durable"`
	AutoDelete bool                   `json:"auto-delete"`
	Exclusive  bool                   `json:"exclusive"`
	NoWait     bool                   `json:"no-wait"`
	Args       map[string]interface{} `json:"args"`
}

// ConsumerConfig 消费者配置
type ConsumerConfig struct {
	Tag            string `json:"tag"`
	AutoAck        bool   `json:"auto-ack"`
	Exclusive      bool   `json:"exclusive"`
	NoWait         bool   `json:"no-wait"`
	PrefetchCount  int    `json:"prefetch-count"`
	PrefetchSize   int    `json:"prefetch-size"`
	GlobalPrefetch bool   `json:"global-prefetch"`
}

// ProducerConfig 生产者配置
type ProducerConfig struct {
	Mandatory   bool `json:"mandatory"`
	Immediate   bool `json:"immediate"`
	ConfirmMode bool `json:"confirm-mode"`
	Timeout     int  `json:"timeout"`
}

// MessageConfig 消息配置
type MessageConfig struct {
	ContentType     string `json:"content-type"`
	ContentEncoding string `json:"content-encoding"`
	DeliveryMode    uint8  `json:"delivery-mode"`
	Priority        uint8  `json:"priority"`
	Expiration      string `json:"expiration"`
}

// RetryConfig 重试配置
type RetryConfig struct {
	Enabled           bool    `json:"enabled"`
	MaxAttempts       int     `json:"max-attempts"`
	Interval          int     `json:"interval"`
	BackoffMultiplier float64 `json:"backoff-multiplier"`
	MaxInterval       int     `json:"max-interval"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	Enabled            bool `json:"enabled"`
	Interval           int  `json:"interval"`
	MonitorQueues      bool `json:"monitor-queues"`
	MonitorExchanges   bool `json:"monitor-exchanges"`
	MonitorConnections bool `json:"monitor-connections"`
}

// RabbitMQClient RabbitMQ 客户端包装器
type RabbitMQClient struct {
	conn     *amqp.Connection
	channels map[string]*amqp.Channel
	options  *Options
	ctx      context.Context
	cancel   context.CancelFunc
	mutex    sync.RWMutex
	l        log.Logger
}

// NewRabbitMQ 初始化 RabbitMQ 客户端
func NewRabbitMQ(opt *Options, l log.Logger) *RabbitMQClient {
	// 设置默认值
	setDefaults(opt)

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())

	client := &RabbitMQClient{
		channels: make(map[string]*amqp.Channel),
		options:  opt,
		ctx:      ctx,
		cancel:   cancel,
		l:        l,
	}

	return client
}

// Connect 连接到 RabbitMQ 服务器
func (r *RabbitMQClient) Connect() error {
	var err error

	// 构建连接 URL
	url := r.buildConnectionURL()

	// 配置 TLS
	var config *amqp.Config
	if r.options.TLS.Enabled {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: r.options.TLS.InsecureSkipVerify,
		}

		if r.options.TLS.CertFile != "" && r.options.TLS.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(r.options.TLS.CertFile, r.options.TLS.KeyFile)
			if err != nil {
				return fmt.Errorf("failed to load TLS certificate: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
		}

		config = &amqp.Config{
			TLSClientConfig: tlsConfig,
			Heartbeat:       time.Duration(r.options.Connection.Heartbeat) * time.Second,
		}
	} else {
		config = &amqp.Config{
			Heartbeat: time.Duration(r.options.Connection.Heartbeat) * time.Second,
		}
	}

	// 建立连接
	r.conn, err = amqp.DialConfig(url, *config)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	// 监听连接关闭事件
	go r.handleConnectionClose()

	r.l.Info("Connected to RabbitMQ")
	return nil
}

// Disconnect 断开连接
func (r *RabbitMQClient) Disconnect() error {
	r.cancel()

	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 关闭所有通道
	for name, ch := range r.channels {
		if err := ch.Close(); err != nil {
			r.l.Error("Failed to close channel %s: %v", name, err)
		}
	}
	r.channels = make(map[string]*amqp.Channel)

	// 关闭连接
	if r.conn != nil && !r.conn.IsClosed() {
		return r.conn.Close()
	}

	r.l.Info("Disconnected from RabbitMQ")
	return nil
}

// GetChannel 获取或创建通道
func (r *RabbitMQClient) GetChannel(name string) (*amqp.Channel, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if ch, exists := r.channels[name]; exists && r.IsConnected() {
		return ch, nil
	}

	ch, err := r.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}

	// 设置 QoS
	if err := ch.Qos(
		r.options.Consumer.PrefetchCount,
		r.options.Consumer.PrefetchSize,
		r.options.Consumer.GlobalPrefetch,
	); err != nil {
		ch.Close()
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	r.channels[name] = ch
	return ch, nil
}

// DeclareExchange 声明交换机
func (r *RabbitMQClient) DeclareExchange(name, kind string) error {
	ch, err := r.GetChannel("exchange")
	if err != nil {
		return err
	}

	return ch.ExchangeDeclare(
		name,
		kind,
		r.options.Exchange.Durable,
		r.options.Exchange.AutoDelete,
		r.options.Exchange.Internal,
		r.options.Exchange.NoWait,
		nil,
	)
}

// DeclareQueue 声明队列
func (r *RabbitMQClient) DeclareQueue(name string) (amqp.Queue, error) {
	ch, err := r.GetChannel("queue")
	if err != nil {
		return amqp.Queue{}, err
	}

	return ch.QueueDeclare(
		name,
		r.options.Queue.Durable,
		r.options.Queue.AutoDelete,
		r.options.Queue.Exclusive,
		r.options.Queue.NoWait,
		r.options.Queue.Args,
	)
}

// BindQueue 绑定队列到交换机
func (r *RabbitMQClient) BindQueue(queueName, routingKey, exchangeName string) error {
	ch, err := r.GetChannel("bind")
	if err != nil {
		return err
	}

	return ch.QueueBind(
		queueName,
		routingKey,
		exchangeName,
		r.options.Queue.NoWait,
		nil,
	)
}

// Publish 发布消息。不携带链路上下文。
func (r *RabbitMQClient) Publish(exchange, routingKey string, body []byte) error {
	return r.publish(context.Background(), exchange, routingKey, body, false)
}

// PublishContext 发布消息。ctx 中已有 span 时创建 producer span，并把 traceparent 写入 AMQP header。
func (r *RabbitMQClient) PublishContext(ctx context.Context, exchange, routingKey string, body []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return r.publish(ctx, exchange, routingKey, body, true)
}

func (r *RabbitMQClient) publish(ctx context.Context, exchange, routingKey string, body []byte, traced bool) (err error) {
	var span trace.Span
	if traced && trace.SpanFromContext(ctx).SpanContext().IsValid() {
		dest := exchange
		if dest == "" {
			dest = routingKey
		}
		ctx, span = otel.Start(ctx, dest+" publish", trace.WithSpanKind(trace.SpanKindProducer))
		defer func() {
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			span.End()
		}()
	}

	ch, err := r.GetChannel("publish")
	if err != nil {
		return err
	}

	if r.options.Producer.ConfirmMode {
		if err = ch.Confirm(false); err != nil {
			return fmt.Errorf("failed to put channel in confirm mode: %w", err)
		}
	}

	publishing := amqp.Publishing{
		ContentType:     r.options.Message.ContentType,
		ContentEncoding: r.options.Message.ContentEncoding,
		DeliveryMode:    r.options.Message.DeliveryMode,
		Priority:        r.options.Message.Priority,
		Timestamp:       time.Now(),
		Body:            body,
	}

	if r.options.Message.Expiration != "" {
		publishing.Expiration = r.options.Message.Expiration
	}
	if span != nil {
		publishing.Headers = amqp.Table(otel.InjectHeaders(ctx, map[string]interface{}(publishing.Headers)))
	}

	err = ch.Publish(
		exchange,
		routingKey,
		r.options.Producer.Mandatory,
		r.options.Producer.Immediate,
		publishing,
	)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	if r.options.Producer.ConfirmMode {
		if confirmed := ch.NotifyPublish(make(chan amqp.Confirmation, 1)); confirmed != nil {
			select {
			case confirm := <-confirmed:
				if !confirm.Ack {
					return fmt.Errorf("message was not confirmed by server")
				}
			case <-time.After(time.Duration(r.options.Producer.Timeout) * time.Second):
				return fmt.Errorf("publish confirmation timeout")
			}
		}
	}

	return nil
}

// Consume 消费消息。每条消息从 header 提取 trace context，并以 consumer span 调用 handler。
func (r *RabbitMQClient) Consume(queueName string, handler func(context.Context, amqp.Delivery)) error {
	if handler == nil {
		return fmt.Errorf("rabbitmq consume: nil handler")
	}
	ch, err := r.GetChannel("consume")
	if err != nil {
		return err
	}

	msgs, err := ch.Consume(
		queueName,
		r.options.Consumer.Tag,
		r.options.Consumer.AutoAck,
		r.options.Consumer.Exclusive,
		false, // no-local
		r.options.Consumer.NoWait,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	go func() {
		for {
			select {
			case msg, ok := <-msgs:
				if !ok {
					return
				}
				r.deliver(queueName, msg, handler)
			case <-r.ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (r *RabbitMQClient) deliver(queueName string, msg amqp.Delivery, handler func(context.Context, amqp.Delivery)) {
	ctx := otel.ExtractHeaders(context.Background(), map[string]interface{}(msg.Headers))
	ctx, span := otel.Start(ctx, queueName+" process", trace.WithSpanKind(trace.SpanKindConsumer))
	defer span.End()
	handler(ctx, msg)
}

// IsConnected 检查连接状态
func (r *RabbitMQClient) IsConnected() bool {
	return r.conn != nil && !r.conn.IsClosed()
}

// GetConnection 获取原始连接
func (r *RabbitMQClient) GetConnection() *amqp.Connection {
	return r.conn
}

// buildConnectionURL 构建连接 URL
func (r *RabbitMQClient) buildConnectionURL() string {
	scheme := "amqp"
	// 当启用 TLS（任一位置）时，使用 amqps
	if r.options.TLS.Enabled || r.options.Connection.TLS {
		scheme = "amqps"
	}

	return fmt.Sprintf("%s://%s:%s@%s:%d%s",
		scheme,
		r.options.Connection.Username,
		r.options.Connection.Password,
		r.options.Connection.Host,
		r.options.Connection.Port,
		r.options.Connection.VHost,
	)
}

// handleConnectionClose 处理连接关闭事件
func (r *RabbitMQClient) handleConnectionClose() {
	closeCh := make(chan *amqp.Error)
	r.conn.NotifyClose(closeCh)

	select {
	case err := <-closeCh:
		if err != nil {

			// 尝试重连
			if r.options.Connection.MaxReconnectAttempts > 0 {
				r.reconnect()
			}
		}
	case <-r.ctx.Done():
		return
	}
}

// reconnect 重连逻辑
func (r *RabbitMQClient) reconnect() {
	for attempt := 1; attempt <= r.options.Connection.MaxReconnectAttempts; attempt++ {

		time.Sleep(time.Duration(r.options.Connection.ReconnectInterval) * time.Second)

		if err := r.Connect(); err != nil {

			continue
		}

		return
	}

}

// setDefaults 设置默认值
func setDefaults(opt *Options) {
	if opt.Connection.Host == "" {
		opt.Connection.Host = "localhost"
	}
	if opt.Connection.Port == 0 {
		opt.Connection.Port = 5672
	}
	if opt.Connection.Username == "" {
		opt.Connection.Username = "guest"
	}
	if opt.Connection.Password == "" {
		opt.Connection.Password = "guest"
	}
	if opt.Connection.VHost == "" {
		opt.Connection.VHost = "/"
	}
	if opt.Connection.Timeout == 0 {
		opt.Connection.Timeout = 30
	}
	if opt.Connection.Heartbeat == 0 {
		opt.Connection.Heartbeat = 60
	}
	if opt.Connection.MaxReconnectAttempts == 0 {
		opt.Connection.MaxReconnectAttempts = 5
	}
	if opt.Connection.ReconnectInterval == 0 {
		opt.Connection.ReconnectInterval = 5
	}
	if opt.Pool.MaxConnections == 0 {
		opt.Pool.MaxConnections = 10
	}
	if opt.Pool.MinIdleConnections == 0 {
		opt.Pool.MinIdleConnections = 2
	}
	if opt.Pool.IdleTimeout == 0 {
		opt.Pool.IdleTimeout = 300
	}
	if opt.Pool.MaxLifetime == 0 {
		opt.Pool.MaxLifetime = 3600
	}
	if opt.Exchange.DefaultType == "" {
		opt.Exchange.DefaultType = "direct"
	}
	if opt.Consumer.PrefetchCount == 0 {
		opt.Consumer.PrefetchCount = 1
	}
	if opt.Producer.Timeout == 0 {
		opt.Producer.Timeout = 30
	}
	if opt.Message.ContentType == "" {
		opt.Message.ContentType = "application/json"
	}
	if opt.Message.DeliveryMode == 0 {
		opt.Message.DeliveryMode = 2 // 持久化
	}
	if opt.Retry.MaxAttempts == 0 {
		opt.Retry.MaxAttempts = 3
	}
	if opt.Retry.Interval == 0 {
		opt.Retry.Interval = 1
	}
	if opt.Retry.BackoffMultiplier == 0 {
		opt.Retry.BackoffMultiplier = 2.0
	}
	if opt.Retry.MaxInterval == 0 {
		opt.Retry.MaxInterval = 60
	}
	if opt.Monitoring.Interval == 0 {
		opt.Monitoring.Interval = 30
	}
}

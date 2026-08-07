package mqtt

import (
	"errors"
	"sort"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func TestSetDefaultsStableReconnect(t *testing.T) {
	opt := &Options{}
	setDefaults(opt)

	if !opt.Client.AutoReconnect {
		t.Fatal("expected AutoReconnect default true")
	}
	if !opt.Client.ResumeSubs {
		t.Fatal("expected ResumeSubs always true")
	}
	if !opt.Client.ConnectRetry {
		t.Fatal("expected ConnectRetry true when AutoReconnect")
	}
	if opt.Client.ReconnectInterval != 5 {
		t.Fatalf("ReconnectInterval=%d", opt.Client.ReconnectInterval)
	}
	if opt.Client.MaxReconnectInterval != 60 {
		t.Fatalf("MaxReconnectInterval=%d", opt.Client.MaxReconnectInterval)
	}
}

func TestBuildClientOptionsStableReconnect(t *testing.T) {
	opt := &Options{
		Broker: "tcp://127.0.0.1:1883",
		Client: ClientConfig{
			ID:                   "owl-device-test",
			CleanSession:         false,
			AutoReconnect:        true,
			ConnectRetry:         true,
			ResumeSubs:           true,
			ReconnectInterval:    5,
			MaxReconnectInterval: 60,
			KeepAlive:            60,
			ConnectTimeout:       30,
		},
		Logging: LoggingConfig{LogConnectionEvents: true},
	}
	setDefaults(opt)
	m := &MQTTClient{options: opt, subs: make(map[string]subscriptionEntry)}
	opts := buildClientOptions(opt, m)
	reader := mqtt.NewClient(opts).OptionsReader()

	if reader.ClientID() != "owl-device-test" {
		t.Fatalf("client id=%s", reader.ClientID())
	}
	if reader.CleanSession() {
		t.Fatal("expected CleanSession=false")
	}
	if !reader.AutoReconnect() {
		t.Fatal("expected AutoReconnect")
	}
	if !reader.ConnectRetry() {
		t.Fatal("expected ConnectRetry")
	}
	if !reader.ResumeSubs() {
		t.Fatal("expected ResumeSubs")
	}
	if reader.ConnectRetryInterval() != 5*time.Second {
		t.Fatalf("ConnectRetryInterval=%v", reader.ConnectRetryInterval())
	}
	if reader.MaxReconnectInterval() != 60*time.Second {
		t.Fatalf("MaxReconnectInterval=%v", reader.MaxReconnectInterval())
	}
}

func TestSubscriptionRegistryAndResubscribe(t *testing.T) {
	opt := &Options{
		Broker:       "tcp://127.0.0.1:1883",
		Client:       ClientConfig{ID: "reg-test", AutoReconnect: true},
		Subscription: SubscriptionConfig{Timeout: 1},
		Logging:      LoggingConfig{LogConnectionEvents: false},
	}
	setDefaults(opt)
	m := &MQTTClient{options: opt, subs: make(map[string]subscriptionEntry)}

	handler := func(mqtt.Client, mqtt.Message) {}
	fake := &fakeMQTTClient{}
	m.client = fake

	if err := m.SubscribeWithQoS("device/telemetry", 1, handler); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := m.SubscribeWithQoS("device/ctl/resp", 1, handler); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	got := m.Subscriptions()
	sort.Strings(got)
	if len(got) != 2 || got[0] != "device/ctl/resp" || got[1] != "device/telemetry" {
		t.Fatalf("subscriptions=%v", got)
	}

	fake.subscribed = nil
	m.resubscribeAll(fake)
	sort.Strings(fake.subscribed)
	if len(fake.subscribed) != 2 {
		t.Fatalf("re-subscribe topics=%v", fake.subscribed)
	}
	if fake.subscribed[0] != "device/ctl/resp" || fake.subscribed[1] != "device/telemetry" {
		t.Fatalf("re-subscribe topics=%v", fake.subscribed)
	}

	if err := m.Unsubscribe("device/telemetry"); err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}
	got = m.Subscriptions()
	if len(got) != 1 || got[0] != "device/ctl/resp" {
		t.Fatalf("after unsubscribe=%v", got)
	}

	fake.subscribed = nil
	m.resubscribeAll(fake)
	if len(fake.subscribed) != 1 || fake.subscribed[0] != "device/ctl/resp" {
		t.Fatalf("re-subscribe after unsubscribe=%v", fake.subscribed)
	}
}

func TestPublishTimeoutTreatedAsError(t *testing.T) {
	opt := &Options{
		Broker:  "tcp://127.0.0.1:1883",
		Client:  ClientConfig{ID: "pub-test"},
		Publish: PublishConfig{Timeout: 1, WaitForAck: true, DefaultQoS: 1},
	}
	setDefaults(opt)
	m := &MQTTClient{
		options: opt,
		client:  &fakeMQTTClient{publishToken: &fakeToken{waitOK: false}},
		subs:    make(map[string]subscriptionEntry),
	}
	err := m.PublishWithQoS("t", "payload", 1)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestSubscribeTimeoutTreatedAsError(t *testing.T) {
	opt := &Options{
		Broker:       "tcp://127.0.0.1:1883",
		Client:       ClientConfig{ID: "sub-test"},
		Subscription: SubscriptionConfig{Timeout: 1},
	}
	setDefaults(opt)
	m := &MQTTClient{
		options: opt,
		client:  &fakeMQTTClient{subscribeToken: &fakeToken{waitOK: false}},
		subs:    make(map[string]subscriptionEntry),
	}
	err := m.SubscribeWithQoS("t", 1, func(mqtt.Client, mqtt.Message) {})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if len(m.Subscriptions()) != 0 {
		t.Fatal("failed subscribe must not register topic")
	}
}

func TestSubscribeErrorNotRegistered(t *testing.T) {
	opt := &Options{
		Broker:       "tcp://127.0.0.1:1883",
		Client:       ClientConfig{ID: "sub-err"},
		Subscription: SubscriptionConfig{Timeout: 1},
	}
	setDefaults(opt)
	m := &MQTTClient{
		options: opt,
		client:  &fakeMQTTClient{subscribeToken: &fakeToken{waitOK: true, err: errors.New("broker reject")}},
		subs:    make(map[string]subscriptionEntry),
	}
	err := m.SubscribeWithQoS("t", 1, func(mqtt.Client, mqtt.Message) {})
	if err == nil {
		t.Fatal("expected subscribe error")
	}
	if len(m.Subscriptions()) != 0 {
		t.Fatal("failed subscribe must not register topic")
	}
}

type fakeToken struct {
	waitOK bool
	err    error
}

func (t *fakeToken) Wait() bool { return t.waitOK }
func (t *fakeToken) WaitTimeout(time.Duration) bool {
	return t.waitOK
}
func (t *fakeToken) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (t *fakeToken) Error() error { return t.err }

type fakeMQTTClient struct {
	subscribed     []string
	subscribeToken mqtt.Token
	publishToken   mqtt.Token
	unsubToken     mqtt.Token
}

func (f *fakeMQTTClient) IsConnected() bool      { return true }
func (f *fakeMQTTClient) IsConnectionOpen() bool { return true }
func (f *fakeMQTTClient) Connect() mqtt.Token {
	return &fakeToken{waitOK: true}
}
func (f *fakeMQTTClient) Disconnect(uint) {}
func (f *fakeMQTTClient) Publish(string, byte, bool, interface{}) mqtt.Token {
	if f.publishToken != nil {
		return f.publishToken
	}
	return &fakeToken{waitOK: true}
}
func (f *fakeMQTTClient) Subscribe(topic string, _ byte, _ mqtt.MessageHandler) mqtt.Token {
	if f.subscribeToken != nil {
		return f.subscribeToken
	}
	f.subscribed = append(f.subscribed, topic)
	return &fakeToken{waitOK: true}
}
func (f *fakeMQTTClient) SubscribeMultiple(map[string]byte, mqtt.MessageHandler) mqtt.Token {
	return &fakeToken{waitOK: true}
}
func (f *fakeMQTTClient) Unsubscribe(topics ...string) mqtt.Token {
	if f.unsubToken != nil {
		return f.unsubToken
	}
	return &fakeToken{waitOK: true}
}
func (f *fakeMQTTClient) AddRoute(string, mqtt.MessageHandler) {}
func (f *fakeMQTTClient) OptionsReader() mqtt.ClientOptionsReader {
	return mqtt.NewClient(mqtt.NewClientOptions()).OptionsReader()
}
